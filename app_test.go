package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.bug.st/serial"

	"github.com/j4ckzh0u/BaudScan/internal/baud"
	"github.com/j4ckzh0u/BaudScan/internal/ports"
)

func fakePorts(names ...string) []ports.Port {
	out := make([]ports.Port, len(names))
	for i, name := range names {
		out[i] = ports.Port{Name: name, Label: "USB串口", USB: true}
	}
	return out
}

func newTestApp() *app {
	a := &app{
		listPorts: func() ([]ports.Port, error) {
			return fakePorts("/dev/cu.usbserial-1"), nil
		},
		duration: durationOptions[2].Label,
		dataBits: "8",
		parity:   "无",
		stopBits: "1",
	}
	a.refreshPlan()
	a.resetResults()
	return a
}

func TestResetResultsListsEveryRate(t *testing.T) {
	a := newTestApp()
	if len(a.results) != len(baud.All) {
		t.Fatalf("results = %d, want %d rows", len(a.results), len(baud.All))
	}
	if a.selected != -1 {
		t.Errorf("selected = %d, want -1 before a scan", a.selected)
	}
	for i, r := range a.results {
		if r.Baud != baud.All[i] || !r.Pending {
			t.Errorf("row %d = %+v, want pending %d", i, r, baud.All[i])
		}
	}
	if a.doneCount() != 0 {
		t.Errorf("doneCount = %d, want0", a.doneCount())
	}
}

func TestStoreFillsRowsAndRanks(t *testing.T) {
	a := newTestApp()
	text := []byte("$GNGGA,123519,4807.038,N,01131.000,E,1,08,0.9,545.4,M,46.9,M,,*47\r\n")
	a.store(baud.Result{Baud: 9600, Bytes: len(text), Data: text, Analysis: baud.Analyze(text)})
	a.store(baud.Result{Baud: 4000000, Bytes: 4, Analysis: baud.Analyze([]byte{0x00, 0xff, 0x11, 0x22})})

	if len(a.results) != len(baud.All) {
		t.Fatalf("rows = %d, want the full list %d", len(a.results), len(baud.All))
	}
	if a.results[0].Baud != 9600 || a.results[0].Pending {
		t.Errorf("top row = %+v, want the sampled9600", a.results[0])
	}
	if a.doneCount() != 2 {
		t.Errorf("doneCount = %d, want2", a.doneCount())
	}
	// Pending rows stay after every sampled row.
	seenPending := false
	for _, r := range a.results {
		if r.Pending {
			seenPending = true
			continue
		}
		if seenPending {
			t.Fatalf("sampled row %+v after a pending row", r)
		}
	}
	if a.results[len(a.results)-1].Pending == false {
		t.Error("last row is sampled, want pending rows at the end")
	}
}

func TestSelectBestPrefersData(t *testing.T) {
	a := newTestApp()
	text := []byte("temperature:23.5 C\r\n")
	a.store(baud.Result{Baud: 9600, Analysis: baud.Analyze(nil)})
	a.selectBest()
	if a.selected != -1 {
		t.Errorf("selected = %d with no data anywhere, want -1", a.selected)
	}
	a.store(baud.Result{Baud: 115200, Bytes: len(text), Data: text, Analysis: baud.Analyze(text)})
	a.selectBest()
	if a.selected < 0 || a.results[a.selected].Baud != 115200 {
		t.Errorf("selected = %d, want the row with data", a.selected)
	}
	// A click pins the choice; later samples do not move it.
	a.manualPick = true
	a.store(baud.Result{Baud: 9600, Bytes: len(text), Data: text, Analysis: baud.Analyze(text)})
	a.selectBest()
	if a.results[a.selected].Baud != 115200 {
		t.Errorf("selected %d, want the pinned115200", a.results[a.selected].Baud)
	}
}

func TestRefreshPortsKeepsChoiceAndRows(t *testing.T) {
	a := newTestApp()
	if err := a.listPorts; err == nil {
		t.Fatal("test setup: listPorts must be set")
	}
	usb := fakePorts("/dev/cu.usbserial-1", "/dev/cu.usbserial-2")
	a.listPorts = func() ([]ports.Port, error) { return usb, nil }
	a.refreshPorts(false)
	if a.port != usb[0].Name {
		t.Fatalf("port = %q, want the first port", a.port)
	}
	if a.statusLevel != statusNormal || a.status == "" {
		t.Errorf("status = %q (%v)", a.status, a.statusLevel)
	}

	a.port = usb[1].Name
	a.refreshPorts(true)
	if a.port != usb[1].Name {
		t.Errorf("port = %q after refresh, want the kept choice", a.port)
	}
	if len(a.results) != len(baud.All) {
		t.Errorf("rows = %d, want the full list", len(a.results))
	}
}

