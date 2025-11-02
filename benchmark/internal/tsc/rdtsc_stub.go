//go:build !amd64

package tsc

func Read() uint64 { return 0 }

func Available() bool { return false }
