package main

import (
	"fmt"
	"image/png"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/j4ckzh0u/BaudScan/internal/baud"
	"github.com/j4ckzh0u/BaudScan/internal/ports"
)

func sampleApp() *app {
	text := []byte(strings.Repeat("$GNGGA,123519,4807.038,N,01131.000,E,1,08,0.9,545.4,M,46.9,M,,*47\r\n", 8))
	r := rand.New(rand.NewPCG(11, 12))
	noise := make([]byte, 900)
	for i := range noise {
		noise[i] = byte(r.Uint32())
	}
	a := &app{
		ports: []ports.Port{
			{Name: "/dev/cu.usbserial-1420", Label: "USB串口", USB: true},
			{Name: "/dev/cu.Bluetooth-Incoming-Port", Label: "蓝牙"},
		},
		port:        "/dev/cu.usbserial-1420",
		duration:    durationOptions[1].Label,
		dataBits:    "8",
		parity:      "无",
		stopBits:    "1",
		statusLevel: statusGood,
	}
	a.plan = baud.All
	a.resetResults()
	// Half the rates are sampled; the rest stay pending, as during a scan.
	for i, rate := range baud.All[:len(baud.All)/2] {
		var res baud.Result
		switch {
		case i == 0:
			res = baud.Result{Baud: rate, Bytes: len(noise), Data: noise, Analysis: baud.Analyze(noise)}
		case i == 1:
			res = baud.Result{Baud: rate, Bytes: len(text), Data: text, Analysis: baud.Analyze(text)}
		default:
			res = baud.Result{Baud: rate, Analysis: baud.Analyze(nil)}
		}
		a.store(res)
	}
	a.selectBest()
	a.status = fmt.Sprintf("完成 ·最可能 %d baud（%d分）·测试了全部 %d档", a.results[0].Baud, a.results[0].Analysis.Score, len(baud.All))
	return a
}

