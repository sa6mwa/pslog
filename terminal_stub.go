//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !windows

package pslog

import "io"

func isTerminal(io.Writer) bool { return false }
