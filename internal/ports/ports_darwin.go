//go:build darwin

package ports

import (
	"path/filepath"
	"strings"
)

// macOS exposes each serial device as a callout node under /dev/cu.*.
// The tty.* nodes are dial-in devices and can block on carrier detect.
func list() ([]Port, error) {
	names, err := filepath.Glob("/dev/cu.*")
	if err != nil {
		return nil, err
	}
	ports := make([]Port, 0, len(names))
	for _, name := range names {
		base := filepath.Base(name)
		lower := strings.ToLower(base)
		switch {
		case strings.Contains(lower, "bluetooth") || strings.HasSuffix(lower, "blth"):
			ports = append(ports, Port{Name: name, Label: "蓝牙"})
		case strings.Contains(lower, "usb"):
			ports = append(ports, Port{Name: name, Label: "USB 串口", USB: true})
		case strings.Contains(lower, "debug-console"):
			ports = append(ports, Port{Name: name, Label: "调试控制台"})
		default:
			ports = append(ports, Port{Name: name, Label: "串口"})
		}
	}
	return ports, nil
}
