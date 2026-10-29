package baud

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestLivePTY exercises the real serial.Open, SetMode, reset and read path
// against a pair of PTYs. It cannot test baud discrimination (a PTY passes
// bytes unchanged), but it catches driver-level plumbing errors.
//
//	socat pty,raw,echo=0,link=/tmp/baudscan-pty-a pty,raw,echo=0,link=/tmp/baudscan-pty-b
//	BAUDSCAN_PTY=/tmp/baudscan-pty-a BAUDSCAN_PTY_PEER=/tmp/baudscan-pty-b go test ./internal/baud -run TestLivePTY
func TestLivePTY(t *testing.T) {
	portName := os.Getenv("BAUDSCAN_PTY")
	peerName := os.Getenv("BAUDSCAN_PTY_PEER")
	if portName == "" || peerName == "" {
		t.Skip("set BAUDSCAN_PTY and BAUDSCAN_PTY_PEER to run the PTY integration test")
	}
	master, err := os.OpenFile(peerName, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		line := []byte("$GNGGA,123519,4807.038,N,01131.000,E,1,08,0.9,545.4,M,46.9,M,,*47\r\n")
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := master.Write(line); err != nil {
				return
			}
			time.Sleep(15 * time.Millisecond)
		}
	}()
	defer func() { close(stop); <-done }()

	results, err := NewScanner().Scan(context.Background(), ScanOptions{
		Port:      portName,
		Rates:     All,
		Sample:    350 * time.Millisecond,
		Settle:    20 * time.Millisecond,
		IdleAfter: 20 * time.Millisecond,
		MinSample: 150 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(All) {
		t.Fatalf("got %d results, want %d", len(results), len(All))
	}
	for _, r := range results {
		if r.Bytes == 0 {
			t.Errorf("%d: no bytes read over the PTY", r.Baud)
		}
	}
	Rank(results, All)
	if results[0].Analysis.Score < 80 {
		t.Fatalf("best score = %d, want text-like: %+v", results[0].Analysis.Score, results[0].Analysis)
	}
	t.Logf("best = %d (%d points), %d bytes", results[0].Baud, results[0].Analysis.Score, results[0].Bytes)
}
