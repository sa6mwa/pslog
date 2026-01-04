package pslog

import (
	"io"
	"os"
)

func closeOutput(w io.Writer) error {
	if w == nil || w == os.Stdout || w == os.Stderr {
		return nil
	}
	if c, ok := w.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}
