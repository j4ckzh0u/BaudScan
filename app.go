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

	duration string

	scanning    bool
	cancel      context.CancelFunc
	scanStarted time.Time
	progress    float64
	status      string
	statusLevel statusLevel
	previewFor  string
	previewText string

	results    []baud.Result
	selected   int
	manualPick bool
	table      ui.ListState
}

var durationOptions = []struct {
	Label string
	Value time.Duration
}{
	{"0.4秒", 400 * time.Millisecond},
	{"0.6秒", 600 * time.Millisecond},
	{"1.0秒", time.Second},
	{"2.0秒", 2 * time.Second},
	{"5.0秒", 5 * time.Second},
}

func newApp() *app {
	a := &app{
		scanner:   baud.NewScanner(),
		listPorts: ports.List,
		duration:  durationOptions[2].Label,
	}
	a.resetResults()
	a.refreshPorts(false)
	return a
}

// resetResults lists every rate before a scan starts, so the table shows the
// whole set that will be tested. Store fills the rows in place.
func (a *app) resetResults() {
	a.results = make([]baud.Result, len(baud.All))
	for i, rate := range baud.All {
		a.results[i] = baud.Result{Baud: rate, Pending: true}
	}
	a.selected = -1
	a.manualPick = false
	a.previewFor = ""
}

// refreshPorts replaces the port list and keeps the user's choice when it is
// still present. A refresh the user asked for reports what it found.
func (a *app) refreshPorts(announce bool) {
	list, err := a.listPorts()
	if err != nil {
		a.ports = nil
		a.port = ""
		a.resetResults()
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
		a.resetResults()
	}

	label := a.portLabel(a.port)
	switch {
	case len(list) == 0:
		a.status = "未发现串口 ·接入设备后点击刷新"
		a.statusLevel = statusWarn
	case announce:
		a.status = fmt.Sprintf("已刷新 · %d个串口 ·当前 %s（%s）·将测试全部 %d档",
			len(list), a.port, label, len(baud.All))
		a.statusLevel = statusNormal
	case a.status == "":
		a.status = fmt.Sprintf("就绪 · %d个串口 ·当前 %s（%s）·将测试全部 %d档",
			len(list), a.port, label, len(baud.All))
		a.statusLevel = statusNormal
	}
}

func (a *app) sampleDuration() time.Duration {
	for _, d := range durationOptions {
		if d.Label == a.duration {
			return d.Value
		}
	}
	return time.Second
}

// estimate is how long a full scan takes with the current sample length.
func (a *app) estimate() time.Duration {
	n := time.Duration(len(baud.All))
	return n*(a.sampleDuration()+baud.DefaultSettle) + baud.OpenDelay
}

func (a *app) startScan() {
	if a.scanning || a.port == "" {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.scanning = true
	a.scanStarted = time.Now()
	a.progress = 0
	a.resetResults()
	a.status = fmt.Sprintf("正在打开串口 ·将测试全部 %d档", len(baud.All))
	a.statusLevel = statusNormal

	win := a.win
	opts := baud.ScanOptions{
		Port:   a.port,
		Rates:  baud.All,
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
				a.store(*ev.Result)
				a.progress = float64(ev.Index+1) / float64(ev.Total)
				a.status = fmt.Sprintf("已采样 %d baud · %d/%d · %s",
					ev.Baud, ev.Index+1, ev.Total, baud.KindLabel(ev.Result.Analysis.Kind))
				a.statusLevel = statusNormal
				a.selectBest()
			})
		},
	}

	go func() {
		results, err := a.scanner.Scan(ctx, opts)
		elapsed := time.Since(a.scanStarted)
		a.update(win, func() {
			a.scanning = false
			a.cancel = nil
			for _, res := range results {
				a.store(res)
			}
			a.progress = 1
			a.selectBest()
			a.finishScan(err, elapsed)
		})
	}()
}

// store puts a completed sample in the row of its rate, keeping the table's
// full list of rates intact, and re-sorts the rows by score.
func (a *app) store(res baud.Result) {
	for i := range a.results {
		if a.results[i].Baud == res.Baud {
			res.Pending = false
			a.results[i] = res
			break
		}
	}
	baud.Rank(a.results)
}

// selectBest follows the best result that has data, so the detail panel
// shows something useful as soon as a usable sample arrives. A row the user
// clicked stays chosen until the next scan.
func (a *app) selectBest() {
	if a.manualPick {
		return
	}
	for i, r := range a.results {
		if r.Pending || r.Bytes == 0 {
			continue
		}
		a.selected = i
		return
	}
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
		a.status = fmt.Sprintf("已停止 ·已完成 %d/%d档", a.doneCount(), len(baud.All))
		a.statusLevel = statusNormal
		return
	case err != nil:
		a.status = "扫描中断 · " + baud.FriendlyError(err)
		a.statusLevel = statusBad
		return
	}

	if !a.hasData() {
		a.status = fmt.Sprintf("未收到任何数据 ·测试了 %d档，其中 %d档被驱动拒绝",
			a.doneCount(), a.unsupportedCount())
		if a.unsupportedCount() == 0 {
			a.status = "未收到任何数据 ·设备可能不会主动输出，或需要更长的采样"
		}
		a.statusLevel = statusWarn
		return
	}

	best := a.results[0]
	switch {
	case best.Analysis.Score >= 60:
		a.status = fmt.Sprintf("完成 ·最可能 %d baud（%d分）·测试了全部 %d档 ·用时 %s",
			best.Baud, best.Analysis.Score, len(baud.All), shortDuration(elapsed))
		a.statusLevel = statusGood
	default:
		a.status = fmt.Sprintf("完成 ·未找到可信波特率（最佳 %d分）·请查看候选样本",
			best.Analysis.Score)
		a.statusLevel = statusWarn
	}
	if a.closeRace() {
		a.status += " ·前两名接近，请对比样本"
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

func (a *app) unsupportedCount() int {
	n := 0
	for _, r := range a.results {
		if r.Unsupported {
			n++
		}
	}
	return n
}

func (a *app) doneCount() int {
	n := 0
	for _, r := range a.results {
		if !r.Pending {
			n++
		}
	}
	return n
}

// closeRace reports whether the two best candidates are too close to call.
func (a *app) closeRace() bool {
	if len(a.results) < 2 {
		return false
	}
	if a.results[0].Pending || a.results[1].Pending {
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
		return fmt.Sprintf("%.1f秒", d.Seconds())
	}
	return fmt.Sprintf("%.0f秒", d.Seconds())
}
