//go:build arm64

package pslog

func firstUnsafeIndex(s string) int {
	if len(s) == 0 {
		return 0
	}
	return firstUnsafeIndexSmall(s)
}
