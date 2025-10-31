package asmlog

import "strconv"

type lineBuffer struct {
	buf []byte
}

const defaultLineBufferCap = 512

func newLineBuffer() *lineBuffer {
	return &lineBuffer{buf: make([]byte, 0, defaultLineBufferCap)}
}

func (b *lineBuffer) reset() {
	if cap(b.buf) > 4<<10 {
		b.buf = make([]byte, 0, defaultLineBufferCap)
		return
	}
	b.buf = b.buf[:0]
}

func (b *lineBuffer) writeByte(by byte) {
	b.buf = append(b.buf, by)
}

func (b *lineBuffer) writeString(s string) {
	b.buf = append(b.buf, s...)
}

func (b *lineBuffer) appendQuoted(s string) {
	b.buf = appendJSONString(b.buf, s)
}

func (b *lineBuffer) appendBool(v bool) {
	b.buf = strconv.AppendBool(b.buf, v)
}

func (b *lineBuffer) appendInt(v int64) {
	b.buf = strconv.AppendInt(b.buf, v, 10)
}

func (b *lineBuffer) appendUint(v uint64) {
	b.buf = strconv.AppendUint(b.buf, v, 10)
}

func (b *lineBuffer) appendFloat(f float64, bits int) {
	b.buf = strconv.AppendFloat(b.buf, f, 'f', -1, bits)
}

func (b *lineBuffer) bytes() []byte {
	return b.buf
}
