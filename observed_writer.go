package pslog

import (
	"io"
	"sync/atomic"
)

// WriteFailure describes one failed write observed by ObservedWriter.
type WriteFailure struct {
	Err       error
	Written   int
	Attempted int
}

// ObservedWriterStats captures aggregated failure counters for ObservedWriter.
type ObservedWriterStats struct {
	Failures    uint64
	ShortWrites uint64
}

// ObservedWriter wraps an io.Writer and records write failures so log loss can
// be observed without changing logger call signatures.
type ObservedWriter struct {
	dst        io.Writer
	onFailure  func(WriteFailure)
	failures   atomic.Uint64
	shortWrite atomic.Uint64
}

// NewObservedWriter wraps dst with failure observation hooks.
func NewObservedWriter(dst io.Writer, onFailure func(WriteFailure)) *ObservedWriter {
	if dst == nil {
		dst = io.Discard
	}
	return &ObservedWriter{
		dst:       dst,
		onFailure: onFailure,
	}
}

func (w *ObservedWriter) Write(p []byte) (int, error) {
	if w == nil || w.dst == nil {
		return len(p), nil
	}

	n, err := w.dst.Write(p)
	if n != len(p) {
		w.shortWrite.Add(1)
		if err == nil {
			err = io.ErrShortWrite
		}
	}

	if err != nil {
		w.failures.Add(1)
		if w.onFailure != nil {
			w.onFailure(WriteFailure{
				Err:       err,
				Written:   n,
				Attempted: len(p),
			})
		}
	}

	return n, err
}

// Stats returns cumulative write-failure counters.
func (w *ObservedWriter) Stats() ObservedWriterStats {
	if w == nil {
		return ObservedWriterStats{}
	}
	return ObservedWriterStats{
		Failures:    w.failures.Load(),
		ShortWrites: w.shortWrite.Load(),
	}
}

// Close delegates close semantics to the wrapped destination.
func (w *ObservedWriter) Close() error {
	if w == nil {
		return nil
	}
	return closeOutput(w.dst)
}

func (w *ObservedWriter) pslogOwnedClose() error {
	if w == nil {
		return nil
	}
	return closeOutput(w.dst)
}
