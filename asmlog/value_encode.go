package asmlog

import (
	"fmt"
	"time"
)

func appendValue(buf *lineBuffer, value any) {
	switch v := value.(type) {
	case string:
		buf.appendQuoted(v)
	case bool:
		buf.appendBool(v)
	case int:
		buf.appendInt(int64(v))
	case int8:
		buf.appendInt(int64(v))
	case int16:
		buf.appendInt(int64(v))
	case int32:
		buf.appendInt(int64(v))
	case int64:
		buf.appendInt(v)
	case uint:
		buf.appendUint(uint64(v))
	case uint8:
		buf.appendUint(uint64(v))
	case uint16:
		buf.appendUint(uint64(v))
	case uint32:
		buf.appendUint(uint64(v))
	case uint64:
		buf.appendUint(v)
	case float32:
		buf.appendFloat(float64(v), 32)
	case float64:
		buf.appendFloat(v, 64)
	case time.Duration:
		buf.appendQuoted(v.String())
	case time.Time:
		buf.appendQuoted(v.UTC().Format(time.RFC3339Nano))
	case error:
		buf.appendQuoted(v.Error())
	case fmt.Stringer:
		buf.appendQuoted(v.String())
	case []byte:
		buf.appendQuoted(string(v))
	default:
		buf.appendQuoted(fmt.Sprint(v))
	}
}
