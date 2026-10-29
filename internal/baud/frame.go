package baud

import (
	"fmt"
	"strconv"
	"strings"

	"go.bug.st/serial"
)

// Frame is the serial framing a scan uses: data bits, parity and stop bits.
// A scan varies the baud rate only and keeps the frame fixed, because a
// wrong frame garbles every rate, so scoring cannot discover it.
type Frame struct {
	DataBits int
	Parity   serial.Parity
	StopBits serial.StopBits
}

// DefaultFrame is 8N1, the framing almost every device uses.
var DefaultFrame = Frame{DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit}

// MinRate and MaxRate bound a rate typed by hand. The upper bound is well
// past any UART's ceiling, so a mistyped number cannot start a scan that
// only fails.
const (
	MinRate = 1
	MaxRate = 20_000_000
)

// Mode returns the serial mode for rate under this framing.
func (f Frame) Mode(rate int) *serial.Mode {
	return &serial.Mode{
		BaudRate: rate,
		DataBits: f.DataBits,
		Parity:   f.Parity,
		StopBits: f.StopBits,
	}
}

// Label writes the framing as serial tools do: 8N1, 8E2, 7O1.5.
func (f Frame) Label() string {
	return fmt.Sprintf("%d%s%s", f.DataBits, parityLetter(f.Parity), stopLabel(f.StopBits))
}

func parityLetter(p serial.Parity) string {
	switch p {
	case serial.NoParity:
		return "N"
	case serial.OddParity:
		return "O"
	case serial.EvenParity:
		return "E"
	case serial.MarkParity:
		return "M"
	case serial.SpaceParity:
		return "S"
	default:
		return "?"
	}
}

func stopLabel(b serial.StopBits) string {
	switch b {
	case serial.OneStopBit:
		return "1"
	case serial.OnePointFiveStopBits:
		return "1.5"
	case serial.TwoStopBits:
		return "2"
	default:
		return "?"
	}
}

// DataBitLabels are the data-bit choices, in the order the interface lists
// them. The value of each is its own label.
var DataBitLabels = []string{"5", "6", "7", "8"}

// ParityLabels are the parity choices, in the order the interface lists
// them.
var ParityLabels = []string{"无", "奇", "偶", "标记", "空格"}

// StopBitLabels are the stop-bit choices, in the order the interface lists
// them.
var StopBitLabels = []string{"1", "1.5", "2"}

// Labels returns the interface labels of the frame's three fields, in the
// order data bits, parity, stop bits.
func (f Frame) Labels() (dataBits, parity, stopBits string) {
	dataBits = strconv.Itoa(f.DataBits)
	parity = "无"
	switch f.Parity {
	case serial.OddParity:
		parity = "奇"
	case serial.EvenParity:
		parity = "偶"
	case serial.MarkParity:
		parity = "标记"
	case serial.SpaceParity:
		parity = "空格"
	}
	stopBits = stopLabel(f.StopBits)
	return dataBits, parity, stopBits
}

// FrameFrom builds a frame from the labels the interface shows. A label it
// does not know falls back to the default for that field.
func FrameFrom(dataBits, parity, stopBits string) Frame {
	f := DefaultFrame
	if bits, err := strconv.Atoi(strings.TrimSpace(dataBits)); err == nil && bits >= 5 && bits <= 8 {
		f.DataBits = bits
	}
	switch strings.TrimSpace(parity) {
	case "奇":
		f.Parity = serial.OddParity
	case "偶":
		f.Parity = serial.EvenParity
	case "标记":
		f.Parity = serial.MarkParity
	case "空格":
		f.Parity = serial.SpaceParity
	}
	switch strings.TrimSpace(stopBits) {
	case "1.5":
		f.StopBits = serial.OnePointFiveStopBits
	case "2":
		f.StopBits = serial.TwoStopBits
	}
	return f
}

// ParseRates reads rates typed by hand, such as "31250,10400". Separators
// may be commas, semicolons or spaces, ASCII or full width.
func ParseRates(text string) ([]int, error) {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		switch r {
		case ',', ';', '，', '；', '、', ' ', '\t', '\n', '\r':
			return true
		}
		return false
	})
	rates := make([]int, 0, len(fields))
	for _, field := range fields {
		rate, err := strconv.Atoi(field)
		if err != nil {
			return nil, fmt.Errorf("无法识别的波特率 %q", field)
		}
		if rate < MinRate || rate > MaxRate {
			return nil, fmt.Errorf("波特率 %d超出 %d…%d的范围", rate, MinRate, MaxRate)
		}
		rates = append(rates, rate)
	}
	return rates, nil
}

// Plan is the order a scan tries: the common rates first, then the rates
// added by hand, then every other standard rate ascending. Duplicates are
// dropped, so a rate typed again keeps the place of the standard entry.
func Plan(extra []int) []int {
	first := make([]int, 0, len(Common)+len(extra))
	first = append(first, Common...)
	first = append(first, extra...)
	return scanOrder(Standard, first)
}

// CustomRates reports the rates of plan that no standard list holds, in
// plan order.
func CustomRates(plan []int) []int {
	standard := make(map[int]bool, len(Standard))
	for _, rate := range Standard {
		standard[rate] = true
	}
	var custom []int
	for _, rate := range plan {
		if !standard[rate] {
			custom = append(custom, rate)
		}
	}
	return custom
}
