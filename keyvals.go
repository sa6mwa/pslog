package pslog

// Keyvals prepares a key/value slice for reuse with Log/Info/Debug/etc.
// It mirrors the normal variadic API but pre-promotes trusted string keys
// so runtime emission can skip repeat scans.
func Keyvals(keyvals ...any) []any {
	if len(keyvals) == 0 {
		return nil
	}
	pairs := len(keyvals)
	dst := make([]any, pairs)
	copy(dst, keyvals)
	pair := 0
	for i := 0; i+1 < pairs; i += 2 {
		switch k := dst[i].(type) {
		case TrustedString:
			// already trusted
		case string:
			if stringTrustedASCII(k) {
				dst[i] = TrustedString(k)
			}
		default:
			key := keyFromValue(dst[i], pair)
			if stringTrustedASCII(key) {
				dst[i] = TrustedString(key)
			} else {
				dst[i] = key
			}
		}
		pair++
	}
	return dst
}
