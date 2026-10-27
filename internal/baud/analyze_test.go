package baud

import (
	"math/rand/v2"
	"strings"
	"testing"
)

func textStream(repeat int) []byte {
	lines := []string{
		"$GNGGA,123519,4807.038,N,01131.000,E,1,08,0.9,545.4,M,46.9,M,,*47",
		"$GNGLL,4807.038,N,01131.000,E,123519.00,A,A*6E",
		"$GNRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W*6A",
	}
	var b strings.Builder
	for range repeat {
		for _, line := range lines {
			b.WriteString(line)
			b.WriteString("\r\n")
		}
	}
	return []byte(b.String())
}

func TestAnalyzeText(t *testing.T) {
	a := Analyze(textStream(20))
	if a.Kind != KindText {
		t.Fatalf("kind = %s, want text (score %d)", a.Kind, a.Score)
	}
	if a.Score < 80 {
		t.Errorf("score = %d, want at least 80 (metrics %+v)", a.Score, a)
	}
	if a.Lines != 60 || a.GoodLines != 60 {
		t.Errorf("lines = %d, good = %d, want 60/60", a.Lines, a.GoodLines)
	}
}

func TestAnalyzeUTF8Text(t *testing.T) {
	data := []byte(strings.Repeat("温度: 23.5 °C  湿度: 48%\r\n", 30))
	a := Analyze(data)
	if a.Kind != KindText || a.Score < 70 {
		t.Errorf("kind = %s, score = %d, want text >= 70: %+v", a.Kind, a.Score, a)
	}
}

func TestAnalyzeNoise(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	data := make([]byte, 8192)
	for i := range data {
		data[i] = byte(r.Uint32())
	}
	a := Analyze(data)
	if a.Kind != KindNoise {
		t.Errorf("kind = %s, want noise (score %d, text %.2f, binary %.2f)", a.Kind, a.Score, a.TextScore, a.BinaryScore)
	}
	if a.Score > 30 {
		t.Errorf("score = %d, want at most 30 for random noise", a.Score)
	}
}

func TestAnalyzeConstant(t *testing.T) {
	for _, b := range []byte{0x00, 0xff, 'A'} {
		data := []byte(strings.Repeat(string([]byte{b}), 100))
		if got := Analyze(data); got.Kind != KindConstant || got.Score != 0 {
			t.Errorf("byte %#x: kind = %s, score = %d, want constant/0", b, got.Kind, got.Score)
		}
	}
}

func TestAnalyzeBinaryFrames(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	data := make([]byte, 0, 64*80)
	for range 80 {
		payload := make([]byte, 56)
		for i := range payload {
			payload[i] = byte(r.Uint32())
		}
		frame := append([]byte{0xAA, 0x55, 0x01, 0x10}, payload...)
		frame = append(frame, 0x00, 0x0D, 0x0A, 0x00)
		data = append(data, frame...)
	}
	a := Analyze(data)
	if a.Kind != KindBinary {
		t.Fatalf("kind = %s, want binary (score %d, text %.2f, binary %.2f, repeat %.2f@%d)",
			a.Kind, a.Score, a.TextScore, a.BinaryScore, a.Repeat, a.RepeatLag)
	}
	if a.Score < 50 {
		t.Errorf("score = %d, want at least 50 for framed binary data: %+v", a.Score, a)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	a := Analyze(nil)
	if a.Kind != KindNoData || a.Score != 0 || a.Bytes != 0 {
		t.Errorf("empty capture = %+v", a)
	}
}

func TestPreviewAndHexDump(t *testing.T) {
	text := Preview([]byte("hello\r\nworld"), 100)
	if text != `hello\r\nworld` {
		t.Errorf("preview = %q", text)
	}
	binary := Preview([]byte{0xaa, 0x55, 0x01}, 100)
	if binary != "AA 55 01" {
		t.Errorf("binary preview = %q", binary)
	}
	dump := HexDump([]byte("ABC\x00\xff"), 0)
	if !strings.Contains(dump, "41 42 43 00  FF") || !strings.Contains(dump, "|ABC..|") {
		t.Errorf("dump = %q", dump)
	}
}

func TestRank(t *testing.T) {
	results := []Result{
		{Baud: 9600, Analysis: Analysis{Score: 30}},
		{Baud: 115200, Analysis: Analysis{Score: 80}},
		{Baud: 57600, Analysis: Analysis{Score: 80}},
	}
	Rank(results)
	if results[0].Baud != 115200 || results[1].Baud != 57600 || results[2].Baud != 9600 {
		t.Errorf("rank order = %v", []int{results[0].Baud, results[1].Baud, results[2].Baud})
	}
}
