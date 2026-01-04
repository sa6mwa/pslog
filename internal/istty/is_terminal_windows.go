//go:build windows

package istty

import "syscall"

func isTerminal(fd int) bool {
	var st uint32
	return syscall.GetConsoleMode(syscall.Handle(fd), &st) == nil
}
