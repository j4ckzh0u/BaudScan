package baud

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"go.bug.st/serial"
)

// fakePort feeds each baud rate a different capture, so the scanner sees
// clean text at the real rate and noise everywhere else, as with hardware.
type fakePort struct {
	mu      sync.Mutex
	streams map[int][]byte
	mode    int
	data    []byte
	at      int
	timeout time.Duration
	closed  bool
	fail    map[int]bool
}

func (p *fakePort) SetMode(m *serial.Mode) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fail[m.BaudRate] {
		return errors.New("cannot set mode")
	}
	p.mode = m.BaudRate
	p.data = p.streams[m.BaudRate]
	p.at = 0
	return nil
}

func (p *fakePort) Read(b []byte) (int, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return 0, &serial.PortError{}
	}
	if p.at < len(p.data) {
		n := copy(b, p.data[p.at:])
		p.at += n
		p.mu.Unlock()
		return n, nil
	}
	timeout := p.timeout
	p.mu.Unlock()
	if timeout <= 0 {
		timeout = 10 * time.Millisecond
	}
	time.Sleep(timeout)
	return 0, nil
}

func (p *fakePort) Write(b []byte) (int, error) { return len(b), nil }
func (p *fakePort) Drain() error                { return nil }
func (p *fakePort) ResetInputBuffer() error {
	p.mu.Lock()
	p.at = 0
	p.mu.Unlock()
	return nil
}
func (p *fakePort) ResetOutputBuffer() error { return nil }
func (p *fakePort) SetDTR(bool) error        { return nil }
func (p *fakePort) SetRTS(bool) error        { return nil }
func (p *fakePort) GetModemStatusBits() (*serial.ModemStatusBits, error) {
	return &serial.ModemStatusBits{}, nil
}
func (p *fakePort) SetReadTimeout(d time.Duration) error {
	p.mu.Lock()
	p.timeout = d
	p.mu.Unlock()
	return nil
}
func (p *fakePort) Close() error {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	return nil
}
func (p *fakePort) Break(time.Duration) error { return nil }

func fakeScanner() (*Scanner, *fakePort) {
	r := rand.New(rand.NewPCG(9, 10))
	streams := make(map[int][]byte)
	for _, rate := range Extended {
		noise := make([]byte, 3000)
		for i := range noise {
			noise[i] = byte(r.Uint32())
		}
		streams[rate] = noise
	}
	streams[9600] = textStream(20)
	p := &fakePort{streams: streams}
	return &Scanner{Open: func(name string, mode *serial.Mode) (serial.Port, error) {
		p.mode = mode.BaudRate
		p.data = p.streams[mode.BaudRate]
		return p, nil
	}}, p
}

func TestScanFindsTextRate(t *testing.T) {
	scanner, _ := fakeScanner()
	opts := ScanOptions{
		Port:      "fake",
		Rates:     Common,
		Sample:    120 * time.Millisecond,
		IdleAfter: 8 * time.Millisecond,
		MinSample: 20 * time.Millisecond,
		Settle:    time.Millisecond,
	}
	results, err := scanner.Scan(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(Common) {
		t.Fatalf("got %d results, want %d", len(results), len(Common))
	}
	Rank(results)
	best := results[0]
	if best.Baud != 9600 {
		t.Fatalf("best = %d (score %d), want 9600: %+v", best.Baud, best.Analysis.Score, results)
	}
	if best.Analysis.Score < 80 {
		t.Errorf("score = %d, want at least 80", best.Analysis.Score)
	}
	if len(best.Data) == 0 || best.Bytes == 0 {
		t.Error("best result has no preview data")
	}
}

func TestScanNoData(t *testing.T) {
	scanner := &Scanner{Open: func(name string, mode *serial.Mode) (serial.Port, error) {
		return &fakePort{}, nil
	}}
	results, err := scanner.Scan(context.Background(), ScanOptions{
		Port:      "fake",
		Rates:     []int{9600, 115200},
		Sample:    40 * time.Millisecond,
		IdleAfter: 5 * time.Millisecond,
		MinSample: 5 * time.Millisecond,
		Settle:    time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, res := range results {
		if res.Bytes != 0 || res.Analysis.Kind != KindNoData {
			t.Errorf("%d: %+v", res.Baud, res)
		}
	}
}

func TestScanOpenError(t *testing.T) {
	scanner := &Scanner{Open: func(name string, mode *serial.Mode) (serial.Port, error) {
		return nil, errors.New("busy")
	}}
	_, err := scanner.Scan(context.Background(), ScanOptions{Port: "fake", Rates: []int{9600}})
	if err == nil {
		t.Fatal("want an open error")
	}
}

func TestScanCancel(t *testing.T) {
	scanner, _ := fakeScanner()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var events int
	done := make(chan error, 1)
	go func() {
		_, err := scanner.Scan(ctx, ScanOptions{
			Port:      "fake",
			Rates:     Common,
			Sample:    200 * time.Millisecond,
			IdleAfter: 20 * time.Millisecond,
			MinSample: 50 * time.Millisecond,
			Settle:    time.Millisecond,
			OnEvent: func(e Event) {
				events++
				if events == 3 {
					cancel()
				}
			},
		})
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("scan did not stop after cancellation")
	}
}

func TestScanSetModeErrorKeepsGoing(t *testing.T) {
	p := &fakePort{
		streams: map[int][]byte{
			9600:  textStream(5),
			19200: textStream(5),
		},
		fail: map[int]bool{19200: true},
	}
	scanner := &Scanner{Open: func(name string, mode *serial.Mode) (serial.Port, error) {
		return p, nil
	}}
	results, err := scanner.Scan(context.Background(), ScanOptions{
		Port:      "fake",
		Rates:     []int{9600, 19200},
		Sample:    30 * time.Millisecond,
		IdleAfter: 4 * time.Millisecond,
		MinSample: 5 * time.Millisecond,
		Settle:    time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if results[0].Err != "" || results[0].Baud != 9600 {
		t.Errorf("first result = %+v, want a clean 9600", results[0])
	}
	if results[1].Err == "" || results[1].Baud != 19200 {
		t.Errorf("second result = %+v, want the mode error", results[1])
	}
}
