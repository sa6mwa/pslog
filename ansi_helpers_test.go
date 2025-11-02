package pslog

import "strings"

func stripANSIString(s string) string {
	var b strings.Builder
	sz := len(s)
	for i := 0; i < sz; i++ {
		if s[i] != '\x1b' {
			b.WriteByte(s[i])
			continue
		}
		if i+1 >= sz || s[i+1] != '[' {
			b.WriteByte(s[i])
			continue
		}
		j := i + 2
		for j < sz && s[j] != 'm' {
			j++
		}
		if j >= sz {
			break
		}
		i = j
	}
	return b.String()
}
