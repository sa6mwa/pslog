package pslog

import (
	"encoding/json"
	"math"
	"strconv"
	"time"
	"unicode/utf8"

	"pkt.systems/pslog/ansi"
)

func writeJSONNumber(w *lineWriter, n int64, _ bool) {
	w.writeInt64(n)
}

func writeJSONNumberColored(w *lineWriter, n int64, color string) {
	if color == "" {
		writeJSONNumber(w, n, false)
		return
	}
	w.reserve(len(color) + 24 + len(ansi.Reset))
	w.buf = append(w.buf, color...)
	w.buf = strconv.AppendInt(w.buf, n, 10)
	w.buf = append(w.buf, ansi.Reset...)
	w.maybeFlush()
}

func writeJSONUint(w *lineWriter, n uint64, _ bool) {
	w.writeUint64(n)
}

func writeJSONUintColored(w *lineWriter, n uint64, color string) {
	if color == "" {
		writeJSONUint(w, n, false)
		return
	}
	w.reserve(len(color) + 24 + len(ansi.Reset))
	w.buf = append(w.buf, color...)
	w.buf = strconv.AppendUint(w.buf, n, 10)
	w.buf = append(w.buf, ansi.Reset...)
	w.maybeFlush()
}

func writeJSONFloat(w *lineWriter, f float64, _ bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		writeJSONStringPlain(w, "NaN")
		return
	}
	w.writeFloat64(f)
}

func writeJSONFloatColored(w *lineWriter, f float64, color string) {
	if color == "" {
		writeJSONFloat(w, f, false)
		return
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		writePTJSONStringColored(w, color, "NaN")
		return
	}
	w.reserve(len(color) + 32 + len(ansi.Reset))
	w.buf = append(w.buf, color...)
	w.buf = strconv.AppendFloat(w.buf, f, 'f', -1, 64)
	w.buf = append(w.buf, ansi.Reset...)
	w.maybeFlush()
}

func writeJSONBoolColored(w *lineWriter, v bool, color string) {
	if color == "" {
		w.writeBoolLiteral(v)
		return
	}
	literal := "false"
	if v {
		literal = "true"
	}
	w.reserve(len(color) + len(literal) + len(ansi.Reset))
	w.buf = append(w.buf, color...)
	w.buf = append(w.buf, literal...)
	w.buf = append(w.buf, ansi.Reset...)
	w.maybeFlush()
}

func writeJSONRaw(w *lineWriter, raw []byte, _ bool) {
	w.writeBytes(raw)
}

func writeJSONStringPlain(w *lineWriter, s string) {
	writeJSONStringTo(w, s)
}

func writeJSONValuePlain(w *lineWriter, value any) {
	switch v := value.(type) {
	case string:
		if promoteTrustedValueString(v) {
			writePTJSONStringTrusted(w, v)
		} else {
			writeJSONStringPlain(w, v)
		}
	case TrustedString:
		writePTJSONStringTrusted(w, string(v))
	case stringer:
		writeJSONStringPlain(w, v.String())
	case error:
		writeJSONStringPlain(w, v.Error())
	case bool:
		w.writeBoolLiteral(v)
	case json.Marshaler:
		bytes, err := v.MarshalJSON()
		if err != nil {
			writeJSONStringPlain(w, err.Error())
			return
		}
		w.writeBytes(bytes)
	case nil:
		w.writeNullLiteral()
	default:
		switch vv := v.(type) {
		case time.Time:
			writeJSONStringPlain(w, w.formatTimeRFC3339(vv))
		case time.Duration:
			writeJSONStringPlain(w, w.formatDuration(vv))
		case int:
			writeJSONNumber(w, int64(vv), false)
		case int8:
			writeJSONNumber(w, int64(vv), false)
		case int16:
			writeJSONNumber(w, int64(vv), false)
		case int32:
			writeJSONNumber(w, int64(vv), false)
		case int64:
			writeJSONNumber(w, vv, false)
		case uint:
			writeJSONUint(w, uint64(vv), false)
		case uint8:
			writeJSONUint(w, uint64(vv), false)
		case uint16:
			writeJSONUint(w, uint64(vv), false)
		case uint32:
			writeJSONUint(w, uint64(vv), false)
		case uint64:
			writeJSONUint(w, vv, false)
		case uintptr:
			writeJSONUint(w, uint64(vv), false)
		case float32:
			writeJSONFloat(w, float64(vv), false)
		case float64:
			writeJSONFloat(w, vv, false)
		case json.Number:
			writeJSONRaw(w, []byte(vv.String()), false)
		case []byte:
			writeJSONStringPlain(w, string(vv))
		default:
			b, err := json.Marshal(v)
			if err != nil {
				writeJSONStringPlain(w, err.Error())
				return
			}
			w.writeBytes(b)
		}
	}
}

