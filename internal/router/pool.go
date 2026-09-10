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

// GetBuffer 从池中获取一个已重置的 bytes.Buffer（导出供跨包使用）
func GetBuffer() *bytes.Buffer { return getBuffer() }

// PutBuffer 将 bytes.Buffer 归还到池中（导出供跨包使用）
func PutBuffer(buf *bytes.Buffer) { putBuffer(buf) }

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

// GetByteSlice 从池中获取一个已重置的 []byte（导出供跨包使用）
func GetByteSlice() *[]byte { return getByteSlice() }

// PutByteSlice 将 []byte 归还到池中（导出供跨包使用）
func PutByteSlice(b *[]byte) { putByteSlice(b) }
