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

func chunkEqualMask(chunk, target uint64) uint64 {
	x := chunk ^ target
	return (x - repeatOnes) & ^x & asciiHighBitsMask
}

func chunkJSONUnsafeMask(chunk uint64) uint64 {
	mask := (chunk - jsonControlThreshold) & ^chunk & asciiHighBitsMask
	mask |= chunkEqualMask(chunk, jsonQuoteMask)
	mask |= chunkEqualMask(chunk, jsonBackslashMask)
	return mask
}

func chunkHasJSONUnsafe(chunk uint64) bool {
	return chunkJSONUnsafeMask(chunk) != 0
}

func chunkConsoleEscapeMask(chunk uint64) uint64 {
	mask := (chunk - jsonControlThreshold) & ^chunk & asciiHighBitsMask
	mask |= chunkEqualMask(chunk, jsonQuoteMask)
	mask |= chunkEqualMask(chunk, jsonBackslashMask)
	mask |= chunk & asciiHighBitsMask
	mask |= chunkEqualMask(chunk, consoleDelMask)
	return mask
}

func chunkHasConsoleUnsafe(chunk uint64) bool {
	if chunk&asciiHighBitsMask != 0 {
		return true
	}
	if chunkHasJSONUnsafe(chunk) {
		return true
	}
	if chunkEqualMask(chunk, consoleSpaceMask) != 0 || chunkEqualMask(chunk, consoleDelMask) != 0 {
		return true
	}
	return false
}
