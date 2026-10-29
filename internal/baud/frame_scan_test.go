package baud

import (
	"context"
	"testing"
	"time"

	"go.bug.st/serial"
)

// recordingPort remembers every mode it was asked for.
type recordingPort struct {
	fakePort
	modes     []serial.Mode
	failFrame bool
}

func (p *recordingPort) SetMode(m *serial.Mode) error {
	p.modes = append(p.modes, *m)
	if p.failFrame && (m.DataBits != 8 || m.Parity != serial.NoParity || m.StopBits != serial.OneStopBit) {
		return &serial.PortError{}
	}
	return p.fakePort.SetMode(m)
}

func TestScanAppliesFrameAndCustomRates(t *testing.T) {
	p := &recordingPort{fakePort: fakePort{streams: map[int][]byte{
		31250: textStream(5),
		10400: textStream(5),
	}}}
	scanner := &Scanner{Open: func(name string, mode *serial.Mode) (serial.Port, error) {
		p.modes = append(p.modes, *mode)
		p.data = p.streams[mode.BaudRate]
		return p, nil
	}}
	frame := Frame{DataBits: 7, Parity: serial.EvenParity, StopBits: serial.TwoStopBits}
	results, err := scanner.Scan(context.Background(), ScanOptions{
		Port:      "fake",
		Rates:     []int{31250, 10400},
		Frame:     frame,
		Sample:    20 * time.Millisecond,
		Settle:    time.Millisecond,
		IdleAfter: 4 * time.Millisecond,
		MinSample: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want2", len(results))
	}
	if len(p.modes) != 2 {
		t.Fatalf("modes set = %d, want one per rate", len(p.modes))
	}
	for i, m := range p.modes {
		if m.DataBits != 7 || m.Parity != serial.EvenParity || m.StopBits != serial.TwoStopBits {
			t.Errorf("mode %d = %+v, want the requested frame", i, m)
		}
	}
	if p.modes[0].BaudRate != 31250 || p.modes[1].BaudRate != 10400 {
		t.Errorf("rates = %d, %d, want31250,10400", p.modes[0].BaudRate, p.modes[1].BaudRate)
	}
	if results[0].Pending || results[1].Pending {
		t.Error("a scanned rate is still pending")
	}
}

func TestScanDefaultsTo8N1(t *testing.T) {
	p := &recordingPort{fakePort: fakePort{streams: map[int][]byte{9600: textStream(3)}}}
	scanner := &Scanner{Open: func(name string, mode *serial.Mode) (serial.Port, error) {
		p.modes = append(p.modes, *mode)
		p.data = p.streams[mode.BaudRate]
		return p, nil
	}}
	if _, err := scanner.Scan(context.Background(), ScanOptions{
		Port:      "fake",
		Rates:     []int{9600, 19200},
		Sample:    20 * time.Millisecond,
		Settle:    time.Millisecond,
		IdleAfter: 4 * time.Millisecond,
		MinSample: 5 * time.Millisecond,
	}); err != nil {
		t.Fatal(err)
	}
	for _, m := range p.modes {
		if m.DataBits != 8 || m.Parity != serial.NoParity || m.StopBits != serial.OneStopBit {
			t.Errorf("mode = %+v, want8N1", m)
		}
	}
}
