package baud

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"go.bug.st/serial"
)

// Preset is a set of baud rates to try, in the order that finds common
// devices first.
type Preset struct {
	Name  string
	Rates []int
}

// Common holds the rates of most serial devices.
var Common = []int{115200, 9600, 57600, 38400, 19200, 4800, 2400, 1200}

// Extended adds the rates of faster and older devices.
var Extended = []int{
	115200, 9600, 57600, 38400, 230400, 19200, 460800, 4800,
	921600, 250000, 2400, 1200, 500000, 300, 600,
}

// Presets are the scan ranges offered by the interface.
var Presets = []Preset{
	{Name: "常用", Rates: Common},
	{Name: "扩展", Rates: Extended},
}

// Result is what one baud rate produced.
type Result struct {
	Baud     int
	Bytes    int
	Data     []byte // the first few kilobytes, for preview
	Analysis Analysis
	Err      string
}

// Rank sorts results from most to least likely, breaking ties by how
// commonly a rate is used.
func Rank(results []Result) {
	order := make(map[int]int, len(Extended))
	for i, rate := range Extended {
		order[rate] = i
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Analysis.Score != results[j].Analysis.Score {
			return results[i].Analysis.Score > results[j].Analysis.Score
		}
		oi, iok := order[results[i].Baud]
		oj, jok := order[results[j].Baud]
		switch {
		case iok && jok:
			return oi < oj
		case iok:
			return true
		case jok:
			return false
		}
		return results[i].Baud > results[j].Baud
	})
}

// Event reports the progress of a scan. Result is nil when a baud rate is
// about to be sampled, and set when its sample is done.
type Event struct {
	Index  int
	Total  int
	Baud   int
	Result *Result
}

// ScanOptions configures a scan.
type ScanOptions struct {
	Port  string
	Rates []int

	// Sample is the longest time spent on one baud rate. IdleAfter ends a
	// sample early once data has arrived and then stopped for that long,
	// and MinSample is the least time a sample with data takes.
	Sample    time.Duration
	IdleAfter time.Duration
	MinSample time.Duration
	Settle    time.Duration

	// OnEvent, when set, is called from the scanning goroutine before and
	// after each baud rate.
	OnEvent func(Event)
}

const (
	// DefaultSample is long enough for devices that emit a line per second.
	DefaultSample = 900 * time.Millisecond
	maxCapture    = 256 << 10
	previewBytes  = 4096
)

// Scanner samples a port. Open is serial.Open in the app, and a fake in
// tests.
type Scanner struct {
	Open func(name string, mode *serial.Mode) (serial.Port, error)
}

// NewScanner returns a Scanner that opens real serial ports.
func NewScanner() *Scanner {
	return &Scanner{Open: serial.Open}
}

