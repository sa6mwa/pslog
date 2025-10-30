//go:build !arm64

package pslog

import (
	"encoding/binary"
	"unsafe"
)

func firstConsoleUnsafeIndex(s string) int {
	n := len(s)
	if n == 0 {
		return 0
	}
	bytes := unsafe.Slice(unsafe.StringData(s), n)
	i := 0
	for i+8 <= n {
		chunk := binary.LittleEndian.Uint64(bytes[i:])
		if chunkHasConsoleUnsafe(chunk) {
			for j := 0; j < 8; j++ {
				if consoleByteUnsafe(bytes[i+j]) {
					return i + j
				}
			}
		}
		i += 8
	}
	for ; i < n; i++ {
		if consoleByteUnsafe(bytes[i]) {
			return i
		}
	}
	return n
}

func consoleChunkHasUnsafe(chunk uint64) bool {
	return chunkHasConsoleUnsafe(chunk)
}

func consoleByteUnsafe(b byte) bool {
	return b < 0x20 || b == ' ' || b == '\\' || b == '"' || b >= 0x7f
}
