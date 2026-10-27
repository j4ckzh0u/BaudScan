package baud

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Preview returns a one-line sample of a capture: text with its control
// characters escaped, or hex when the capture is binary.
func Preview(data []byte, max int) string {
	if len(data) == 0 {
		return ""
	}
	a := Analyze(data)
	if a.TextRatio >= 0.75 {
		return textPreview(data, max)
	}
	return hexPreview(data, max)
}

func textPreview(data []byte, max int) string {
	var b strings.Builder
	used := 0
	i := 0
	for i < len(data) && used < max {
		r, size := utf8.DecodeRune(data[i:])
		switch {
		case r == '\n':
			b.WriteString(`\n`)
			used += 2
		case r == '\r':
			b.WriteString(`\r`)
			used += 2
		case r == '\t':
			b.WriteString(`\t`)
			used += 2
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, `\x%02X`, data[i])
			used += 4
		case unicode.IsPrint(r):
			b.WriteRune(r)
			used++
		default:
			fmt.Fprintf(&b, `\x%02X`, data[i])
			used += 4
		}
		i += size
	}
	if i < len(data) {
		b.WriteString("…")
	}
	return b.String()
}

func hexPreview(data []byte, max int) string {
	if max > len(data) {
		max = len(data)
	}
	var b strings.Builder
	for i := range max {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%02X", data[i])
	}
	if max < len(data) {
		b.WriteString("…")
	}
	return b.String()
}

// HexDump formats up to max bytes with offsets and an ASCII column. Eight
// bytes per line fit the narrow detail panel without wrapping.
func HexDump(data []byte, max int) string {
	const perLine = 8
	if len(data) == 0 {
		return "没有收到数据。"
	}
	if max <= 0 || max > len(data) {
		max = len(data)
	}
	var b strings.Builder
	for off := 0; off < max; off += perLine {
		end := off + perLine
		if end > max {
			end = max
		}
		fmt.Fprintf(&b, "%08X  ", off)
		for i := 0; i < perLine; i++ {
			if off+i < end {
				fmt.Fprintf(&b, "%02X ", data[off+i])
			} else {
				b.WriteString("   ")
			}
			if i == 3 {
				b.WriteByte(' ')
			}
		}
		b.WriteString(" |")
		for i := off; i < end; i++ {
			c := data[i]
			if c >= 0x20 && c <= 0x7e {
				b.WriteByte(c)
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteString("|\n")
	}
	if max < len(data) {
		fmt.Fprintf(&b, "… 共 %s，只显示前 %d 字节", FormatBytes(len(data)), max)
	}
	return strings.TrimRight(b.String(), "\n")
}

// FormatBytes writes a byte count for people.
func FormatBytes(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

// KindLabel names a capture kind in the interface.
func KindLabel(k Kind) string {
	switch k {
	case KindText:
		return "文本"
	case KindBinary:
		return "二进制结构"
	case KindConstant:
		return "恒定电平"
	case KindNoise:
		return "噪声"
	case KindNoData:
		return "无数据"
	default:
		return "—"
	}
}

// BaudLabel writes a baud rate with no separator, as serial tools do.
func BaudLabel(rate int) string {
	return fmt.Sprintf("%d", rate)
}
