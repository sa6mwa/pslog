package pslog

import "strconv"

func needsQuote(s string) bool {
	return firstConsoleUnsafeIndex(s) != len(s)
}

func strconvAppendInt(buf []byte, value int64) []byte {
	return strconv.AppendInt(buf, value, 10)
}

func strconvAppendUint(buf []byte, value uint64) []byte {
	return strconv.AppendUint(buf, value, 10)
}

func strconvAppendFloat(buf []byte, value float64) []byte {
	return strconv.AppendFloat(buf, value, 'f', -1, 64)
}

func writeConsoleQuotedString(lw *lineWriter, value string) {
	lw.reserve(len(value)*4 + 2)
	lw.buf = append(lw.buf, '"')
	lw.buf = appendConsoleEscapedContentTo(lw.buf, value)
	lw.buf = append(lw.buf, '"')
	lw.maybeFlush()
}

func appendConsoleQuotedString(buf []byte, value string) []byte {
	buf = append(buf, '"')
	buf = appendConsoleEscapedContentTo(buf, value)
	buf = append(buf, '"')
	return buf
}
