package main

import (
	"fmt"
	"image/png"
	"math/rand/v2"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"baudscan/internal/baud"
	"baudscan/internal/ports"
)

func sampleApp() *app {
	text := []byte(strings.Repeat("$GNGGA,123519,4807.038,N,01131.000,E,1,08,0.9,545.4,M,46.9,M,,*47\r\n", 8))
	r := rand.New(rand.NewPCG(11, 12))
	noise := make([]byte, 900)
	for i := range noise {
		noise[i] = byte(r.Uint32())
	}
	empty := []byte{}
	results := []baud.Result{
		{Baud: 9600, Bytes: len(text), Data: text, Analysis: baud.Analyze(text)},
		{Baud: 115200, Bytes: len(noise), Data: noise, Analysis: baud.Analyze(noise)},
		{Baud: 57600, Bytes: len(empty), Data: empty, Analysis: baud.Analyze(empty)},
	}
	baud.Rank(results)
	return &app{
		ports: []ports.Port{
			{Name: "/dev/cu.usbserial-1420", Label: "USB 串口", USB: true},
			{Name: "/dev/cu.Bluetooth-Incoming-Port", Label: "蓝牙"},
		},
		port:        "/dev/cu.usbserial-1420",
		preset:      baud.Presets[0].Name,
		duration:    durationOptions[1].Label,
		results:     results,
		selected:    0,
		status:      fmt.Sprintf("完成 · 最可能 %d baud（%d 分）· 用时 2.4 秒", results[0].Baud, results[0].Analysis.Score),
		statusLevel: statusGood,
	}
}

func TestViewRendersResults(t *testing.T) {
	a := sampleApp()
	tt := ui.NewTester(a.view, 1120, 760)
	if !tt.HasText("串口波特率扫描") {
		t.Error("missing title")
	}
	if !tt.HasText("9600") || !tt.HasText("原始字节") {
		t.Errorf("missing result details, texts: %q", tt.Texts())
	}
	if path := os.Getenv("BAUDSCAN_SNAPSHOT"); path != "" {
		writeSnapshot(t, tt, path)
	}
}

func TestViewDark(t *testing.T) {
	a := sampleApp()
	tt := ui.NewTester(a.view, 1120, 760)
	tt.SetDark(true)
	tt.Frame()
	if path := os.Getenv("BAUDSCAN_SNAPSHOT_DARK"); path != "" {
		writeSnapshot(t, tt, path)
	}
}

func TestViewSelectRow(t *testing.T) {
	a := sampleApp()
	tt := ui.NewTester(a.view, 1120, 760)
	if err := tt.Click("115200"); err != nil {
		t.Fatal(err)
	}
	if a.selected != 1 {
		t.Errorf("selected = %d, want 1", a.selected)
	}
	if !tt.HasText("不是最佳候选；最佳为 9600 baud（100 分）。") {
		t.Errorf("missing row verdict, texts: %q", tt.Texts())
	}
}

func TestViewNoData(t *testing.T) {
	a := sampleApp()
	a.results = []baud.Result{
		{Baud: 9600, Analysis: baud.Analyze(nil)},
		{Baud: 115200, Analysis: baud.Analyze(nil)},
	}
	baud.Rank(a.results)
	a.selected = -1
	a.status = "未收到任何数据 · 设备可能不会主动输出，或需要更长的采样"
	a.statusLevel = statusWarn
	tt := ui.NewTester(a.view, 1120, 760)
	if !tt.HasText("没有收到数据") {
		t.Errorf("missing no-data detail, texts: %q", tt.Texts())
	}
	if !tt.HasText("未收到任何数据 · 设备可能不会主动输出，或需要更长的采样") {
		t.Error("missing no-data status")
	}
}

func TestViewMinimumSize(t *testing.T) {
	a := sampleApp()
	tt := ui.NewTester(a.view, 980, 680)
	if !tt.HasText("候选波特率") || !tt.HasText("检测详情") {
		t.Errorf("minimum size misses cards, texts: %q", tt.Texts())
	}
	if path := os.Getenv("BAUDSCAN_SNAPSHOT_MIN"); path != "" {
		writeSnapshot(t, tt, path)
	}
}

func writeSnapshot(t *testing.T, tt *ui.Tester, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

func TestViewEmpty(t *testing.T) {
	a := &app{
		preset:      baud.Presets[0].Name,
		duration:    durationOptions[1].Label,
		selected:    -1,
		status:      "未发现串口 · 接入设备后点击刷新",
		statusLevel: statusWarn,
	}
	tt := ui.NewTester(a.view, 1120, 760)
	if !tt.HasText("未发现串口设备") {
		t.Errorf("missing empty state, texts: %q", tt.Texts())
	}
}
