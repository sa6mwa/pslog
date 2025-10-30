//go:build arm64

package pslog

func appendConsoleStringInline(buf []byte, value string) []byte {
	idx := firstConsoleUnsafeIndex(value)
	buf = append(buf, value[:idx]...)
	if idx == len(value) {
		return buf
	}
	return strconvAppendQuoted(buf, value)
}
