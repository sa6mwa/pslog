//go:build amd64

package asmlog

import (
	"strconv"
	"unsafe"
)

func asmJSONStringAvailable() bool {
	return true
}

func appendJSONStringAsm(dst []byte, s string) []byte {
	needed := len(s)*6 + 2
	dst = ensureCapacity(dst, needed)
	if cap(dst) == 0 {
		return appendJSONStringGo(dst, s)
	}
	buf := dst[:cap(dst)]
	start := len(dst)
	basePtr := unsafe.Add(unsafe.Pointer(&buf[0]), uintptr(start))
	srcPtr := unsafe.StringData(s)
	written := quoteJSONStringAMD64((*byte)(basePtr), srcPtr, len(s))
	return buf[:start+written]
}

func appendJSONStringGo(dst []byte, s string) []byte {
	return strconv.AppendQuote(dst, s)
}

//go:noescape
func quoteJSONStringAMD64(dst *byte, src *byte, n int) int
