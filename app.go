package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"baudscan/internal/baud"
	"baudscan/internal/ports"
)

type statusLevel int

const (
	statusNormal statusLevel = iota
	statusGood
	statusWarn
	statusBad
)

type app struct {
	win *mygo.Window

	scanner   *baud.Scanner
	listPorts func() ([]ports.Port, error)

	ports []ports.Port
	port  string

	preset   string
	duration string

	scanning    bool
	cancel      context.CancelFunc
	scanStarted time.Time
	progress    float64
	status      string
	statusLevel statusLevel
	previewFor  string
	previewText string

	results  []baud.Result
	selected int
	table    ui.ListState
}

var durationOptions = []struct {
	Label string
	Value time.Duration
}{
	{"0.6 秒", 600 * time.Millisecond},
	{"1.0 秒", time.Second},
	{"2.0 秒", 2 * time.Second},
	{"5.0 秒", 5 * time.Second},
}

func newApp() *app {
	a := &app{
		scanner:   baud.NewScanner(),
		listPorts: ports.List,
		preset:    baud.Presets[0].Name,
		duration:  durationOptions[1].Label,
		selected:  -1,
	}
	a.refreshPorts(false)
	return a
}

// refreshPorts replaces the port list and keeps the user's choice when it
// is still present. A refresh the user asked for reports what it found.
func (a *app) refreshPorts(announce bool) {
	list, err := a.listPorts()
	if err != nil {
		a.ports = nil
		a.port = ""
		if a.selected >= 0 {
			a.results = nil
			a.selected = -1
		}
		a.status = "无法列出串口：" + err.Error()
		a.statusLevel = statusBad
		return
	}

	a.ports = list

	found := false
	for _, p := range list {
		if p.Name == a.port {
			found = true
			break
		}
	}
	if !found {
		a.port = ""
		if len(list) > 0 {
			a.port = list[0].Name
		}
		// Results belong to the port that produced them.
		a.results = nil
		a.selected = -1
	}

	label := a.portLabel(a.port)
	switch {
	case len(list) == 0:
		a.status = "未发现串口 · 接入设备后点击刷新"
		a.statusLevel = statusWarn
	case announce:
		a.status = fmt.Sprintf("已刷新 · %d 个串口 · 当前 %s（%s）", len(list), a.port, label)
		a.statusLevel = statusNormal
	case a.status == "":
		a.status = fmt.Sprintf("就绪 · %d 个串口 · 当前 %s（%s）", len(list), a.port, label)
		a.statusLevel = statusNormal
	}
}

func (a *app) selectedPreset() baud.Preset {
	for _, p := range baud.Presets {
		if p.Name == a.preset {
			return p
		}
	}
	return baud.Presets[0]
}

func (a *app) sampleDuration() time.Duration {
	for _, d := range durationOptions {
		if d.Label == a.duration {
			return d.Value
		}
	}
	return time.Second
}

func (a *app) startScan() {
	if a.scanning || a.port == "" {
		return
	}
	preset := a.selectedPreset()
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.scanning = true
	a.scanStarted = time.Now()
	a.progress = 0
	a.results = nil
	a.selected = -1
	a.previewFor = ""
	a.status = "正在打开串口…"
	a.statusLevel = statusNormal

	win := a.win
	opts := baud.ScanOptions{
		Port:   a.port,
		Rates:  preset.Rates,
		Sample: a.sampleDuration(),
		OnEvent: func(ev baud.Event) {
			// The callback runs on the scanning goroutine. Window.Update
			// moves these changes to the UI thread.
			a.update(win, func() {
				if ev.Result == nil {
					a.progress = float64(ev.Index) / float64(ev.Total)
					a.status = fmt.Sprintf("正在测试 %d baud · %d/%d", ev.Baud, ev.Index+1, ev.Total)
					a.statusLevel = statusNormal
					return
				}
				a.results = append(a.results, *ev.Result)
				baud.Rank(a.results)
				if len(a.results) > 0 && (a.selected < 0 || a.selected >= len(a.results)) && a.results[0].Bytes > 0 {
					a.selected = 0
				}
				a.progress = float64(ev.Index+1) / float64(ev.Total)
				a.status = fmt.Sprintf("已采样 %d baud · %d/%d", ev.Baud, ev.Index+1, ev.Total)
				a.statusLevel = statusNormal
			})
		},
	}

	go func() {
		results, err := a.scanner.Scan(ctx, opts)
		elapsed := time.Since(a.scanStarted)
		a.update(win, func() {
			a.scanning = false
			a.cancel = nil

			if len(results) > 0 {
				a.results = results
				baud.Rank(a.results)
			}
			if len(a.results) > 0 && a.results[0].Bytes > 0 {
				a.selected = 0
			} else {
				a.selected = -1
			}
			a.progress = 1
			a.finishScan(err, elapsed)
		})
	}()
}

func (a *app) stopScan() {
	if a.cancel != nil {
		a.cancel()
		a.status = "正在停止…"
		a.statusLevel = statusNormal
	}
}

func (a *app) finishScan(err error, elapsed time.Duration) {
	switch {
	case errors.Is(err, context.Canceled):
		a.status = fmt.Sprintf("已停止 · 已完成 %d 档", len(a.results))
		a.statusLevel = statusNormal
		return
	case err != nil:
		a.status = "扫描中断 · " + baud.FriendlyError(err)
		a.statusLevel = statusBad
		return
	}
	if len(a.results) == 0 {
		a.status = "扫描没有结果"
		a.statusLevel = statusWarn
		return
	}

	if !a.hasData() {
		a.status = "未收到任何数据 · 设备可能不会主动输出，或需要更长的采样"
		a.statusLevel = statusWarn
		return
	}

	best := a.results[0]
	switch {
	case best.Analysis.Score >= 60:
		a.status = fmt.Sprintf("完成 · 最可能 %d baud（%d 分）· 用时 %s",
			best.Baud, best.Analysis.Score, shortDuration(elapsed))
		a.statusLevel = statusGood
	default:
		a.status = fmt.Sprintf("完成 · 未找到可信波特率（最佳 %d 分）· 请查看候选样本",
			best.Analysis.Score)
		a.statusLevel = statusWarn
	}
	if a.closeRace() {
		a.status += " · 前两名接近，请对比样本"
		a.statusLevel = statusWarn
	}
}

func (a *app) portLabel(name string) string {
	for _, p := range a.ports {
		if p.Name == name {
			return p.Label
		}
	}
	return "串口"
}

func (a *app) hasData() bool {
	for _, r := range a.results {
		if r.Bytes > 0 {
			return true
		}
	}
	return false
}

// closeRace reports whether the two best candidates are too close to call.
func (a *app) closeRace() bool {
	if len(a.results) < 2 {
		return false
	}
	best, second := a.results[0].Analysis.Score, a.results[1].Analysis.Score
	return best >= 30 && best-second <= 6
}

func (a *app) margin() int {
	if len(a.results) < 2 {
		return 0
	}
	return a.results[0].Analysis.Score - a.results[1].Analysis.Score
}

// update runs fn on the UI thread. In tests there is no window, and the
// view reads the same goroutine's state.
func (a *app) update(win *mygo.Window, fn func()) {
	if win == nil {
		fn()
		return
	}
	win.Update(fn)
}

func shortDuration(d time.Duration) string {
	if d < 10*time.Second {
		return fmt.Sprintf("%.1f 秒", d.Seconds())
	}
	return fmt.Sprintf("%.0f 秒", d.Seconds())
}
