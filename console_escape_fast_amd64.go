//go:build amd64

package pslog

import "golang.org/x/sys/cpu"

func firstConsoleUnsafeIndex(s string) int {
	if len(s) == 0 {
		return 0
	}
	if cpu.X86.HasAVX2 {
		return firstConsoleUnsafeIndexAVX2(s)
	}
	return firstConsoleUnsafeIndexSSE(s)
}

//go:noescape
func firstConsoleUnsafeIndexAVX2(s string) int

//go:noescape
func firstConsoleUnsafeIndexSSE(s string) int
