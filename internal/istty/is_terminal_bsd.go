//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package istty

import (
	"syscall"
	"unsafe"
)

const ioctlReadTermios = syscall.TIOCGETA

func isTerminal(fd int) bool {
	var termios syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(ioctlReadTermios), uintptr(unsafe.Pointer(&termios)))
	return errno == 0
}