func TestViewRendersResults(t *testing.T) {
	a := sampleApp()
	tt := ui.NewTester(a.view, 1120, 760)
	if !tt.HasText("串口波特率扫描") {
		t.Error("missing title")
	}
	if !tt.HasText("波特率测试结果") || !tt.HasText("原始字节") {
		t.Errorf("missing result card or details, texts: %q", tt.Texts())
	}
	if !tt.HasText("待测") {
		t.Errorf("pending rows are not marked, texts: %q", tt.Texts())
	}
	if !tt.HasText(fmt.Sprintf("全部 %d档 ·预计", len(baud.All))) {
		t.Logf("coverage line not found; texts: %q", tt.Texts())
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
	if err := tt.Click(baud.BaudLabel(baud.All[2])); err != nil {
		t.Fatal(err)
	}
	if !a.manualPick {
		t.Error("a click did not pin the choice")
	}
	row := a.results[a.selected]
	if row.Baud != baud.All[2] || row.Pending {
		t.Errorf("selected = %+v, want the clicked row %d", row, baud.All[2])
	}
	want := fmt.Sprintf("不是最佳候选；最佳为 %d baud（%d分）。", a.results[0].Baud, a.results[0].Analysis.Score)
	if !tt.HasText(want) {
		t.Errorf("missing row verdict %q, texts: %q", want, tt.Texts())
	}
}

func TestViewNoData(t *testing.T) {
	a := sampleApp()
	a.resetResults()
	a.selected = -1
	a.status = "未收到任何数据 ·设备可能不会主动输出，或需要更长的采样"
	a.statusLevel = statusWarn
	tt := ui.NewTester(a.view, 1120, 760)
	if !tt.HasText("还没有可用样本") {
		t.Errorf("missing no-data detail, texts: %q", tt.Texts())
	}
	if !tt.HasText("未收到任何数据 ·设备可能不会主动输出，或需要更长的采样") {
		t.Error("missing no-data status")
	}
	if !tt.HasText("待测") {
		t.Error("missing pending rows")
	}
}

func TestViewMinimumSize(t *testing.T) {
	a := sampleApp()
	tt := ui.NewTester(a.view, 980, 680)
	if !tt.HasText("波特率测试结果") || !tt.HasText("检测详情") {
		t.Errorf("minimum size misses cards, texts: %q", tt.Texts())
	}
	if path := os.Getenv("BAUDSCAN_SNAPSHOT_MIN"); path != "" {
		writeSnapshot(t, tt, path)
	}
}

func TestViewEmpty(t *testing.T) {
	a := &app{
		duration:    durationOptions[1].Label,
		dataBits:    "8",
		parity:      "无",
		stopBits:    "1",
		selected:    -1,
		status:      "未发现串口 ·接入设备后点击刷新",
		statusLevel: statusWarn,
		plan:        baud.All,
	}
	tt := ui.NewTester(a.view, 1120, 760)
	if !tt.HasText("未发现串口设备") {
		t.Errorf("missing empty state, texts: %q", tt.Texts())
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

func TestViewUnsupportedRow(t *testing.T) {
	a := sampleApp()
	a.store(baud.Result{Baud: 250000, Err: baud.FriendlyError(syscall.EINVAL), Unsupported: true})
	a.manualPick = true
	a.selected = 0
	for i, r := range a.results {
		if r.Baud == 250000 {
			a.selected = i
			break
		}
	}
	tt := ui.NewTester(a.view, 1120, 760)
	if !tt.HasText("设备不支持") || !tt.HasText("驱动不支持该速率") {
		t.Errorf("missing unsupported detail, texts: %q", tt.Texts())
	}
	if path := os.Getenv("BAUDSCAN_SNAPSHOT_UNSUPPORTED"); path != "" {
		writeSnapshot(t, tt, path)
	}
}

func TestViewShowsFrameAndCustomRateControls(t *testing.T) {
	a := sampleApp()
	tt := ui.NewTester(a.view, 1120, 760)
	if !tt.HasText("数据位") || !tt.HasText("校验位") || !tt.HasText("停止位") {
		t.Errorf("missing framing controls, texts: %q", tt.Texts())
	}
	if !tt.HasText("自定义波特率") {
		t.Errorf("missing the custom rate field, texts: %q", tt.Texts())
	}
	if !tt.HasText("帧格式") || !tt.HasText("8N1") {
		t.Errorf("missing the frame hint, texts: %q", tt.Texts())
	}
	if path := os.Getenv("BAUDSCAN_SNAPSHOT"); path != "" {
		writeSnapshot(t, tt, path)
	}
}

// clickBelow clicks the element just under the label s, where a field's
// input sits: a text input has no text of its own to click.
func clickBelow(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("no element shows %q", label)
	}
	tt.ClickAt(r.X+r.W/2, r.Y+r.H+14)
	tt.Frame()
}

func TestViewTypingCustomRateUpdatesTable(t *testing.T) {
	a := sampleApp()
	tt := ui.NewTester(a.view, 1120, 760)
	clickBelow(t, tt, "自定义波特率")
	tt.Type("31250")
	tt.Frame()
	if a.customRates != "31250" {
		t.Fatalf("customRates = %q, want the typed text", a.customRates)
	}
	if !slices.Contains(a.plan, 31250) {
		t.Errorf("plan = %v, want it to hold the typed rate", a.plan)
	}
	if !tt.HasText("31250") {
		t.Errorf("the typed rate is not in the table, texts: %q", tt.Texts())
	}
	if !tt.HasText("（31250）") {
		t.Errorf("the hint does not list the custom rate, texts: %q", tt.Texts())
	}
}

func TestViewRejectsUnreadableCustomRate(t *testing.T) {
	a := sampleApp()
	tt := ui.NewTester(a.view, 1120, 760)
	clickBelow(t, tt, "自定义波特率")
	tt.Type("abc")
	tt.Frame()
	if a.customErr == "" {
		t.Fatal("customErr is empty for unreadable text")
	}
	if !tt.HasText("自定义波特率无效") {
		t.Errorf("missing the error hint, texts: %q", tt.Texts())
	}
	if a.statusLevel != statusBad {
		t.Errorf("status level = %v, want bad", a.statusLevel)
	}
}
