package httpserver

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// SecurityConfig 安全中间件配置
type SecurityConfig struct {
	// CORS
	AllowedOrigins []string // 允许的 Origin 列表，空表示允许所有
	AllowedMethods []string // 允许的 HTTP 方法
	AllowedHeaders []string // 允许的请求头

	// 请求大小限制（字节），0 表示使用默认值 10MB
	MaxRequestSize int64

	// IP 限流（令牌桶）
	RateLimitRPS   float64 // 每秒允许的请求数，0 表示不限流
	RateLimitBurst int     // 桶容量（突发请求数）
}

// DefaultSecurityConfig 返回默认安全配置
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		AllowedOrigins: []string{},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization", "X-Request-Id", "X-Admin-Key"},
		MaxRequestSize: 10 * 1024 * 1024, // 10MB
		RateLimitRPS:   0,                 // 默认不限流
		RateLimitBurst: 0,
	}
}

// corsMiddleware CORS 中间件
func corsMiddleware(cfg SecurityConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if len(cfg.AllowedOrigins) == 0 {
					// 未配置时允许所有来源
					w.Header().Set("Access-Control-Allow-Origin", origin)
				} else {
					for _, allowed := range cfg.AllowedOrigins {
						if allowed == origin {
							w.Header().Set("Access-Control-Allow-Origin", origin)
							break
						}
					}
				}
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			// 预检请求直接返回
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// securityHeadersMiddleware 安全头中间件
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

// requestSizeLimitMiddleware 请求大小限制中间件
func requestSizeLimitMiddleware(maxSize int64) func(http.Handler) http.Handler {
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024 // 默认 10MB
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxSize)
			// 使用 maxBytesResponseWriter 捕获因 body 过大导致的写入错误
			mw := &maxBytesResponseWriter{ResponseWriter: w}
			next.ServeHTTP(mw, r)
			if mw.truncated {
				// 如果 handler 尚未写入状态码，返回 413
				writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
					"error": map[string]any{
						"message": "request body too large",
						"type":    "request_too_large",
					},
				})
			}
		})
	}
}

// maxBytesResponseWriter 检测 http.MaxBytesReader 触发的截断写入
type maxBytesResponseWriter struct {
	http.ResponseWriter
	truncated bool
	written   bool
}

func (mw *maxBytesResponseWriter) WriteHeader(code int) {
	if !mw.written {
		mw.written = true
		mw.ResponseWriter.WriteHeader(code)
	}
}

func (mw *maxBytesResponseWriter) Write(b []byte) (int, error) {
	n, err := mw.ResponseWriter.Write(b)
	if err != nil {
		// http.MaxBytesReader 会在 body 超限时导致写入失败
		mw.truncated = true
	}
	return n, err
}

// Hijack 支持 WebSocket 升级
func (mw *maxBytesResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := mw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("hijacking not supported")
}

// Flush 支持 SSE 等流式响应
func (mw *maxBytesResponseWriter) Flush() {
	if flusher, ok := mw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Push 支持 HTTP/2 Server Push
func (mw *maxBytesResponseWriter) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := mw.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

// IPRateLimiter 基于令牌桶的 IP 限流器
type IPRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*tokenBucket
	rps      float64
	burst    int
}

type tokenBucket struct {
	tokens    float64
	lastCheck time.Time
}

// NewIPRateLimiter 创建 IP 限流器
func NewIPRateLimiter(rps float64, burst int) *IPRateLimiter {
	return &IPRateLimiter{
		visitors: make(map[string]*tokenBucket),
		rps:      rps,
		burst:    burst,
	}
}

// getBucket 获取或创建令牌桶
func (l *IPRateLimiter) getBucket(ip string) *tokenBucket {
	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, exists := l.visitors[ip]
	if !exists {
		bucket = &tokenBucket{
			tokens:    float64(l.burst),
			lastCheck: time.Now(),
		}
		l.visitors[ip] = bucket
	}
	return bucket
}

// allow 检查是否允许请求
func (l *IPRateLimiter) allow(ip string) bool {
	bucket := l.getBucket(ip)

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(bucket.lastCheck).Seconds()
	bucket.lastCheck = now

	// 补充令牌
	bucket.tokens += elapsed * l.rps
	if bucket.tokens > float64(l.burst) {
		bucket.tokens = float64(l.burst)
	}

	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}
	return false
}

// ipRateLimitMiddleware IP 限流中间件
func ipRateLimitMiddleware(limiter *IPRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}

			if !limiter.allow(ip) {
				w.Header().Set("Retry-After", "1")
				writeJSON(w, http.StatusTooManyRequests, map[string]any{
					"error": map[string]any{
						"message": "too many requests",
						"type":    "rate_limit_error",
					},
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// applySecurityMiddlewares 将所有安全中间件应用到 handler
// 执行顺序（从外到内）：CORS → 安全头 → 请求大小限制 → IP 限流
func applySecurityMiddlewares(handler http.Handler, cfg SecurityConfig) http.Handler {
	h := handler

	// IP 限流（最内层，最先检查）
	if cfg.RateLimitRPS > 0 {
		burst := cfg.RateLimitBurst
		if burst <= 0 {
		burst = int(cfg.RateLimitRPS)
		}
		limiter := NewIPRateLimiter(cfg.RateLimitRPS, burst)
		h = ipRateLimitMiddleware(limiter)(h)
	}

	// 请求大小限制
	h = requestSizeLimitMiddleware(cfg.MaxRequestSize)(h)

	// 安全头
	h = securityHeadersMiddleware(h)

	// CORS（最外层）
	h = corsMiddleware(cfg)(h)

	return h
}