// Scan opens the port once and samples every rate in turn. Opening once
// matters: many boards reset when their port opens, and a reset would hide
// the device's output from later probes. Changing the baud rate of an open
// port does not touch DTR.
//
// Scan never writes to the device.
func (s *Scanner) Scan(ctx context.Context, opts ScanOptions) ([]Result, error) {
	rates := cleanRates(opts.Rates)
	if opts.Port == "" {
		return nil, errors.New("未选择串口")
	}
	if len(rates) == 0 {
		return nil, errors.New("没有要测试的波特率")
	}
	opts = withDefaults(opts)
	open := s.Open
	if open == nil {
		open = serial.Open
	}

	mode := &serial.Mode{BaudRate: rates[0], DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit}
	port, err := open(opts.Port, mode)
	if err != nil {
		return nil, fmt.Errorf("打开串口失败: %w", err)
	}
	defer port.Close()

	if err := port.SetReadTimeout(40 * time.Millisecond); err != nil {
		return nil, fmt.Errorf("设置串口读超时失败: %w", err)
	}

	results := make([]Result, 0, len(rates))
	// Opening a USB port can reset the board. Let it boot and flush the
	// messages it printed before we start judging rates.
	if !sleep(ctx, 220*time.Millisecond) {
		return results, ctx.Err()
	}
	_ = port.ResetInputBuffer()

	for i, rate := range rates {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		if i > 0 {
			if err := port.SetMode(&serial.Mode{BaudRate: rate, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit}); err != nil {
				res := Result{Baud: rate, Err: FriendlyError(err)}
				results = append(results, res)
				emit(opts, Event{Index: i, Total: len(rates), Baud: rate, Result: &results[len(results)-1]})
				continue
			}
			if !sleep(ctx, opts.Settle) {
				return results, ctx.Err()
			}
			_ = port.ResetInputBuffer()
		}
		emit(opts, Event{Index: i, Total: len(rates), Baud: rate})

		data, err := readWindow(ctx, port, opts)
		if err != nil {
			if ctx.Err() != nil {
				return results, ctx.Err()
			}
			res := Result{Baud: rate, Bytes: len(data), Err: FriendlyError(err)}
			if len(data) > 0 {
				res.Analysis = Analyze(data)
			}
			results = append(results, res)
			emit(opts, Event{Index: i, Total: len(rates), Baud: rate, Result: &results[len(results)-1]})
			return results, err
		}
		analysis := Analyze(data)
		res := Result{Baud: rate, Bytes: len(data), Data: trimPreview(data), Analysis: analysis}
		results = append(results, res)
		emit(opts, Event{Index: i, Total: len(rates), Baud: rate, Result: &results[len(results)-1]})
	}
	return results, nil
}

func withDefaults(opts ScanOptions) ScanOptions {
	if opts.Sample <= 0 {
		opts.Sample = DefaultSample
	}
	if opts.Settle <= 0 {
		opts.Settle = 45 * time.Millisecond
	}
	if opts.IdleAfter <= 0 {
		opts.IdleAfter = 140 * time.Millisecond
	}
	if opts.MinSample <= 0 {
		opts.MinSample = 320 * time.Millisecond
		if opts.MinSample > opts.Sample {
			opts.MinSample = opts.Sample
		}
	}
	return opts
}

func cleanRates(rates []int) []int {
	seen := make(map[int]bool, len(rates))
	out := make([]int, 0, len(rates))
	for _, rate := range rates {
		if rate > 0 && !seen[rate] {
			seen[rate] = true
			out = append(out, rate)
		}
	}
	return out
}

// readWindow reads until the sample time is up, or until the device has
// sent data and then gone quiet. With an incorrect baud rate a stream
// becomes random bytes, but those still arrive continuously; the idle rule
// therefore shortens a good busy sample instead of favoring it.
func readWindow(ctx context.Context, port serial.Port, opts ScanOptions) ([]byte, error) {
	start := time.Now()
	deadline := start.Add(opts.Sample)
	last := start
	buf := make([]byte, 8192)
	var out []byte
	for {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		n, err := port.Read(buf)
		now := time.Now()
		if err != nil {
			return out, err
		}
		if n > 0 {
			out = append(out, buf[:n]...)
			last = now
			if len(out) >= maxCapture {
				break
			}
		}
		if !now.Before(deadline) {
			break
		}
		if len(out) > 0 && now.Sub(last) >= opts.IdleAfter && now.Sub(start) >= opts.MinSample {
			break
		}
	}
	return out, nil
}

func trimPreview(data []byte) []byte {
	if len(data) <= previewBytes {
		return data
	}
	return data[:previewBytes]
}

func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func emit(opts ScanOptions, event Event) {
	if opts.OnEvent != nil {
		opts.OnEvent(event)
	}
}

// FriendlyError turns the serial package's errors into Chinese advice.
func FriendlyError(err error) string {
	var pe serial.PortError
	if errors.As(err, &pe) {
		switch pe.Code() {
		case serial.PortBusy:
			return "串口被占用，可能正被其他程序使用"
		case serial.PortNotFound:
			return "串口不存在，设备可能已拔出"
		case serial.PermissionDenied:
			return "没有访问权限（Linux 请把用户加入 dialout 组）"
		case serial.InvalidSpeed:
			return "此设备不支持该波特率"
		case serial.PortClosed:
			return "串口已断开"
		}
	}
	return err.Error()
}
