//go:build !amd64

package pslog

func appendEscapedStringContent(lw *lineWriter, s string) {
	const hex = "0123456789abcdef"
	for len(s) > 0 {
		idx := firstUnsafeIndex(s)
		lw.buf = append(lw.buf, s[:idx]...)
		if idx == len(s) {
			break
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
