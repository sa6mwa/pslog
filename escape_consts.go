package pslog

const (
	asciiHighBitsMask    uint64 = 0x8080808080808080
	repeatOnes           uint64 = 0x0101010101010101
	jsonControlThreshold uint64 = 0x2020202020202020
	jsonQuoteMask        uint64 = 0x2222222222222222
	jsonBackslashMask    uint64 = 0x5c5c5c5c5c5c5c5c
	consoleSpaceMask     uint64 = 0x2020202020202020
	consoleDelMask       uint64 = 0x7f7f7f7f7f7f7f7f
)

func chunkHasByte(chunk, target uint64) bool {
	x := chunk ^ target
	return (x-repeatOnes)&^x&asciiHighBitsMask != 0
}

func chunkHasJSONUnsafe(chunk uint64) bool {
	if chunk&asciiHighBitsMask != 0 {
		return true
	}
	control := (chunk - jsonControlThreshold) & ^chunk & asciiHighBitsMask
	if control != 0 {
		return true
	}
	if chunkHasByte(chunk, jsonQuoteMask) || chunkHasByte(chunk, jsonBackslashMask) {
		return true
	}
	return false
}

func chunkHasConsoleUnsafe(chunk uint64) bool {
	if chunkHasJSONUnsafe(chunk) {
		return true
	}
	if chunkHasByte(chunk, consoleSpaceMask) || chunkHasByte(chunk, consoleDelMask) {
		return true
	}
	return false
}
