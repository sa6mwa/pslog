//go:build arm64

package pslog

func appendConsoleStringInline(buf []byte, value string) []byte {
	if needsQuote(value) {
		return appendConsoleQuotedString(buf, value)
	}
	return append(buf, value...)
}
