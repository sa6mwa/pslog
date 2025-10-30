//go:build amd64

package pslog

func firstUnsafeIndex(s string) int {
	if len(s) == 0 {
		return 0
	}
	return firstUnsafeIndexAsm(s)
}

//go:noescape
func firstUnsafeIndexAsm(s string) int
