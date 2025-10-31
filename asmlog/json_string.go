package asmlog

var jsonStringAsmEnabled = asmJSONStringAvailable()

func appendJSONString(dst []byte, s string) []byte {
	if jsonStringAsmEnabled {
		return appendJSONStringAsm(dst, s)
	}
	return appendJSONStringGo(dst, s)
}

func ensureCapacity(dst []byte, extra int) []byte {
	if extra <= 0 {
		return dst
	}
	need := len(dst) + extra
	if need <= cap(dst) {
		return dst
	}
	newCap := cap(dst)*2 + extra
	if newCap < need {
		newCap = need
	}
	newBuf := make([]byte, len(dst), newCap)
	copy(newBuf, dst)
	return newBuf
}

func setJSONStringASM(enabled bool) func() {
	prev := jsonStringAsmEnabled
	jsonStringAsmEnabled = enabled
	return func() { jsonStringAsmEnabled = prev }
}
