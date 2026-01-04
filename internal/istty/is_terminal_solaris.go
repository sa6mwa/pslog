//go:build solaris

package istty

import (
	"syscall"
	"unsafe"
)

// TCGETS on Solaris isn't exposed in syscall; value from x/sys/unix.
const ioctlReadTermios = 0x540d

//go:linkname ioctl syscall.ioctl
func ioctl(fd, req, arg uintptr) syscall.Errno

func isTerminal(fd int) bool {
	var termios syscall.Termios
	err := ioctl(uintptr(fd), ioctlReadTermios, uintptr(unsafe.Pointer(&termios)))
	return err == 0
}