func writeJSONValueColored(w *lineWriter, value any, color string) {
	if color == "" {
		writeJSONValuePlain(w, value)
		return
	}
	switch v := value.(type) {
	case string:
		if promoteTrustedValueString(v) {
			writePTJSONStringTrustedColored(w, color, v)
		} else {
			writePTJSONStringColored(w, color, v)
		}
	case TrustedString:
		writePTJSONStringTrustedColored(w, color, string(v))
	case stringer:
		writePTJSONStringColored(w, color, v.String())
	case error:
		writePTJSONStringColored(w, color, v.Error())
	case bool:
		writeJSONBoolColored(w, v, color)
	case json.Marshaler:
		bytes, err := v.MarshalJSON()
		if err != nil {
			writePTJSONStringColored(w, color, err.Error())
			return
		}
		w.reserve(len(color) + len(bytes) + len(ansi.Reset))
		w.buf = append(w.buf, color...)
		w.buf = append(w.buf, bytes...)
		w.buf = append(w.buf, ansi.Reset...)
		w.maybeFlush()
	case nil:
		w.reserve(len(color) + len("null") + len(ansi.Reset))
		w.buf = append(w.buf, color...)
		w.buf = append(w.buf, 'n', 'u', 'l', 'l')
		w.buf = append(w.buf, ansi.Reset...)
		w.maybeFlush()
	default:
		switch vv := v.(type) {
		case time.Time:
			writePTJSONStringColored(w, color, w.formatTimeRFC3339(vv))
		case time.Duration:
			writePTJSONStringColored(w, color, w.formatDuration(vv))
		case int:
			writeJSONNumberColored(w, int64(vv), color)
		case int8:
			writeJSONNumberColored(w, int64(vv), color)
		case int16:
			writeJSONNumberColored(w, int64(vv), color)
		case int32:
			writeJSONNumberColored(w, int64(vv), color)
		case int64:
			writeJSONNumberColored(w, vv, color)
		case uint:
			writeJSONUintColored(w, uint64(vv), color)
		case uint8:
			writeJSONUintColored(w, uint64(vv), color)
		case uint16:
			writeJSONUintColored(w, uint64(vv), color)
		case uint32:
			writeJSONUintColored(w, uint64(vv), color)
		case uint64:
			writeJSONUintColored(w, vv, color)
		case uintptr:
			writeJSONUintColored(w, uint64(vv), color)
		case float32:
			writeJSONFloatColored(w, float64(vv), color)
		case float64:
			writeJSONFloatColored(w, vv, color)
		case json.Number:
			w.reserve(len(color) + len(vv.String()) + len(ansi.Reset))
			w.buf = append(w.buf, color...)
			w.buf = append(w.buf, vv.String()...)
			w.buf = append(w.buf, ansi.Reset...)
			w.maybeFlush()
		case []byte:
			writePTJSONStringColored(w, color, string(vv))
		default:
			b, err := json.Marshal(v)
			if err != nil {
				writePTJSONStringColored(w, color, err.Error())
				return
			}
			w.reserve(len(color) + len(b) + len(ansi.Reset))
			w.buf = append(w.buf, color...)
			w.buf = append(w.buf, b...)
			w.buf = append(w.buf, ansi.Reset...)
			w.maybeFlush()
		}
	}
}

func writeJSONStringTo(w *lineWriter, s string) {
	w.writeByte('"')
	start := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r >= 0x20 && r != '\\' && r != '"' {
			i += size
			continue
		}
		if start < i {
			w.writeString(s[start:i])
		}
		switch r {
		case '"', '\\':
			w.writeByte('\\')
			w.writeByte(byte(r))
		case '\b':
			w.writeString(`\b`)
		case '\f':
			w.writeString(`\f`)
		case '\n':
			w.writeString(`\n`)
		case '\r':
			w.writeString(`\r`)
		case '\t':
			w.writeString(`\t`)
		default:
			w.writeString(`\u`)
			const hex = "0123456789abcdef"
			w.writeByte('0')
			w.writeByte('0')
			w.writeByte(hex[(r>>4)&0x0f])
			w.writeByte(hex[r&0x0f])
		}
		i += size
		start = i
	}
	if start < len(s) {
		w.writeString(s[start:])
	}
	w.writeByte('"')
}
