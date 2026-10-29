package baud

import (
	"reflect"
	"testing"

	"go.bug.st/serial"
)

func TestPlanPutsCommonThenCustomThenStandard(t *testing.T) {
	plan := Plan([]int{31250})
	if len(plan) != len(Standard)+1 {
		t.Fatalf("plan = %d rates, want %d", len(plan), len(Standard)+1)
	}
	for i, rate := range Common {
		if plan[i] != rate {
			t.Errorf("plan[%d] = %d, want common rate %d", i, plan[i], rate)
		}
	}
	if plan[len(Common)] != 31250 {
		t.Errorf("plan[%d] = %d, want the custom rate", len(Common), plan[len(Common)])
	}
	seen := make(map[int]bool, len(plan))
	for _, rate := range plan {
		if seen[rate] {
			t.Errorf("rate %d appears twice", rate)
		}
		seen[rate] = true
	}
	if !seen[31250] {
		t.Error("the custom rate is missing")
	}
}

func TestPlanKeepsStandardOrderWhenARateIsTypedAgain(t *testing.T) {
	plan := Plan([]int{9600, 31250})
	if !reflect.DeepEqual(plan[:len(Common)], Common) {
		t.Errorf("plan = %v, want the common rates first", plan[:len(Common)])
	}
	if plan[len(Common)] != 31250 {
		t.Errorf("plan[%d] = %d, want only the new rate after the common ones", len(Common), plan[len(Common)])
	}
}

func TestCustomRates(t *testing.T) {
	if got := CustomRates(Plan(nil)); len(got) != 0 {
		t.Errorf("custom rates = %v, want none", got)
	}
	if got := CustomRates(Plan([]int{31250, 9600})); !reflect.DeepEqual(got, []int{31250}) {
		t.Errorf("custom rates = %v, want [31250]", got)
	}
}

func TestParseRates(t *testing.T) {
	cases := []struct {
		text string
		want []int
	}{
		{"", nil},
		{"31250", []int{31250}},
		{"31250,10400", []int{31250, 10400}},
		{"31250，10400；7600,38400；1200", []int{31250, 10400, 7600, 38400, 1200}},
		{"250000 ", []int{250000}},
	}
	for _, c := range cases {
		got, err := ParseRates(c.text)
		if err != nil {
			t.Errorf("ParseRates(%q) error: %v", c.text, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("ParseRates(%q) = %v, want %v", c.text, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("ParseRates(%q) = %v, want %v", c.text, got, c.want)
				break
			}
		}
	}
}

func TestParseRatesRejectsBadInput(t *testing.T) {
	for _, text := range []string{"abc", "9600x", "0", "-1200", "30000000", "9600,,x"} {
		if got, err := ParseRates(text); err == nil {
			t.Errorf("ParseRates(%q) = %v, want an error", text, got)
		}
	}
}

func TestFrameLabel(t *testing.T) {
	cases := []struct {
		frame Frame
		want  string
	}{
		{Frame{8, serial.NoParity, serial.OneStopBit}, "8N1"},
		{Frame{7, serial.EvenParity, serial.TwoStopBits}, "7E2"},
		{Frame{8, serial.OddParity, serial.OnePointFiveStopBits}, "8O1.5"},
		{Frame{5, serial.MarkParity, serial.OneStopBit}, "5M1"},
		{Frame{6, serial.SpaceParity, serial.TwoStopBits}, "6S2"},
	}
	for _, c := range cases {
		if got := c.frame.Label(); got != c.want {
			t.Errorf("Label() = %q, want %q", got, c.want)
		}
	}
}

func TestFrameFromLabelsRoundTrips(t *testing.T) {
	frames := []Frame{
		DefaultFrame,
		{7, serial.EvenParity, serial.TwoStopBits},
		{5, serial.OddParity, serial.OnePointFiveStopBits},
		{8, serial.MarkParity, serial.OneStopBit},
		{6, serial.SpaceParity, serial.OneStopBit},
	}
	for _, want := range frames {
		dataBits, parity, stopBits := want.Labels()
		if got := FrameFrom(dataBits, parity, stopBits); got != want {
			t.Errorf("FrameFrom(%q, %q, %q) = %+v, want %+v", dataBits, parity, stopBits, got, want)
		}
	}
}

func TestFrameFromFallsBackOnUnknownLabels(t *testing.T) {
	if got := FrameFrom("9", "x", "3"); got != DefaultFrame {
		t.Errorf("FrameFrom with unknown labels = %+v, want the default frame", got)
	}
	if got := FrameFrom("", "", ""); got != DefaultFrame {
		t.Errorf("FrameFrom with empty labels = %+v, want the default frame", got)
	}
}

func TestFrameMode(t *testing.T) {
	f := Frame{7, serial.EvenParity, serial.TwoStopBits}
	m := f.Mode(31250)
	if m.BaudRate != 31250 || m.DataBits != 7 || m.Parity != serial.EvenParity || m.StopBits != serial.TwoStopBits {
		t.Errorf("Mode = %+v, want the frame at31250", m)
	}
}
