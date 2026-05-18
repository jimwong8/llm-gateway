package router

import (
	"bytes"
	"sync"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func getBuffer() *bytes.Buffer {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func putBuffer(buf *bytes.Buffer) {
	if buf.Cap() > 64*1024 {
		return
	}
	bufferPool.Put(buf)
}

var byteSlicePool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 4096)
		return &b
	},
}

func getByteSlice() *[]byte {
	return byteSlicePool.Get().(*[]byte)
}

func putByteSlice(b *[]byte) {
	if cap(*b) > 256*1024 {
		return
	}
	*b = (*b)[:0]
	byteSlicePool.Put(b)
}