func TestRefreshPortsResetsRowsWhenPortGoes(t *testing.T) {
	a := newTestApp()
	text := []byte("hello world\r\n")
	a.store(baud.Result{Baud: 9600, Bytes: len(text), Data: text, Analysis: baud.Analyze(text)})
	a.listPorts = func() ([]ports.Port, error) { return nil, nil }
	a.refreshPorts(true)
	if a.port != "" || a.doneCount() != 0 {
		t.Errorf("port = %q, done = %d; want an empty port and fresh rows", a.port, a.doneCount())
	}
	if a.statusLevel != statusWarn {
		t.Errorf("status level = %v, want warning", a.statusLevel)
	}
}

func TestRefreshPortsError(t *testing.T) {
	a := newTestApp()
	a.listPorts = func() ([]ports.Port, error) { return nil, errors.New("no access") }
	a.refreshPorts(true)
	if a.statusLevel != statusBad || a.port != "" {
		t.Errorf("port = %q, status = %q (%v)", a.port, a.status, a.statusLevel)
	}
}

func TestFinishScanStatuses(t *testing.T) {
	text := []byte("$GNGGA,123519,4807.038,N,01131.000,E,1,08,0.9,545.4,M,46.9,M,,*47\r\n")
	a := newTestApp()
	a.store(baud.Result{Baud: 9600, Bytes: len(text), Data: text, Analysis: baud.Analyze(text)})
	a.selectBest()
	a.finishScan(nil, 2*time.Second)
	if a.statusLevel != statusGood {
		t.Errorf("text result: status = %q (%v), want good", a.status, a.statusLevel)
	}

	a = newTestApp()
	a.finishScan(nil, time.Second)
	if a.statusLevel != statusWarn || a.hasData() {
		t.Errorf("no data: status = %q (%v), hasData = %v", a.status, a.statusLevel, a.hasData())
	}

	a = newTestApp()
	a.finishScan(context.Canceled, time.Second)
	if a.statusLevel != statusNormal {
		t.Errorf("canceled: status = %q (%v), want normal", a.status, a.statusLevel)
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
	a.results[1].Pending = true
	if a.closeRace() {
		t.Error("closeRace = true with a pending runner-up")
	}
}

func TestSampleDurationAndEstimate(t *testing.T) {
	a := &app{duration: "5.0秒"}
	a.plan = baud.All
	if got := a.sampleDuration(); got != 5*time.Second {
		t.Errorf("duration = %v", got)
	}
	a.duration = "unknown"
	if got := a.sampleDuration(); got != time.Second {
		t.Errorf("fallback duration = %v", got)
	}
	want := time.Duration(len(baud.All)) * (time.Second + baud.DefaultSettle)
	if got := a.estimate(); got != want+baud.OpenDelay {
		t.Errorf("estimate = %v, want %v", got, want+baud.OpenDelay)
	}
}

func TestRateListIsComplete(t *testing.T) {
	seen := make(map[int]bool, len(baud.All))
	for _, rate := range baud.All {
		if rate <= 0 || seen[rate] {
			t.Errorf("bad or duplicate rate %d", rate)
		}
		seen[rate] = true
	}
	for _, rate := range baud.Standard {
		if !seen[rate] {
			t.Errorf("standard rate %d missing from All", rate)
		}
	}
	for _, rate := range baud.Common {
		if !seen[rate] {
			t.Errorf("common rate %d missing from All", rate)
		}
	}
	if len(baud.All) != len(baud.Standard) {
		t.Errorf("All = %d rates, Standard = %d", len(baud.All), len(baud.Standard))
	}
	// Common rates come first, so a common device is found early.
	for i, rate := range baud.Common {
		if baud.All[i] != rate {
			t.Errorf("All[%d] = %d, want common rate %d", i, baud.All[i], rate)
		}
	}
}

func TestCustomRatesExtendPlan(t *testing.T) {
	a := newTestApp()
	if len(a.results) != len(baud.All) {
		t.Fatalf("rows = %d, want %d", len(a.results), len(baud.All))
	}
	a.customRates = "31250,10400"
	a.customRatesChanged()
	if a.customErr != "" {
		t.Fatalf("customErr = %q", a.customErr)
	}
	if len(a.results) != len(baud.All)+2 {
		t.Fatalf("rows = %d, want two more than %d", len(a.results), len(baud.All))
	}
	want := baud.Plan([]int{31250, 10400})
	for i := range want {
		if a.plan[i] != want[i] {
			t.Fatalf("plan[%d] = %d, want %d", i, a.plan[i], want[i])
		}
	}
	if a.results[0].Baud != want[0] || !a.results[0].Pending {
		t.Errorf("first row = %+v, want pending %d", a.results[0], want[0])
	}
	if got := a.estimate(); got != time.Duration(len(want))*(a.sampleDuration()+baud.DefaultSettle)+baud.OpenDelay {
		t.Errorf("estimate = %v, want it to follow the plan", got)
	}
}

func TestCustomRatesErrorKeepsRowsAndBlocksScan(t *testing.T) {
	a := newTestApp()
	a.port = "/dev/cu.usbserial-1"
	rows := len(a.results)

	a.customRates = "abc"
	a.customRatesChanged()
	if a.customErr == "" {
		t.Fatal("customErr is empty for unreadable text")
	}
	if len(a.results) != rows {
		t.Errorf("rows = %d, want them unchanged at %d", len(a.results), rows)
	}
	if a.statusLevel != statusBad {
		t.Errorf("status level = %v, want bad", a.statusLevel)
	}
	if _, err := baud.ParseRates(a.customRates); err == nil {
		t.Error("ParseRates accepted the text the app rejected")
	}
}

func TestCustomRatesClearedRestoresStandardPlan(t *testing.T) {
	a := newTestApp()
	a.customRates = "31250"
	a.customRatesChanged()
	a.customRates = ""
	a.customRatesChanged()
	if a.customErr != "" {
		t.Fatalf("customErr = %q", a.customErr)
	}
	if len(a.results) != len(baud.All) {
		t.Errorf("rows = %d, want the standard list %d", len(a.results), len(baud.All))
	}
	if got := baud.CustomRates(a.plan); len(got) != 0 {
		t.Errorf("custom rates = %v, want none", got)
	}
}

func TestFrameChangedResetsRows(t *testing.T) {
	a := newTestApp()
	text := []byte("hello world\r\n")
	a.store(baud.Result{Baud: 9600, Bytes: len(text), Data: text, Analysis: baud.Analyze(text)})
	if !a.hasData() {
		t.Fatal("setup: no stored sample")
	}

	a.parity = "偶"
	a.frameChanged()
	if a.hasData() {
		t.Error("samples read under the old framing survive a framing change")
	}
	if a.frame().Label() != "8E1" {
		t.Errorf("frame = %s, want8E1", a.frame().Label())
	}
	if a.doneCount() != 0 || a.selected != -1 {
		t.Errorf("done = %d, selected = %d; want a fresh table", a.doneCount(), a.selected)
	}
	if a.statusLevel != statusNormal || a.status == "" {
		t.Errorf("status = %q (%v)", a.status, a.statusLevel)
	}
}

func TestScanUsesPlanAndFrame(t *testing.T) {
	a := newTestApp()
	a.port = "/dev/cu.usbserial-1"
	a.customRates = "31250"
	a.customRatesChanged()
	a.parity = "偶"
	a.stopBits = "2"

	opened := make(chan serial.Mode, 1)
	a.scanner = &baud.Scanner{Open: func(name string, mode *serial.Mode) (serial.Port, error) {
		opened <- *mode
		return nil, errors.New("stop here")
	}}
	a.startScan()

	select {
	case mode := <-opened:
		if mode.DataBits != 8 || mode.Parity != serial.EvenParity || mode.StopBits != serial.TwoStopBits {
			t.Errorf("opened with %+v, want the frame the controls show", mode)
		}
		if mode.BaudRate != a.plan[0] {
			t.Errorf("opened at %d, want the first rate of the plan, %d", mode.BaudRate, a.plan[0])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the scan did not open the port")
	}
}

func TestFinishScanReportsOpenError(t *testing.T) {
	a := newTestApp()
	a.finishScan(errors.New("busy"), time.Second)
	if a.statusLevel != statusBad || !strings.Contains(a.status, "扫描中断") {
		t.Errorf("status = %q (%v), want the open error reported", a.status, a.statusLevel)
	}
}

func TestDefaultLabelsMatchFrame(t *testing.T) {
	if defaultLabels[0] != "8" || defaultLabels[1] != "无" || defaultLabels[2] != "1" {
		t.Errorf("defaultLabels = %v, want8/无/1", defaultLabels)
	}
	if f := (baud.FrameFrom(defaultLabels[0], defaultLabels[1], defaultLabels[2])); f.Label() != "8N1" {
		t.Errorf("default frame = %s, want8N1", f.Label())
	}
}
