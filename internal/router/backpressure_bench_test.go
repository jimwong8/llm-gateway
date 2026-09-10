package router

import (
	"bytes"
	"io"
	"testing"
	"time"
)

// noopWriter 是一个丢弃写入数据的 io.Writer，用于隔离 BackpressureWriter 自身的开销。
type noopWriter struct{}

func (n *noopWriter) Write(p []byte) (int, error) { return len(p), nil }

// flushConsumer 在后台定期 Flush BackpressureWriter，防止 channel 满导致超时。
func flushConsumer(bp *BackpressureWriter, done <-chan struct{}) {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			bp.Close()
			return
		case <-ticker.C:
			bp.Flush()
		}
	}
}

// ============================================================
// BackpressureWriter.Write vs 直接 Write
// ============================================================

func BenchmarkBackpressureWriterWrite(b *testing.B) {
	bp := NewBackpressureWriter(&noopWriter{}, 1024, 5*time.Second)
	done := make(chan struct{})
	go flushConsumer(bp, done)

	data := []byte("benchmark backpressure writer data payload")
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := bp.Write(data)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	close(done)
}

func BenchmarkDirectWrite(b *testing.B) {
	w := &noopWriter{}
	data := []byte("benchmark backpressure writer data payload")
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := w.Write(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBackpressureWriterWriteSmall(b *testing.B) {
	bp := NewBackpressureWriter(&noopWriter{}, 1024, 5*time.Second)
	done := make(chan struct{})
	go flushConsumer(bp, done)

	data := []byte("small")
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := bp.Write(data)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	close(done)
}

func BenchmarkDirectWriteSmall(b *testing.B) {
	w := &noopWriter{}
	data := []byte("small")
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := w.Write(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBackpressureWriterWriteLarge(b *testing.B) {
	bp := NewBackpressureWriter(&noopWriter{}, 1024, 5*time.Second)
	done := make(chan struct{})
	go flushConsumer(bp, done)

	data := make([]byte, 32*1024) // 32KB
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := bp.Write(data)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	close(done)
}

func BenchmarkDirectWriteLarge(b *testing.B) {
	w := &noopWriter{}
	data := make([]byte, 32*1024) // 32KB
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := w.Write(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ============================================================
// BackpressureWriter + Flush vs bytes.Buffer（模拟无背压场景）
// ============================================================

func BenchmarkBackpressureWriterWriteAndFlush(b *testing.B) {
	var buf bytes.Buffer
	bp := NewBackpressureWriter(&buf, 64, 5*time.Second)

	data := []byte("benchmark write and flush data")
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := bp.Write(data)
		if err != nil {
			b.Fatal(err)
		}
		if err := bp.Flush(); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	bp.Close()
}

func BenchmarkBytesBufferWrite(b *testing.B) {
	var buf bytes.Buffer
	data := []byte("benchmark write and flush data")
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf.Write(data)
	}
}

// ============================================================
// RateLimiter.Allow vs 无锁（空操作对比基线）
// ============================================================

func BenchmarkRateLimiterAllow(b *testing.B) {
	rl := NewRateLimiter(10000, 10000)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rl.Allow()
	}
}

func BenchmarkRateLimiterAllowUnlimited(b *testing.B) {
	// 极高的 QPS 确保始终有令牌，测量纯锁开销
	rl := NewRateLimiter(1e9, 1e6)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rl.Allow()
	}
}

// noOpLimiter 用于测量"无锁"基线 — 纯函数调用开销
type noOpLimiter struct{}

func (n *noOpLimiter) Allow() bool { return true }

func BenchmarkNoOpLimiterAllow(b *testing.B) {
	var rl noOpLimiter
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rl.Allow()
	}
}

// ============================================================
// 并发场景
// ============================================================

func BenchmarkRateLimiterAllowParallel(b *testing.B) {
	rl := NewRateLimiter(1e6, 1e6)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rl.Allow()
		}
	})
}

func BenchmarkNoOpLimiterAllowParallel(b *testing.B) {
	var rl noOpLimiter
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rl.Allow()
		}
	})
}

func BenchmarkBackpressureWriterWriteParallel(b *testing.B) {
	bp := NewBackpressureWriter(&noopWriter{}, 4096, 5*time.Second)
	done := make(chan struct{})
	go flushConsumer(bp, done)

	data := []byte("parallel backpressure write data")
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := bp.Write(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.StopTimer()
	close(done)
}

func BenchmarkDirectWriteParallel(b *testing.B) {
	w := &noopWriter{}
	data := []byte("parallel backpressure write data")
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := w.Write(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// ============================================================
// io.Discard 作为底层 writer 的对比（更贴近真实场景）
// ============================================================

func BenchmarkBackpressureWriterWriteToDiscard(b *testing.B) {
	bp := NewBackpressureWriter(io.Discard, 64, 5*time.Second)
	defer bp.Close()

	data := []byte("benchmark with io.Discard writer")
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		bp.Write(data)
		bp.Flush()
	}
}

func BenchmarkDirectWriteToDiscard(b *testing.B) {
	data := []byte("benchmark with io.Discard writer")
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		io.Discard.Write(data)
	}
}
