package router

import (
	"context"
	"io"
	"sync"
	"time"
)

// BackpressureWriter 带背压控制的 io.Writer。
// 内部使用有缓冲 channel 暂存写入数据，当缓冲区满时根据 timeout 决定是阻塞等待还是返回超时错误。
// 适用于 SSE 流式响应等生产快于消费的场景。
type BackpressureWriter struct {
	w       io.Writer
	buf     chan []byte
	mu      sync.Mutex
	closed  bool
	timeout time.Duration
}

// NewBackpressureWriter 创建一个 BackpressureWriter。
// bufferSize 为 channel 缓冲区大小，timeout 为写入超时时间。
func NewBackpressureWriter(w io.Writer, bufferSize int, timeout time.Duration) *BackpressureWriter {
	if bufferSize <= 0 {
		bufferSize = 64
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &BackpressureWriter{
		w:       w,
		buf:     make(chan []byte, bufferSize),
		timeout: timeout,
	}
}

// Write 将数据写入背压缓冲区。若缓冲区满，等待 timeout 后返回超时错误。
func (bp *BackpressureWriter) Write(p []byte) (int, error) {
	bp.mu.Lock()
	if bp.closed {
		bp.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	bp.mu.Unlock()

	// 复制数据，避免调用方复用底层数组
	data := make([]byte, len(p))
	copy(data, p)

	ctx, cancel := context.WithTimeout(context.Background(), bp.timeout)
	defer cancel()

	select {
	case bp.buf <- data:
		return len(p), nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// Flush 将缓冲区中的所有数据刷新到底层 writer。
// 应在 SSE 的 Flush() 调用之后或同时调用。
func (bp *BackpressureWriter) Flush() error {
	for {
		select {
		case data := <-bp.buf:
			if _, err := bp.w.Write(data); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

// Close 关闭背压 writer，刷新剩余数据并标记为已关闭。
func (bp *BackpressureWriter) Close() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if bp.closed {
		return nil
	}
	bp.closed = true

	// 刷新剩余数据
	for {
		select {
		case data := <-bp.buf:
			if _, err := bp.w.Write(data); err != nil {
				return err
			}
		default:
			close(bp.buf)
			return nil
		}
	}
}

// RateLimiter 基于令牌桶的限流器。
// 支持按 QPS 限制请求速率，可选突发容量。
type RateLimiter struct {
	mu       sync.Mutex
	tokens   float64
	max      float64
	rate     float64
	lastTime time.Duration
	interval time.Duration
}

// NewRateLimiter 创建一个令牌桶限流器。
// qps 为每秒允许的请求数，burst 为突发容量（默认为 qps）。
func NewRateLimiter(qps float64, burst int) *RateLimiter {
	if qps <= 0 {
		qps = 10
	}
	if burst <= 0 {
		burst = int(qps)
	}
	return &RateLimiter{
		tokens:   float64(burst),
		max:      float64(burst),
		rate:     qps,
		lastTime: 0,
		interval: time.Second / time.Duration(qps),
	}
}

// Allow 检查是否允许一个请求通过。非阻塞，立即返回。
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Duration(time.Now().UnixNano())
	elapsed := now - rl.lastTime
	if elapsed > 0 {
		rl.tokens += float64(elapsed) / float64(time.Second) * rl.rate
		if rl.tokens > rl.max {
			rl.tokens = rl.max
		}
		rl.lastTime = now
	}

	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}
	return false
}

// Wait 阻塞直到获得一个令牌或 context 取消。
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		if rl.Allow() {
			return nil
		}
		timer := time.NewTimer(rl.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			continue
		}
	}
}
