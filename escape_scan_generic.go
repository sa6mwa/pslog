//go:build !amd64 && !arm64

package pslog

func firstUnsafeIndex(s string) int {
	return firstUnsafeIndexSmall(s)
}
