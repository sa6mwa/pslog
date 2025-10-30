package pslog

import (
	"time"
	"unicode/utf8"

	"pkt.systems/pslog/ansi"
)

var jsonNeedsEscape = func() [256]bool {
	var table [256]bool
	for i := range 32 {
		table[i] = true
	}
	table['"'] = true
	table['\\'] = true
	return table
}()

// TrustedString marks a string that is known to be JSON safe (no control
// characters, quotes, or backslashes).
type TrustedString string

// NewTrustedString returns a TrustedString when s contains no characters that
// require JSON escaping. The second return value reports whether the string was
// trusted.
func NewTrustedString(s string) (TrustedString, bool) {
	if stringHasUnsafe(s) {
		return "", false
	}
	return TrustedString(s), true
}

func makeKeyData(key string, leadingComma bool) []byte {
	buf := make([]byte, 0, len(key)*2+4)
	if leadingComma {
		buf = append(buf, ',')
	}
	if promoteTrustedKey(key) {
		buf = append(buf, '"')
		buf = append(buf, key...)
		buf = append(buf, '"', ':')
		return buf
	}
	buf = appendEscapedKey(buf, key)
	return buf
}

func writePTLogValue(lw *lineWriter, value any) {
	switch v := value.(type) {
	case []any:
		writePTLogArray(lw, v)
	case TrustedString:
		writePTJSONStringTrusted(lw, string(v))
	case string:
		if promoteTrustedValueString(v) {
			writePTJSONStringTrusted(lw, v)
		} else {
			writePTJSONString(lw, v)
		}
	case bool:
		lw.writeBool(v)
	case int:
		writeJSONNumber(lw, int64(v), false)
	case int8:
		writeJSONNumber(lw, int64(v), false)
	case int16:
		writeJSONNumber(lw, int64(v), false)
	case int32:
		writeJSONNumber(lw, int64(v), false)
	case int64:
		writeJSONNumber(lw, v, false)
	case uint:
		writeJSONUint(lw, uint64(v), false)
	case uint8:
		writeJSONUint(lw, uint64(v), false)
	case uint16:
		writeJSONUint(lw, uint64(v), false)
	case uint32:
		writeJSONUint(lw, uint64(v), false)
	case uint64:
		writeJSONUint(lw, v, false)
	case uintptr:
		writeJSONUint(lw, uint64(v), false)
	case float32:
		writeJSONFloat(lw, float64(v), false)
	case float64:
		writeJSONFloat(lw, v, false)
	case time.Time:
		writePTJSONString(lw, lw.formatTimeRFC3339(v))
	case time.Duration:
		writePTJSONString(lw, lw.formatDuration(v))
	default:
		writeJSONValuePlain(lw, v)
	}
}

func writePTLogValueColored(lw *lineWriter, value any, color string) {
	if color == "" {
		writePTLogValue(lw, value)
		return
	}
	switch v := value.(type) {
	case []any:
		lw.reserve(len(color))
		lw.buf = append(lw.buf, color...)
		writePTLogArray(lw, v)
		lw.buf = append(lw.buf, ansi.Reset...)
		lw.maybeFlush()
	case TrustedString:
		writePTJSONStringTrustedColored(lw, color, string(v))
	case string:
		if promoteTrustedValueString(v) {
			writePTJSONStringTrustedColored(lw, color, v)
		} else {
			writePTJSONStringColored(lw, color, v)
		}
	case bool:
		writeJSONBoolColored(lw, v, color)
	case int:
		writeJSONNumberColored(lw, int64(v), color)
	case int8:
		writeJSONNumberColored(lw, int64(v), color)
	case int16:
		writeJSONNumberColored(lw, int64(v), color)
	case int32:
		writeJSONNumberColored(lw, int64(v), color)
	case int64:
		writeJSONNumberColored(lw, v, color)
	case uint:
		writeJSONUintColored(lw, uint64(v), color)
	case uint8:
		writeJSONUintColored(lw, uint64(v), color)
	case uint16:
		writeJSONUintColored(lw, uint64(v), color)
	case uint32:
		writeJSONUintColored(lw, uint64(v), color)
	case uint64:
		writeJSONUintColored(lw, v, color)
	case uintptr:
		writeJSONUintColored(lw, uint64(v), color)
	case float32:
		writeJSONFloatColored(lw, float64(v), color)
	case float64:
		writeJSONFloatColored(lw, v, color)
	case time.Time:
		writePTJSONStringColored(lw, color, lw.formatTimeRFC3339(v))
	case time.Duration:
		writePTJSONStringColored(lw, color, lw.formatDuration(v))
	default:
		writeJSONValueColored(lw, v, color)
	}
}

