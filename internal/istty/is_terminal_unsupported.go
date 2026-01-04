//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows && !plan9 && !zos

package istty

func isTerminal(int) bool { return false }
