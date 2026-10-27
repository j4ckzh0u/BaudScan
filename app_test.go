package main

import (
	"errors"
	"testing"
	"time"

	"baudscan/internal/baud"
	"baudscan/internal/ports"
)

func fakePorts(names ...string) []ports.Port {
	out := make([]ports.Port, len(names))
	for i, name := range names {
		out[i] = ports.Port{Name: name, Label: "USB 串口", USB: true}
	}
	return out
}

func TestRefreshPortsChoosesFirstAndKeepsChoice(t *testing.T) {
	usb := fakePorts("/dev/cu.usbserial-1", "/dev/cu.usbserial-2")
	a := &app{
		selected: -1,
		listPorts: func() ([]ports.Port, error) {
			return usb, nil
		},
	}
	a.refreshPorts(false)
	if a.port != usb[0].Name {
		t.Fatalf("port = %q, want the first port", a.port)
	}
	if a.statusLevel != statusNormal || a.status == "" {
		t.Errorf("status = %q (%v)", a.status, a.statusLevel)
	}

	// A device that stays plugged in keeps the user's choice.
	a.port = usb[1].Name
	a.refreshPorts(true)
	if a.port != usb[1].Name {
		t.Errorf("port = %q after refresh, want the kept choice", a.port)
	}
	if a.statusLevel != statusNormal {
		t.Errorf("status level = %v, want normal", a.statusLevel)
	}
}

func TestRefreshPortsWhenEmpty(t *testing.T) {
	a := &app{
		selected:  0,
		results:   []baud.Result{{Baud: 9600}},
		listPorts: func() ([]ports.Port, error) { return nil, nil },
	}
	a.refreshPorts(true)
	if a.port != "" || a.selected != -1 || a.results != nil {
		t.Errorf("port = %q, selected = %d, results = %d; want empty", a.port, a.selected, len(a.results))
	}
	if a.statusLevel != statusWarn {
		t.Errorf("status level = %v, want warning", a.statusLevel)
	}
}

func TestRefreshPortsError(t *testing.T) {
	a := &app{
		port:      "/dev/cu.usb",
		selected:  0,
		results:   []baud.Result{{Baud: 9600}},
		listPorts: func() ([]ports.Port, error) { return nil, errors.New("no access") },
	}
	a.refreshPorts(true)
	if a.statusLevel != statusBad || a.port != "" || len(a.results) != 0 {
		t.Errorf("port = %q, results = %d, status = %q (%v)", a.port, len(a.results), a.status, a.statusLevel)
	}
}

func TestFinishScanStatuses(t *testing.T) {
	text := []byte("$GNGGA,123519,4807.038,N,01131.000,E,1,08,0.9,545.4,M,46.9,M,,*47\r\n")
	a := &app{}
	a.results = []baud.Result{
		{Baud: 9600, Bytes: len(text), Data: text, Analysis: baud.Analyze(text)},
	}
	a.finishScan(nil, 2*time.Second)
	if a.statusLevel != statusGood {
		t.Errorf("text result: status = %q (%v), want good", a.status, a.statusLevel)
	}
	if !a.hasData() {
		t.Error("hasData = false with data")
	}

	a.results = []baud.Result{
		{Baud: 9600, Analysis: baud.Analyze(nil)},
		{Baud: 115200, Analysis: baud.Analyze(nil)},
	}
	a.finishScan(nil, time.Second)
	if a.statusLevel != statusWarn || a.hasData() {
		t.Errorf("no data: status = %q (%v), hasData = %v", a.status, a.statusLevel, a.hasData())
	}
}

func TestCloseRace(t *testing.T) {
	a := &app{results: []baud.Result{
		{Baud: 9600, Analysis: baud.Analysis{Score: 80}},
		{Baud: 115200, Analysis: baud.Analysis{Score: 76}},
	}}
	if !a.closeRace() || a.margin() != 4 {
		t.Errorf("closeRace = %v, margin = %d; want true/4", a.closeRace(), a.margin())
	}
	a.results[1].Analysis.Score = 60
	if a.closeRace() {
		t.Error("closeRace = true for a clear winner")
	}
}

func TestDurationAndPreset(t *testing.T) {
	a := &app{duration: "5.0 秒", preset: "扩展"}
	if got := a.sampleDuration(); got != 5*time.Second {
		t.Errorf("duration = %v", got)
	}
	if got := a.selectedPreset().Name; got != "扩展" {
		t.Errorf("preset = %q", got)
	}
	a.duration = "unknown"
	a.preset = "unknown"
	if got := a.sampleDuration(); got != time.Second {
		t.Errorf("fallback duration = %v", got)
	}
	if got := a.selectedPreset().Name; got != baud.Presets[0].Name {
		t.Errorf("fallback preset = %q", got)
	}
}
