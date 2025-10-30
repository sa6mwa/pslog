//go:build amd64

package pslog

import "golang.org/x/sys/cpu"

func firstUnsafeIndex(s string) int {
	if len(s) == 0 {
		return 0
	}
	if cpu.X86.HasAVX2 {
		return firstUnsafeIndexAVX2(s)
	}
	return firstUnsafeIndexSSE(s)
}

//go:noescape
func firstUnsafeIndexAVX2(s string) int

//go:noescape
func firstUnsafeIndexSSE(s string) int
