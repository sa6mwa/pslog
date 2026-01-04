package istty

// IsTerminal reports whether the given file descriptor is a terminal.
func IsTerminal(fd int) bool {
	return isTerminal(fd)
}
