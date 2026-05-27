package router

import (
	"bytes"
	"testing"
)

// ============================================================
// GetBuffer / PutBuffer vs new(bytes.Buffer)
// ============================================================

func BenchmarkGetBufferPutBuffer(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := GetBuffer()
		buf.WriteString("hello world")
		PutBuffer(buf)
	}
}

func BenchmarkNewBytesBuffer(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := new(bytes.Buffer)
		buf.WriteString("hello world")
		// 让 GC 回收，模拟无池化场景
		_ = buf
	}
}

func BenchmarkGetBufferPutBufferWithWrite(b *testing.B) {
	data := []byte("benchmark test data for buffer pool")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := GetBuffer()
		buf.Write(data)
		PutBuffer(buf)
	}
}

func BenchmarkNewBytesBufferWithWrite(b *testing.B) {
	data := []byte("benchmark test data for buffer pool")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := new(bytes.Buffer)
		buf.Write(data)
		_ = buf
	}
}

// ============================================================
// GetByteSlice / PutByteSlice vs make([]byte)
// ============================================================

func BenchmarkGetByteSlicePutByteSlice(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s := GetByteSlice()
		*s = append(*s, "hello world"...)
		PutByteSlice(s)
	}
}

func BenchmarkMakeByteSlice(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s := make([]byte, 0, 4096)
		s = append(s, "hello world"...)
		_ = s
	}
}

func BenchmarkGetByteSlicePutByteSliceLarge(b *testing.B) {
	data := make([]byte, 8192)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := GetByteSlice()
		*s = append(*s, data...)
		PutByteSlice(s)
	}
}

func BenchmarkMakeByteSliceLarge(b *testing.B) {
	data := make([]byte, 8192)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := make([]byte, 0, 4096)
		s = append(s, data...)
		_ = s
	}
}

// ============================================================
// 并发场景：pool vs 无池化
// ============================================================

func BenchmarkGetBufferPutBufferParallel(b *testing.B) {
	data := []byte("concurrent benchmark data")
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := GetBuffer()
			buf.Write(data)
			PutBuffer(buf)
		}
	})
}

func BenchmarkNewBytesBufferParallel(b *testing.B) {
	data := []byte("concurrent benchmark data")
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := new(bytes.Buffer)
			buf.Write(data)
			_ = buf
		}
	})
}

func BenchmarkGetByteSlicePutByteSliceParallel(b *testing.B) {
	data := make([]byte, 1024)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s := GetByteSlice()
			*s = append(*s, data...)
			PutByteSlice(s)
		}
	})
}

func BenchmarkMakeByteSliceParallel(b *testing.B) {
	data := make([]byte, 1024)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s := make([]byte, 0, 4096)
			s = append(s, data...)
			_ = s
		}
	})
}
