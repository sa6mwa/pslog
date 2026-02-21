package pslog

import (
	"context"
	"io"
	"os"
	"sync"
	"unsafe"
)

var cacheOwners sync.Map
var contextCancelOwners sync.Map

func ownerToken[T any](logger *T) uintptr {
	if logger == nil {
		return 0
	}
	return uintptr(unsafe.Pointer(logger))
}

func claimTimeCacheOwnership(cache *timeCache, owner uintptr) {
	if cache == nil || owner == 0 {
		return
	}
	cacheOwners.LoadOrStore(cache, owner)
}

func claimContextCancellation(ctx context.Context, writer io.Writer, cache *timeCache, owner uintptr) {
	if owner == 0 || ctx == nil || ctx.Done() == nil {
		return
	}
	if cache == nil && !writerNeedsOwnedClose(writer) {
		return
	}
	stop := context.AfterFunc(ctx, func() {
		_ = closeLoggerRuntime(writer, cache, owner)
	})
	contextCancelOwners.Store(owner, stop)
}

func closeLoggerRuntime(writer io.Writer, cache *timeCache, owner uintptr) error {
	if stop, ok := contextCancelOwners.LoadAndDelete(owner); ok {
		if stopFn, ok := stop.(func() bool); ok && stopFn != nil {
			stopFn()
		}
	}
	if cache != nil {
		if claimed, ok := cacheOwners.Load(cache); ok && claimed == owner {
			cache.Close()
			cacheOwners.Delete(cache)
		}
	}
	return closeOutput(writer)
}

func closeOutput(w io.Writer) error {
	if w == nil || w == os.Stdout || w == os.Stderr {
		return nil
	}
	if c, ok := w.(pslogOwnedCloser); ok {
		return c.pslogOwnedClose()
	}
	return nil
}

func writerNeedsOwnedClose(w io.Writer) bool {
	if w == nil || w == os.Stdout || w == os.Stderr {
		return false
	}
	_, ok := w.(pslogOwnedCloser)
	return ok
}
