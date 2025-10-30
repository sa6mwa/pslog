//go:build arm64

package pslog

func firstConsoleUnsafeIndex(s string) int {
	if len(s) == 0 {
		return 0
	}
	return firstConsoleUnsafeIndexAsm(s)
}

//go:noescape
func firstConsoleUnsafeIndexAsm(s string) int
