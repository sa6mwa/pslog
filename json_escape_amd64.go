//go:build amd64

package pslog

import (
	"unsafe"

	"golang.org/x/sys/cpu"
)

func appendEscapedStringContent(lw *lineWriter, s string) {
	if len(s) == 0 {
		return
	}
	if !cpu.X86.HasAVX2 {
		appendEscapedStringContentFallback(lw, s)
		return
	}

	firstUnsafe := firstUnsafeIndex(s)
	if firstUnsafe == len(s) {
		lw.buf = append(lw.buf, s...)
		return
	}

	remaining := s[firstUnsafe:]
	// Worst-case each remaining byte expands to a \u00XX escape.
	maxGrow := len(s) + len(remaining)*5
	lw.reserve(maxGrow)

	buf := lw.buf[:cap(lw.buf)]
	base := len(lw.buf)
	if firstUnsafe > 0 {
		copy(buf[base:], s[:firstUnsafe])
		base += firstUnsafe
	}

	dst := (*byte)(unsafe.Add(unsafe.Pointer(&buf[0]), uintptr(base)))
	written := escapeJSONStringAMD64(dst, unsafe.StringData(remaining), len(remaining))
	lw.buf = buf[:base+written]
}

// appendEscapedStringContentFallback retains the previous scalar implementation
// so we can run correctly on CPUs without AVX2.
func appendEscapedStringContentFallback(lw *lineWriter, s string) {
	const hex = "0123456789abcdef"
	for len(s) > 0 {
		idx := firstUnsafeIndex(s)
		lw.buf = append(lw.buf, s[:idx]...)
		if idx == len(s) {
			return
		}
		switch c := s[idx]; c {
		case '\\', '"':
			lw.buf = append(lw.buf, '\\', c)
		case '\b':
			lw.buf = append(lw.buf, '\\', 'b')
		case '\f':
			lw.buf = append(lw.buf, '\\', 'f')
		case '\n':
			lw.buf = append(lw.buf, '\\', 'n')
		case '\r':
			lw.buf = append(lw.buf, '\\', 'r')
		case '\t':
			lw.buf = append(lw.buf, '\\', 't')
		default:
			lw.buf = append(lw.buf, '\\', 'u', '0', '0', hex[c>>4], hex[c&0x0f])
		}
		s = s[idx+1:]
	}
}

//go:noescape
func escapeJSONStringAMD64(dst *byte, src *byte, n int) int
