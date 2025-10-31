//go:build !amd64

package asmlog

func asmJSONStringAvailable() bool {
	return false
}

func appendJSONStringAsm(dst []byte, s string) []byte {
	return appendJSONStringGo(dst, s)
}
