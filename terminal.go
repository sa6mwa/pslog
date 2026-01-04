package pslog

import (
	"io"

	"pkt.systems/pslog/internal/istty"
)

type fdWriter interface {
	Fd() uintptr
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(fdWriter)
	if !ok {
		return false
	}
	return istty.IsTerminal(int(f.Fd()))
}
