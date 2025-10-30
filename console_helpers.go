package pslog

import (
	"strconv"
)

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

func strconvAppendQuoted(buf []byte, s string) []byte {
	return strconv.AppendQuote(buf, s)
}
