//go:build zos

package istty

import (
	"syscall"
	"unsafe"
)

// SYS_TCGETATTR value from x/sys/unix for z/OS.
const sysTcgetattr = 0x1d0

type termios struct {
	Cflag uint32
	Iflag uint32
	Lflag uint32
	Oflag uint32
	Cc    [11]uint8
}

func isTerminal(fd int) bool {
	var t termios
	_, _, errno := syscall.Syscall(sysTcgetattr, uintptr(fd), uintptr(unsafe.Pointer(&t)), 0)
	return errno == 0
}
