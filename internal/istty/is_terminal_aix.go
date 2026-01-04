//go:build aix

package istty

import (
	"syscall"
	"unsafe"
)

// TCGETS on AIX isn't exposed in syscall; value from x/sys/unix.
const ioctlReadTermios = 0x5401

//go:linkname ioctl syscall.ioctl
func ioctl(fd, req, arg uintptr) syscall.Errno

func isTerminal(fd int) bool {
	var termios syscall.Termios
	err := ioctl(uintptr(fd), ioctlReadTermios, uintptr(unsafe.Pointer(&termios)))
	return err == 0
}