func writePTLogArray(lw *lineWriter, values []any) {
	lw.writeByte('[')
	for i, elem := range values {
		if i > 0 {
			lw.writeByte(',')
		}
		if nested, ok := elem.([]any); ok {
			writePTLogArray(lw, nested)
			continue
		}
		switch vv := elem.(type) {
		case TrustedString:
			writePTJSONStringTrusted(lw, string(vv))
			continue
		case string:
			if promoteTrustedValueString(vv) {
				writePTJSONStringTrusted(lw, vv)
			} else {
				writePTJSONString(lw, vv)
			}
			continue
		}
		writeJSONValuePlain(lw, elem)
	}
	lw.writeByte(']')
}

func writePTJSONString(lw *lineWriter, s string) {
	lw.reserve(len(s)*2 + 3)
	lw.buf = append(lw.buf, '"')
	appendEscapedStringContent(lw, s)
	lw.buf = append(lw.buf, '"')
	lw.maybeFlush()
}

func appendEscapedStringContent(lw *lineWriter, s string) {
	const hex = "0123456789abcdef"
	start := 0
	for i := 0; i < len(s); i++ {
		if !jsonNeedsEscape[s[i]] {
			continue
		}
		if start < i {
			lw.buf = append(lw.buf, s[start:i]...)
		}
		switch c := s[i]; c {
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
		start = i + 1
	}
	if start < len(s) {
		lw.buf = append(lw.buf, s[start:]...)
	}
}

func writePTJSONStringColored(lw *lineWriter, color string, s string) {
	if color == "" {
		writePTJSONString(lw, s)
		return
	}
	reset := ansi.Reset
	lw.reserve(len(color) + len(s)*2 + len(reset) + 3)
	lw.buf = append(lw.buf, color...)
	lw.buf = append(lw.buf, '"')
	appendEscapedStringContent(lw, s)
	lw.buf = append(lw.buf, '"')
	lw.buf = append(lw.buf, reset...)
	lw.maybeFlush()
}

func writePTJSONStringTrusted(lw *lineWriter, s string) {
	lw.reserve(len(s) + 2)
	lw.buf = append(lw.buf, '"')
	lw.buf = append(lw.buf, s...)
	lw.buf = append(lw.buf, '"')
	lw.maybeFlush()
}

func writePTJSONStringTrustedColored(lw *lineWriter, color string, s string) {
	if color == "" {
		writePTJSONStringTrusted(lw, s)
		return
	}
	reset := ansi.Reset
	lw.reserve(len(color) + len(reset) + len(s) + 2)
	lw.buf = append(lw.buf, color...)
	lw.buf = append(lw.buf, '"')
	lw.buf = append(lw.buf, s...)
	lw.buf = append(lw.buf, '"')
	lw.buf = append(lw.buf, reset...)
	lw.maybeFlush()
}

func writePTJSONStringMaybeTrusted(lw *lineWriter, s string, trusted bool) {
	if trusted {
		writePTJSONStringTrusted(lw, s)
		return
	}
	writePTJSONString(lw, s)
}

func writePTFieldPrefix(lw *lineWriter, first *bool, key string, trusted bool) {
	if *first {
		*first = false
	} else {
		lw.buf = append(lw.buf, ',')
	}
	if trusted {
		lw.reserve(len(key) + 4)
		lw.buf = append(lw.buf, '"')
		lw.buf = append(lw.buf, key...)
		lw.buf = append(lw.buf, '"', ':')
		lw.maybeFlush()
		return
	}
	writePTJSONStringWithColon(lw, key)
}

func writePTJSONStringWithColon(lw *lineWriter, s string) {
	lw.reserve(len(s)*2 + 4)
	lw.buf = append(lw.buf, '"')
	appendEscapedStringContent(lw, s)
	lw.buf = append(lw.buf, '"', ':')
	lw.maybeFlush()
}

func extractKeyFast(v any) (string, bool) {
	switch k := v.(type) {
	case TrustedString:
		return string(k), true
	case string:
		return k, false
	default:
		return "", false
	}
}

func stringHasUnsafe(s string) bool {
	if !utf8.ValidString(s) {
		return true
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c == '"' || c == '\\' {
			return true
		}
	}
	return false
}
