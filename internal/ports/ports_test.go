package ports

import (
	"reflect"
	"testing"
)

func TestNaturalLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"/dev/ttyUSB2", "/dev/ttyUSB10", true},
		{"/dev/ttyUSB10", "/dev/ttyUSB2", false},
		{"COM2", "COM10", true},
		{"COM10", "COM2", false},
		{"/dev/cu.usbmodem1", "/dev/cu.usbserial1", true},
		{"/dev/ttyS1", "/dev/ttyS1", false},
	}
	for _, c := range cases {
		if got := naturalLess(c.a, c.b); got != c.want {
			t.Errorf("naturalLess(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestSortPortsPutsUSBFirst(t *testing.T) {
	got := []Port{
		{Name: "/dev/ttyS0", Label: "主板串口"},
		{Name: "/dev/ttyUSB10", Label: "USB 串口", USB: true},
		{Name: "/dev/ttyUSB2", Label: "USB 串口", USB: true},
	}
	sortPorts(got)
	want := []string{"/dev/ttyUSB2", "/dev/ttyUSB10", "/dev/ttyS0"}
	names := make([]string, len(got))
	for i, p := range got {
		names[i] = p.Name
	}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("order = %v, want %v", names, want)
	}
}
