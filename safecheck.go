package pslog

// promoteTrustedKey returns true when key contains only JSON-safe runes.
func promoteTrustedKey(key string) bool {
	if key == "" {
		return false
	}
	return !stringHasUnsafe(key)
}

// promoteTrustedValueString reports whether s can be emitted without escaping.
func promoteTrustedValueString(s string) bool {
	if s == "" {
		return true
	}
	return !stringHasUnsafe(s)
}
