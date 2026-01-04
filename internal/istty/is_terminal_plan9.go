//go:build plan9

package istty

import "syscall"

func isTerminal(fd int) bool {
	path, err := syscall.Fd2path(fd)
	if err != nil {
		return false
	}
	return path == "/dev/cons" || path == "/mnt/term/dev/cons"
}
