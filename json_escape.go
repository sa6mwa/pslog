package pslog

import (
	"math/bits"
	"unsafe"
)

func appendEscapedStringContent(lw *lineWriter, s string) {
	const hex = "0123456789abcdef"
	n := len(s)
	if n == 0 {
		return
	}
	ptr := unsafe.StringData(s)
	lastSafe := 0
	scan := 0
	for scan < n {
		remaining := n - scan
		var mask uint64
		var base int
		if remaining >= 16 {
			chunk := *(*uint64)(unsafe.Add(unsafe.Pointer(ptr), uintptr(scan)))
			mask = chunkJSONUnsafeMask(chunk)
			base = scan
			if mask == 0 {
				chunk = *(*uint64)(unsafe.Add(unsafe.Pointer(ptr), uintptr(scan+8)))
				mask = chunkJSONUnsafeMask(chunk)
				if mask == 0 {
					scan += 16
					continue
				}
				base = scan + 8
			}
		} else if remaining >= 8 {
			base = scan
			chunk := *(*uint64)(unsafe.Add(unsafe.Pointer(ptr), uintptr(base)))
			mask = chunkJSONUnsafeMask(chunk)
			if mask == 0 {
				scan += 8
				continue
			}
		} else {
			break
		}

		if lastSafe < base {
			lw.buf = append(lw.buf, s[lastSafe:base]...)
		}

		maskCopy := mask
		cursor := base
		for maskCopy != 0 {
			offset := bits.TrailingZeros64(maskCopy) >> 3
			pos := base + offset
			if pos >= n {
				break
			}
			if cursor < pos {
				lw.buf = append(lw.buf, s[cursor:pos]...)
			}
			appendEscapedChar(lw, s[pos], hex)
			cursor = pos + 1
			maskCopy &^= uint64(0x80) << (offset * 8)
		}

		lastSafe = cursor
		scan = max(base+8, cursor)
	}

	for ; scan < n; scan++ {
		c := s[scan]
		if jsonNeedsEscape[c] {
			if lastSafe < scan {
				lw.buf = append(lw.buf, s[lastSafe:scan]...)
			}
			appendEscapedChar(lw, c, hex)
			lastSafe = scan + 1
		}
	}

	if lastSafe < n {
		lw.buf = append(lw.buf, s[lastSafe:]...)
	}
}

func appendEscapedChar(lw *lineWriter, c byte, hex string) {
	switch c {
	case '\\', '"':
		lw.reserve(2)
		lw.buf = append(lw.buf, '\\', c)
	case '\b':
		lw.reserve(2)
		lw.buf = append(lw.buf, '\\', 'b')
	case '\f':
		lw.reserve(2)
		lw.buf = append(lw.buf, '\\', 'f')
	case '\n':
		lw.reserve(2)
		lw.buf = append(lw.buf, '\\', 'n')
	case '\r':
		lw.reserve(2)
		lw.buf = append(lw.buf, '\\', 'r')
	case '\t':
		lw.reserve(2)
		lw.buf = append(lw.buf, '\\', 't')
	case '<':
		lw.reserve(6)
		lw.buf = append(lw.buf, '\\', 'u', '0', '0', '3', 'c')
	case '\'':
		lw.reserve(6)
		lw.buf = append(lw.buf, '\\', 'u', '0', '0', '2', '7')
	default:
		lw.reserve(6)
		lw.buf = append(lw.buf, '\\', 'u', '0', '0', hex[c>>4], hex[c&0x0f])
	}
}
