package pslog

import (
	"time"
	"unicode/utf8"
	"unsafe"

	"pkt.systems/pslog/ansi"
)

var jsonNeedsEscape = func() [256]bool {
	var table [256]bool
	for i := range 32 {
		table[i] = true
	}
	table['"'] = true
	table['\\'] = true
	table['<'] = true
	table['\''] = true
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
		writePTJSONString(lw, v)
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
		writePTJSONStringTrusted(lw, lw.formatTimeRFC3339(v))
	case time.Duration:
		writePTJSONStringTrusted(lw, lw.formatDuration(v))
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
		writePTJSONStringColored(lw, color, v)
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
		writePTJSONStringTrustedColored(lw, color, lw.formatTimeRFC3339(v))
	case time.Duration:
		writePTJSONStringTrustedColored(lw, color, lw.formatDuration(v))
	default:
		writeJSONValueColored(lw, v, color)
	}
}

func writeRuntimeValuePlainInline(lw *lineWriter, value any) bool {
	switch v := value.(type) {
	case TrustedString:
		writePTJSONStringTrusted(lw, string(v))
		return true
	case string:
		writePTJSONString(lw, v)
		return true
	case bool:
		lw.writeBool(v)
		return true
	case int:
		writeJSONNumber(lw, int64(v), false)
		return true
	case int8:
		writeJSONNumber(lw, int64(v), false)
		return true
	case int16:
		writeJSONNumber(lw, int64(v), false)
		return true
	case int32:
		writeJSONNumber(lw, int64(v), false)
		return true
	case int64:
		writeJSONNumber(lw, v, false)
		return true
	case uint:
		writeJSONUint(lw, uint64(v), false)
		return true
	case uint8:
		writeJSONUint(lw, uint64(v), false)
		return true
	case uint16:
		writeJSONUint(lw, uint64(v), false)
		return true
	case uint32:
		writeJSONUint(lw, uint64(v), false)
		return true
	case uint64:
		writeJSONUint(lw, v, false)
		return true
	case uintptr:
		writeJSONUint(lw, uint64(v), false)
		return true
	case float32:
		writeJSONFloat(lw, float64(v), false)
		return true
	case float64:
		writeJSONFloat(lw, v, false)
		return true
	case []byte:
		s := string(v)
		if stringTrustedASCII(s) {
			writePTJSONStringTrusted(lw, s)
		} else {
			writePTJSONString(lw, s)
		}
		return true
	case time.Time:
		writePTJSONStringTrusted(lw, lw.formatTimeRFC3339(v))
		return true
	case time.Duration:
		writePTJSONStringTrusted(lw, lw.formatDuration(v))
		return true
	case stringer:
		s := v.String()
		if stringTrustedASCII(s) {
			writePTJSONStringTrusted(lw, s)
		} else {
			writePTJSONString(lw, s)
		}
		return true
	case error:
		s := v.Error()
		if stringTrustedASCII(s) {
			writePTJSONStringTrusted(lw, s)
		} else {
			writePTJSONString(lw, s)
		}
		return true
	}
	return false
}

func writeRuntimeValueColorInline(lw *lineWriter, value any, color string) bool {
	if color == "" {
		return writeRuntimeValuePlainInline(lw, value)
	}
	switch v := value.(type) {
	case TrustedString:
		writePTJSONStringTrustedColored(lw, color, string(v))
		return true
	case string:
		writePTJSONStringColored(lw, color, v)
		return true
	case bool:
		writeJSONBoolColored(lw, v, color)
		return true
	case int:
		writeJSONNumberColored(lw, int64(v), color)
		return true
	case int8:
		writeJSONNumberColored(lw, int64(v), color)
		return true
	case int16:
		writeJSONNumberColored(lw, int64(v), color)
		return true
	case int32:
		writeJSONNumberColored(lw, int64(v), color)
		return true
	case int64:
		writeJSONNumberColored(lw, v, color)
		return true
	case uint:
		writeJSONUintColored(lw, uint64(v), color)
		return true
	case uint8:
		writeJSONUintColored(lw, uint64(v), color)
		return true
	case uint16:
		writeJSONUintColored(lw, uint64(v), color)
		return true
	case uint32:
		writeJSONUintColored(lw, uint64(v), color)
		return true
	case uint64:
		writeJSONUintColored(lw, v, color)
		return true
	case uintptr:
		writeJSONUintColored(lw, uint64(v), color)
		return true
	case float32:
		writeJSONFloatColored(lw, float64(v), color)
		return true
	case float64:
		writeJSONFloatColored(lw, v, color)
		return true
	case []byte:
		s := string(v)
		if stringTrustedASCII(s) {
			writePTJSONStringTrustedColored(lw, color, s)
		} else {
			writePTJSONStringColored(lw, color, s)
		}
		return true
	case time.Time:
		writePTJSONStringTrustedColored(lw, color, lw.formatTimeRFC3339(v))
		return true
	case time.Duration:
		writePTJSONStringTrustedColored(lw, color, lw.formatDuration(v))
		return true
	case stringer:
		s := v.String()
		if stringTrustedASCII(s) {
			writePTJSONStringTrustedColored(lw, color, s)
		} else {
			writePTJSONStringColored(lw, color, s)
		}
		return true
	case error:
		s := v.Error()
		if stringTrustedASCII(s) {
			writePTJSONStringTrustedColored(lw, color, s)
		} else {
			writePTJSONStringColored(lw, color, s)
		}
		return true
	}
	return false
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
			writePTJSONString(lw, vv)
			continue
		}
		writeJSONValuePlain(lw, elem)
	}
	lw.writeByte(']')
}

