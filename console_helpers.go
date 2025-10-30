package pslog

import (
	"strconv"
)

func needsQuote(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c > 0x7e || c == ' ' || c == '\\' || c == '"' {
			return true
		}
	}
	return false
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
