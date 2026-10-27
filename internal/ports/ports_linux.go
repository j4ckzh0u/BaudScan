//go:build linux

package ports

import (
	"path/filepath"
	"strings"
)

// Linux names serial devices by driver. USB adapters appear as ttyUSB,
// ttyACM or one of the vendor-specific names; ttyS are motherboard UARTs.
var patterns = []string{
	"/dev/ttyUSB*",
	"/dev/ttyACM*",
	"/dev/ttyAMA*",
	"/dev/ttyCH343USB*",
	"/dev/ttyCH341USB*",
	"/dev/ttyXRUSB*",
	"/dev/ttyHS*",
	"/dev/ttyO*",
	"/dev/ttymxc*",
	"/dev/rfcomm*",
	"/dev/ttyS*",
}

func list() ([]Port, error) {
	seen := make(map[string]bool)
	var ports []Port
	for _, pattern := range patterns {
		names, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			if seen[name] {
				continue
			}
			seen[name] = true
			base := filepath.Base(name)
			lower := strings.ToLower(base)
			switch {
			case strings.Contains(lower, "usb"), strings.Contains(lower, "acm"):
				ports = append(ports, Port{Name: name, Label: "USB 串口", USB: true})
			case strings.HasPrefix(base, "ttyS"):
				ports = append(ports, Port{Name: name, Label: "主板串口"})
			case strings.HasPrefix(base, "rfcomm"):
				ports = append(ports, Port{Name: name, Label: "蓝牙"})
			default:
				ports = append(ports, Port{Name: name, Label: "串口"})
			}
		}
	}
	return ports, nil
}
