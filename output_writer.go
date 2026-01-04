package pslog

import "io"

type teeWriter struct {
	writers []io.Writer
	closer  io.Closer
}

func newTeeWriter(closer io.Closer, writers ...io.Writer) io.Writer {
	return &teeWriter{writers: writers, closer: closer}
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

func (t *teeWriter) Close() error {
	if t.closer == nil {
		return nil
	}
	return t.closer.Close()
}
