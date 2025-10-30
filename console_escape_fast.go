package pslog

import (
	"encoding/binary"
	"unsafe"
)

const (
	asciiHighBitsMask    = 0x8080808080808080
	repeatOnes           = 0x0101010101010101
	jsonControlThreshold = 0x2020202020202020
	jsonQuoteMask        = 0x2222222222222222
	jsonBackslashMask    = 0x5c5c5c5c5c5c5c5c
	consoleSpaceMask     = 0x2020202020202020
	consoleDelMask       = 0x7f7f7f7f7f7f7f7f
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
		if consoleChunkHasUnsafe(chunk) {
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
	if chunk&asciiHighBitsMask != 0 {
		return true
	}
	control := (chunk - jsonControlThreshold) & ^chunk & asciiHighBitsMask
	if control != 0 {
		return true
	}
	if chunkHasByte(chunk, jsonQuoteMask) || chunkHasByte(chunk, jsonBackslashMask) ||
		chunkHasByte(chunk, consoleSpaceMask) || chunkHasByte(chunk, consoleDelMask) {
		return true
	}
	return false
}

func chunkHasByte(chunk, target uint64) bool {
	x := chunk ^ target
	return (x-repeatOnes)&^x&asciiHighBitsMask != 0
}

func consoleByteUnsafe(b byte) bool {
	return b < 0x20 || b == ' ' || b == '\\' || b == '"' || b >= 0x7f
}