func writePTJSONString(lw *lineWriter, s string) {
	lw.reserve(len(s)*6 + 2)
	lw.buf = append(lw.buf, '"')
	appendEscapedStringContent(lw, s)
	lw.buf = append(lw.buf, '"')
	lw.maybeFlush()
}

func writePTJSONStringColored(lw *lineWriter, color string, s string) {
	if color == "" {
		writePTJSONString(lw, s)
		return
	}
	lw.writeString(color)
	writePTJSONString(lw, s)
	lw.writeString(ansi.Reset)
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

func writeTrustedKeyColon(lw *lineWriter, key string) {
	lw.reserve(len(key) + 3)
	lw.buf = append(lw.buf, '"')
	lw.buf = append(lw.buf, key...)
	lw.buf = append(lw.buf, '"', ':')
	lw.maybeFlush()
}

func writePTJSONStringWithColon(lw *lineWriter, s string) {
	lw.reserve(len(s)*6 + 3)
	lw.buf = append(lw.buf, '"')
	appendEscapedStringContent(lw, s)
	lw.buf = append(lw.buf, '"', ':')
	lw.maybeFlush()
}

func stringHasUnsafe(s string) bool {
	for i := 0; i < len(s); {
		c := s[i]
		if c < 0x80 {
			if c < 0x20 || c == 0x7f || c == '"' || c == '\\' {
				return true
			}
			i++
			continue
		}
		// DecodeRuneInString validates multi-byte sequences; a size of 1 with RuneError
		// signals invalid UTF-8.
		_, size := utf8.DecodeRuneInString(s[i:])
		if size == 1 {
			return true
		}
		i += size
	}
	return false
}

func runtimeKeyFromValue(v any, pair int) (string, bool) {
	switch k := v.(type) {
	case TrustedString:
		return string(k), true
	case string:
		return k, false
	default:
		key := keyFromValue(v, pair)
		if key == "" {
			return "", false
		}
		return key, promoteTrustedKey(key)
	}
}

func writePTLogValueFast(lw *lineWriter, value any) bool {
	switch v := value.(type) {
	case TrustedString:
		writePTJSONStringTrusted(lw, string(v))
		return true
	case string:
		writePTJSONString(lw, v)
		return true
	case bool:
		lw.writeBool(v)
		return true
	case int:
		writeJSONNumber(lw, int64(v), false)
		return true
	case int8:
		writeJSONNumber(lw, int64(v), false)
		return true
	case int16:
		writeJSONNumber(lw, int64(v), false)
		return true
	case int32:
		writeJSONNumber(lw, int64(v), false)
		return true
	case int64:
		writeJSONNumber(lw, v, false)
		return true
	case uint:
		writeJSONUint(lw, uint64(v), false)
		return true
	case uint8:
		writeJSONUint(lw, uint64(v), false)
		return true
	case uint16:
		writeJSONUint(lw, uint64(v), false)
		return true
	case uint32:
		writeJSONUint(lw, uint64(v), false)
		return true
	case uint64:
		writeJSONUint(lw, v, false)
		return true
	case uintptr:
		writeJSONUint(lw, uint64(v), false)
		return true
	case float32:
		writeJSONFloat(lw, float64(v), false)
		return true
	case float64:
		writeJSONFloat(lw, v, false)
		return true
	case time.Time:
		writePTJSONStringTrusted(lw, lw.formatTimeRFC3339(v))
		return true
	case time.Duration:
		writePTJSONStringTrusted(lw, lw.formatDuration(v))
		return true
	}
	return false
}

func stringTrustedASCII(s string) bool {
	n := len(s)
	if n == 0 {
		return true
	}
	ptr := unsafe.StringData(s)
	i := 0
	for i+8 <= n {
		chunk := *(*uint64)(unsafe.Add(unsafe.Pointer(ptr), uintptr(i)))
		if chunkJSONUnsafeMask(chunk) != 0 || chunk&asciiHighBitsMask != 0 {
			return false
		}
		i += 8
	}
	for ; i < n; i++ {
		c := s[i]
		if c < 0x20 || c == '"' || c == '\\' || c >= 0x80 {
			return false
		}
	}
	return true
}

func writePTLogValueColoredFast(lw *lineWriter, value any, color string) bool {
	if color == "" {
		return writePTLogValueFast(lw, value)
	}
	switch v := value.(type) {
	case TrustedString:
		writePTJSONStringTrustedColored(lw, color, string(v))
		return true
	case string:
		writePTJSONStringColored(lw, color, v)
		return true
	case bool:
		writeJSONBoolColored(lw, v, color)
		return true
	case int:
		writeJSONNumberColored(lw, int64(v), color)
		return true
	case int8:
		writeJSONNumberColored(lw, int64(v), color)
		return true
	case int16:
		writeJSONNumberColored(lw, int64(v), color)
		return true
	case int32:
		writeJSONNumberColored(lw, int64(v), color)
		return true
	case int64:
		writeJSONNumberColored(lw, v, color)
		return true
	case uint:
		writeJSONUintColored(lw, uint64(v), color)
		return true
	case uint8:
		writeJSONUintColored(lw, uint64(v), color)
		return true
	case uint16:
		writeJSONUintColored(lw, uint64(v), color)
		return true
	case uint32:
		writeJSONUintColored(lw, uint64(v), color)
		return true
	case uint64:
		writeJSONUintColored(lw, v, color)
		return true
	case uintptr:
		writeJSONUintColored(lw, uint64(v), color)
		return true
	case float32:
		writeJSONFloatColored(lw, float64(v), color)
		return true
	case float64:
		writeJSONFloatColored(lw, v, color)
		return true
	case time.Time:
		writePTJSONStringTrustedColored(lw, color, lw.formatTimeRFC3339(v))
		return true
	case time.Duration:
		writePTJSONStringTrustedColored(lw, color, lw.formatDuration(v))
		return true
	}
	return false
}

func writeRuntimeValuePlain(lw *lineWriter, value any) bool {
	switch v := value.(type) {
	case TrustedString:
		writePTJSONStringTrusted(lw, string(v))
		return true
	case string:
		writePTJSONString(lw, v)
		return true
	case bool:
		lw.writeBool(v)
		return true
	case int:
		writeJSONNumber(lw, int64(v), false)
		return true
	case int8:
		writeJSONNumber(lw, int64(v), false)
		return true
	case int16:
		writeJSONNumber(lw, int64(v), false)
		return true
	case int32:
		writeJSONNumber(lw, int64(v), false)
		return true
	case int64:
		writeJSONNumber(lw, v, false)
		return true
	case uint:
		writeJSONUint(lw, uint64(v), false)
		return true
	case uint8:
		writeJSONUint(lw, uint64(v), false)
		return true
	case uint16:
		writeJSONUint(lw, uint64(v), false)
		return true
	case uint32:
		writeJSONUint(lw, uint64(v), false)
		return true
	case uint64:
		writeJSONUint(lw, v, false)
		return true
	case uintptr:
		writeJSONUint(lw, uint64(v), false)
		return true
	case float32:
		writeJSONFloat(lw, float64(v), false)
		return true
	case float64:
		writeJSONFloat(lw, v, false)
		return true
	case time.Time:
		writePTJSONStringTrusted(lw, lw.formatTimeRFC3339(v))
		return true
	case time.Duration:
		writePTJSONStringTrusted(lw, lw.formatDuration(v))
		return true
	}
	return false
}

func writeRuntimeValueColor(lw *lineWriter, value any, color string) bool {
	if color == "" {
		return writeRuntimeValuePlain(lw, value)
	}
	switch v := value.(type) {
	case TrustedString:
		writePTJSONStringTrustedColored(lw, color, string(v))
		return true
	case string:
		writePTJSONStringColored(lw, color, v)
		return true
	case bool:
		writeJSONBoolColored(lw, v, color)
		return true
	case int:
		writeJSONNumberColored(lw, int64(v), color)
		return true
	case int8:
		writeJSONNumberColored(lw, int64(v), color)
		return true
	case int16:
		writeJSONNumberColored(lw, int64(v), color)
		return true
	case int32:
		writeJSONNumberColored(lw, int64(v), color)
		return true
	case int64:
		writeJSONNumberColored(lw, v, color)
		return true
	case uint:
		writeJSONUintColored(lw, uint64(v), color)
		return true
	case uint8:
		writeJSONUintColored(lw, uint64(v), color)
		return true
	case uint16:
		writeJSONUintColored(lw, uint64(v), color)
		return true
	case uint32:
		writeJSONUintColored(lw, uint64(v), color)
		return true
	case uint64:
		writeJSONUintColored(lw, v, color)
		return true
	case uintptr:
		writeJSONUintColored(lw, uint64(v), color)
		return true
	case float32:
		writeJSONFloatColored(lw, float64(v), color)
		return true
	case float64:
		writeJSONFloatColored(lw, v, color)
		return true
	case time.Time:
		writePTJSONStringTrustedColored(lw, color, lw.formatTimeRFC3339(v))
		return true
	case time.Duration:
		writePTJSONStringTrustedColored(lw, color, lw.formatDuration(v))
		return true
	}
	return false
}
