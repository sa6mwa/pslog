package pslog

import (
	"encoding/json"
	"strconv"
	"time"
)

type stringer interface {
	String() string
}

func keyFromAny(v any) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case time.Time:
		return val.Format(time.RFC3339Nano), true
	case time.Duration:
		return val.String(), true
	case []byte:
		return string(val), true
	case error:
		return val.Error(), true
	case bool:
		if val {
			return "true", true
		}
		return "false", true
	case json.Number:
		return val.String(), true
	case stringer:
		return val.String(), true
	case int:
		return strconv.FormatInt(int64(val), 10), true
	case int8:
		return strconv.FormatInt(int64(val), 10), true
	case int16:
		return strconv.FormatInt(int64(val), 10), true
	case int32:
		return strconv.FormatInt(int64(val), 10), true
	case int64:
		return strconv.FormatInt(val, 10), true
	case uint:
		return strconv.FormatUint(uint64(val), 10), true
	case uint8:
		return strconv.FormatUint(uint64(val), 10), true
	case uint16:
		return strconv.FormatUint(uint64(val), 10), true
	case uint32:
		return strconv.FormatUint(uint64(val), 10), true
	case uint64:
		return strconv.FormatUint(val, 10), true
	case uintptr:
		return strconv.FormatUint(uint64(val), 10), true
	case float32:
		return strconv.FormatFloat(float64(val), 'g', -1, 32), true
	case float64:
		return strconv.FormatFloat(val, 'g', -1, 64), true
	default:
		return "", false
	}
}

func stringFromAny(v any) string {
	if key, ok := keyFromAny(v); ok {
		return key
	}
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case json.Marshaler:
		data, err := val.MarshalJSON()
		if err != nil {
			return err.Error()
		}
		return string(data)
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return err.Error()
		}
		return string(data)
	}
}

func argKeyName(pair int) string {
	return "arg" + strconv.Itoa(pair)
}
