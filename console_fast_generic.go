//go:build !amd64 && !arm64

package pslog

func appendConsoleStringInline(buf []byte, value string) []byte {
	if needsQuote(value) {
		return strconvAppendQuoted(buf, value)
	}
	return append(buf, value...)
}
