// Package baud detects the baud rate of a serial device that streams data,
// without sending anything to the device.
package baud

import (
	"math"
	"unicode"
	"unicode/utf8"
)

// Kind is what a capture looks like.
type Kind string

const (
	KindNoData   Kind = "no-data"
	KindConstant Kind = "constant"
	KindNoise    Kind = "noise"
	KindText     Kind = "text"
	KindBinary   Kind = "binary"
)

// Analysis describes one capture and scores how much it looks like a real
// serial stream rather than the noise an incorrect baud rate produces.
type Analysis struct {
	Bytes int

	// TextRatio is the share of bytes that form printable characters,
	// including tabs and line breaks. Random bytes score near 0.4.
	TextRatio float64
	// PrintableRatio is the share of printable ASCII bytes, for display.
	PrintableRatio float64
	// ControlRatio is the share of control bytes other than tab, CR and LF.
	ControlRatio float64

	Entropy  float64
	Distinct int

	// Lines counts non-empty lines; GoodLines counts those made of text.
	Lines     int
	GoodLines int
	// LineRatio is GoodLines/Lines, 0 without lines.
	LineRatio float64

	// Repeat is the highest self-similarity of the capture at a lag
	// between 2 and 384 bytes: frames of a real protocol repeat their
	// headers, while noise does not. RepeatLag is that lag.
	Repeat    float64
	RepeatLag int

	// TextScore and BinaryScore are the two hypotheses; Score is the
	// better one, rounded to 0..100.
	TextScore   float64
	BinaryScore float64
	Score       int
	Kind        Kind
}

// Analyze measures a capture. It never uses the data's content beyond
// statistics, and is deterministic.
func Analyze(data []byte) Analysis {
	a := Analysis{Bytes: len(data)}
	if len(data) == 0 {
		a.Kind = KindNoData
		return a
	}

	var counts [256]int
	for _, b := range data {
		counts[b]++
	}
	var textBytes, printable, control int
	for b, n := range counts {
		switch {
		case b >= 0x20 && b <= 0x7e:
			printable += n
			textBytes += n
		case b == '\t' || b == '\n' || b == '\r':
			textBytes += n
		case b < 0x20 || b == 0x7f:
			control += n
		}
		if n > 0 {
			a.Distinct++
			p := float64(n) / float64(len(data))
			a.Entropy -= p * math.Log2(p)
		}
	}

	// Bytes above ASCII may be UTF-8 text, as Chinese sensor output is.
	// Count the ones that decode to printable characters; invalid and
	// control bytes are neither text nor ASCII controls.
	for i := 0; i < len(data); {
		b := data[i]
		if b < utf8.RuneSelf {
			i++
			continue
		}
		r, size := utf8.DecodeRune(data[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			// An invalid UTF-8 byte is neither text nor an ASCII
			// control; it only lowers TextRatio.
			i++
		case unicode.IsPrint(r):
			textBytes += size
			printable += size
			i += size
		default:
			control += size
			i += size
		}
	}

	n := float64(len(data))
	a.TextRatio = float64(textBytes) / n
	a.PrintableRatio = float64(printable) / n
	a.ControlRatio = float64(control) / n
	a.Lines, a.GoodLines = lineStats(data)
	if a.Lines > 0 {
		a.LineRatio = float64(a.GoodLines) / float64(a.Lines)
	}
	a.Repeat, a.RepeatLag = repeatStats(data)

	// A line-free stream is still text, just with weaker evidence.
	line := a.LineRatio
	switch a.Lines {
	case 0:
		line = 0
	case 1:
		line *= 0.45
	case 2:
		line *= 0.75
	}
	repeat := clamp01((a.Repeat - 0.01) / 0.16)
	entropy := clamp01((7.75 - a.Entropy) / 2.6)
	clean := 1 - clamp01(a.ControlRatio/0.18)
	distinct := clamp01(float64(a.Distinct-1) / 8)
	text := clamp01((a.TextRatio - 0.42) / 0.53)

	a.TextScore = 100 * (0.55*text + 0.18*line + 0.10*repeat + 0.10*entropy + 0.07*clean)
	a.TextScore *= 0.72 + 0.28*distinct
	a.TextScore = clamp(a.TextScore, 0, 100)

	// Binary protocols are not printable, so they are recognized by
	// repeating frame structure and a byte distribution that is not
	// uniform noise. A 4-byte header in a 64-byte frame already makes
	// 1/16 of the bytes repeat at the frame length, far above the ~1/256
	// of noise. Control bytes are common in binary and penalized only
	// when they dominate.
	binClean := 1 - clamp01(a.ControlRatio/0.5)
	a.BinaryScore = 100 * (0.68*repeat + 0.14*entropy + 0.12*binClean + 0.06*distinct)
	a.BinaryScore = clamp(a.BinaryScore, 0, 100)

	// Idle lines and repeated constants carry no baud information.
	if len(data) >= 6 && a.Distinct <= 2 {
		a.Kind = KindConstant
		a.Score = 0
		return a
	}

	score := math.Max(a.TextScore, a.BinaryScore)
	switch {
	case len(data) < 2:
		score = 0
	case len(data) < 4:
		score = math.Min(score, 30)
	case len(data) < 8:
		score = math.Min(score, 50)
	case len(data) < 16:
		score = math.Min(score, 70)
	}
	a.Score = int(math.Round(score))

	switch {
	case a.Score < 20 || (text < 0.25 && a.BinaryScore < 25):
		a.Kind = KindNoise
	case a.TextScore >= a.BinaryScore && text >= 0.25:
		a.Kind = KindText
	case a.BinaryScore >= 40:
		a.Kind = KindBinary
	default:
		a.Kind = KindNoise
	}
	return a
}

// lineStats counts non-empty lines and how many consist of printable text.
func lineStats(data []byte) (lines, good int) {
	start := 0
	for i := 0; i <= len(data); i++ {
		if i < len(data) && data[i] != '\n' {
			continue
		}
		line := data[start:i]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if len(line) > 0 && len(line) <= 240 {
			lines++
			if textLine(line) {
				good++
			}
		}
		start = i + 1
	}
	return lines, good
}

// textLine reports whether a line consists of printable characters and
// contains something other than spaces.
func textLine(line []byte) bool {
	nonSpace := false
	for i := 0; i < len(line); {
		r, size := utf8.DecodeRune(line[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			return false
		case r == '\t':
		case unicode.IsPrint(r):
			if !unicode.IsSpace(r) {
				nonSpace = true
			}
		default:
			return false
		}
		i += size
	}
	return nonSpace
}

// repeatStats returns the highest byte equality at a fixed lag. Real
// protocols repeat headers, line lengths, or whole frames; wrong baud
// rates produce noise with no such period.
func repeatStats(data []byte) (best float64, bestLag int) {
	if len(data) > 8192 {
		data = data[:8192]
	}
	if len(data) < 16 {
		return 0, 0
	}
	maxLag := len(data) / 3
	if maxLag > 384 {
		maxLag = 384
	}
	for lag := 2; lag <= maxLag; lag++ {
		match := 0
		for i := lag; i < len(data); i++ {
			if data[i] == data[i-lag] {
				match++
			}
		}
		ratio := float64(match) / float64(len(data)-lag)
		if ratio > best {
			best, bestLag = ratio, lag
		}
	}
	return best, bestLag
}

func clamp01(v float64) float64 { return clamp(v, 0, 1) }

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
