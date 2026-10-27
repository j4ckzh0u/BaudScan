// Package ports lists the serial ports of the machine without cgo, so the
// app can be cross-compiled from any OS.
package ports

import (
	"sort"
	"strings"
)

// Port is a serial port the user can scan.
type Port struct {
	Name  string // the device path, as serial.Open takes it
	Label string // a short kind: USB 串口, 蓝牙, 主板串口
	USB   bool
}

// List returns the serial ports present now, USB devices first.
func List() ([]Port, error) {
	ports, err := list()
	if err != nil {
		return nil, err
	}
	sortPorts(ports)
	return ports, nil
}

func sortPorts(ports []Port) {
	sort.SliceStable(ports, func(i, j int) bool {
		if ports[i].USB != ports[j].USB {
			return ports[i].USB
		}
		return naturalLess(ports[i].Name, ports[j].Name)
	})
}

// naturalLess compares names so that ttyUSB2 comes before ttyUSB10.
func naturalLess(a, b string) bool {
	for a != "" && b != "" {
		ai := digitPrefix(a)
		bi := digitPrefix(b)
		if ai > 0 && bi > 0 {
			an := strings.TrimLeft(a[:ai], "0")
			bn := strings.TrimLeft(b[:bi], "0")
			if len(an) != len(bn) {
				return len(an) < len(bn)
			}
			if an != bn {
				return an < bn
			}
			a, b = a[ai:], b[bi:]
			continue
		}
		ar, as := firstRune(a)
		br, bs := firstRune(b)
		if ar != br {
			return ar < br
		}
		a, b = a[as:], b[bs:]
	}
	return len(a) < len(b)
}

func digitPrefix(s string) int {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}

func firstRune(s string) (rune, int) {
	for _, r := range s {
		return r, len(string(r))
	}
	return 0, 0
}
