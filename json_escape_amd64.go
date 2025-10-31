//go:build amd64

package pslog

import "unsafe"

func appendEscapedStringContent(lw *lineWriter, s string) {
	if len(s) == 0 {
		return
	}
	maxGrow := len(s) * 6
	lw.reserve(maxGrow)
	base := len(lw.buf)
	buf := lw.buf[:cap(lw.buf)]
	dst := (*byte)(unsafe.Add(unsafe.Pointer(&buf[0]), uintptr(base)))
	written := escapeJSONStringAMD64(dst, unsafe.StringData(s), len(s))
	lw.buf = buf[:base+written]
}

//go:noescape
func escapeJSONStringAMD64(dst *byte, src *byte, n int) int
