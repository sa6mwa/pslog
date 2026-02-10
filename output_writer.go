package pslog

import "io"

type teeWriter struct {
	writers []io.Writer
}

func newTeeWriter(writers ...io.Writer) io.Writer {
	return &teeWriter{writers: writers}
}

func (t *teeWriter) Write(p []byte) (int, error) {
	for _, w := range t.writers {
		n, err := w.Write(p)
		if err != nil {
			return n, err
		}
		if n != len(p) {
			return n, io.ErrShortWrite
		}
	}
	return len(p), nil
}
