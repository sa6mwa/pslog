//go:build arm64

package pslog

func firstUnsafeIndex(s string) int {
	if len(s) < 32 {
		return firstUnsafeIndexSmall(s)
	}
	return firstUnsafeIndexAsm(s)
}

//go:noescape
func firstUnsafeIndexAsm(s string) int
