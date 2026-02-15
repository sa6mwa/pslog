package pslog

import "sync/atomic"

const (
	lineHintMaxPrealloc   = lineWriterFlushTrigger
	lineHintDecayShift    = 3
	lineHintDecayMinDelta = 64
)

// updateLineHint keeps preallocation hints bounded while gradually adapting
// down after transient long lines.
func updateLineHint(hint *atomic.Int64, lineLen int) {
	if hint == nil || lineLen <= 0 {
		return
	}
	if lineLen > lineHintMaxPrealloc {
		lineLen = lineHintMaxPrealloc
	}
	next := int64(lineLen)
	current := hint.Load()
	if current <= 0 {
		hint.Store(next)
		return
	}
	if next >= current {
		if next != current {
			hint.Store(next)
		}
		return
	}
	// Ignore normal line-length jitter; only decay after substantial drops.
	if next*2 > current {
		return
	}
	delta := current - next
	if delta < lineHintDecayMinDelta {
		return
	}
	decayed := current - (delta >> lineHintDecayShift)
	if decayed <= next {
		decayed = next
	} else if decayed == current {
		decayed = current - 1
	}
	if decayed != current {
		hint.Store(decayed)
	}
}
