package pslog

import (
	"io"
	"os"
	"sync"
	"unsafe"
)

var cacheOwners sync.Map

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

func closeLoggerRuntime(writer io.Writer, cache *timeCache, owner uintptr) error {
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
