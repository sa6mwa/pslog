package pslog

func firstUnsafeIndexSmall(s string) int {
	for i := 0; i < len(s); i++ {
		if jsonNeedsEscape[s[i]] {
			return i
		}
	}
	return len(s)
}
