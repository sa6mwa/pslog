package pslog

import (
	"io"
	"sync"
)

type pslogOwnedCloser interface {
	pslogOwnedClose() error
}

type ownedOutput struct {
	writer   io.Writer
	closer   io.Closer
	closeErr error
	once     sync.Once
}

func newOwnedOutput(writer io.Writer, closer io.Closer) io.Writer {
	if writer == nil {
		writer = io.Discard
	}
	if closer == nil {
		return writer
	}
	if existing, ok := writer.(*ownedOutput); ok {
		return existing
	}
	return &ownedOutput{writer: writer, closer: closer}
}

func (o *ownedOutput) Write(p []byte) (int, error) {
	return o.writer.Write(p)
}

func (o *ownedOutput) Close() error {
	return o.pslogOwnedClose()
}

func (o *ownedOutput) pslogOwnedClose() error {
	o.once.Do(func() {
		if o.closer != nil {
			o.closeErr = o.closer.Close()
		}
	})
	return o.closeErr
}
