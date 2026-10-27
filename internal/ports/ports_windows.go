//go:build windows

package ports

import (
	"sort"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// Windows lists COM ports in the registry; this includes ports of USB
// adapters and of Bluetooth serial devices.
func list() ([]Port, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DEVICEMAP\SERIALCOMM`, registry.QUERY_VALUE)
	if err != nil {
		// No serial ports have ever been installed.
		return nil, nil
	}
	defer key.Close()

	names, err := key.ReadValueNames(0)
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	var ports []Port
	seen := make(map[string]bool)
	for _, name := range names {
		value, _, err := key.GetStringValue(name)
		if err != nil || value == "" {
			continue
		}
		upper := strings.ToUpper(value)
		if seen[upper] {
			continue
		}
		seen[upper] = true
		ports = append(ports, Port{Name: value, Label: "串口"})
	}
	return ports, nil
}
