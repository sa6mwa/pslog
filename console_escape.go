package pslog

import (
	"math/bits"
	"unsafe"
)

var consoleNeedsEscape = func() [256]bool {
	var table [256]bool
	for i := range 0x20 {
		table[i] = true
	}
	table[0x7f] = true
	table['"'] = true
	table['\\'] = true
	return table
}()

func appendConsoleEscapedContentTo(dst []byte, s string) []byte {
	const hex = "0123456789abcdef"
	n := len(s)
	if n == 0 {
		return dst
	}
	ptr := unsafe.StringData(s)
	lastSafe := 0
	scan := 0
	for scan < n {
		remaining := n - scan
		var mask uint64
		base := scan
		if remaining >= 16 {
			chunk := *(*uint64)(unsafe.Add(unsafe.Pointer(ptr), uintptr(scan)))
			mask = chunkConsoleEscapeMask(chunk)
			if mask == 0 {
				chunk = *(*uint64)(unsafe.Add(unsafe.Pointer(ptr), uintptr(scan+8)))
				mask = chunkConsoleEscapeMask(chunk)
				if mask == 0 {
					scan += 16
					continue
				}
				base = scan + 8
			}
		} else if remaining >= 8 {
			chunk := *(*uint64)(unsafe.Add(unsafe.Pointer(ptr), uintptr(scan)))
			mask = chunkConsoleEscapeMask(chunk)
			if mask == 0 {
				scan += 8
				continue
			}
		} else {
			break
		}

		if lastSafe < base {
			dst = append(dst, s[lastSafe:base]...)
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
				dst = append(dst, s[cursor:pos]...)
			}
			dst = appendConsoleEscapedChar(dst, s[pos], hex)
			cursor = pos + 1
			maskCopy &^= uint64(0x80) << (offset * 8)
		}

		lastSafe = cursor
		scan = max(base+8, cursor)
	}

	for ; scan < n; scan++ {
		c := s[scan]
		if consoleNeedsEscape[c] {
			if lastSafe < scan {
				dst = append(dst, s[lastSafe:scan]...)
			}
			dst = appendConsoleEscapedChar(dst, c, hex)
			lastSafe = scan + 1
		}
	}

	if lastSafe < n {
		dst = append(dst, s[lastSafe:]...)
	}
	return dst
}

func appendConsoleEscapedChar(dst []byte, c byte, hex string) []byte {
	switch c {
	case '\\', '"':
		return append(dst, '\\', c)
	case '\b':
		return append(dst, '\\', 'b')
	case '\f':
		return append(dst, '\\', 'f')
	case '\n':
		return append(dst, '\\', 'n')
	case '\r':
		return append(dst, '\\', 'r')
	case '\t':
		return append(dst, '\\', 't')
	default:
		return append(dst, '\\', 'x', hex[c>>4], hex[c&0x0f])
	}
}
