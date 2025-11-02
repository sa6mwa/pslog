//go:build amd64

package tsc

// Read returns the current value of the time-stamp counter.
func Read() uint64

// Available reports whether rdtsc-based cycle counting is supported.
func Available() bool { return true }
