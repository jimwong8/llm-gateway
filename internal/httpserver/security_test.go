package httpserver

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// ========== CORS 中间件测试 ==========

func TestCORS_AllOriginsAllowed_WhenEmpty(t *testing.T) {
	cfg := SecurityConfig{
		AllowedOrigins: []string{},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
	}
	handler := corsMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Fatalf("expected origin https://example.com, got %q", got)
	}
}

func TestCORS_SpecificOriginAllowed(t *testing.T) {
	cfg := SecurityConfig{
		AllowedOrigins: []string{"https://allowed.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
	}
	handler := corsMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://allowed.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://allowed.com" {
		t.Fatalf("expected origin https://allowed.com, got %q", got)
	}
}

func TestCORS_OriginNotAllowed(t *testing.T) {
	cfg := SecurityConfig{
		AllowedOrigins: []string{"https://allowed.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
	}
	handler := corsMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected empty origin, got %q", got)
	}
}

func TestCORS_PreflightRequest(t *testing.T) {
	cfg := SecurityConfig{
		AllowedOrigins: []string{},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}
	handler := corsMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for preflight")
	}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Fatalf("expected methods to contain POST, got %q", got)
	}
}

func TestCORS_NoOriginHeader(t *testing.T) {
	cfg := SecurityConfig{
		AllowedOrigins: []string{"https://allowed.com"},
		AllowedMethods: []string{"GET"},
		AllowedHeaders: []string{"Content-Type"},
	}
	handler := corsMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	// 不设置 Origin
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCORS_AllowCredentialsHeader(t *testing.T) {
	cfg := SecurityConfig{AllowedOrigins: []string{}}
	handler := corsMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected credentials true, got %q", got)
	}
}

// ========== 安全头中间件测试 ==========

func TestSecurityHeaders_AllPresent(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	tests := map[string]string{
		"X-Frame-Options":           "DENY",
		"X-Content-Type-Options":    "nosniff",
		"X-XSS-Protection":          "1; mode=block",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Content-Security-Policy":   "default-src 'self'",
	}

	for header, expected := range tests {
		got := rr.Header().Get(header)
		if !strings.Contains(got, expected) {
			t.Errorf("header %q: expected to contain %q, got %q", header, expected, got)
		}
	}
}

func TestSecurityHeaders_CSPFrameAncestors(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	csp := rr.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP should contain frame-ancestors 'none', got %q", csp)
	}
}

// ========== 请求大小限制中间件测试 =========

func TestRequestSizeLimit_WithinLimit(t *testing.T) {
	handler := requestSizeLimitMiddleware(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))

	body := bytes.Repeat([]byte("a"), 512)
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestRequestSizeLimit_ExceedsLimit(t *testing.T) {
	handler := requestSizeLimitMiddleware(100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			// handler 检测到 body 读取错误，返回 413
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
				"error": map[string]any{"message": "request body too large", "type": "request_too_large"},
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	body := bytes.Repeat([]byte("a"), 200)
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rr.Code)
	}
}

func TestRequestSizeLimit_DefaultSize(t *testing.T) {
	// maxSize=0 应使用默认 10MB
	handler := requestSizeLimitMiddleware(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 1MB 请求应该通过（小于 10MB 默认值）
	body := bytes.Repeat([]byte("a"), 1024*1024)
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for 1MB request with default limit, got %d", rr.Code)
	}
}

// ========== IP 限流中间件测试 ==========

func TestIPRateLimit_AllowsWithinBurst(t *testing.T) {
	limiter := NewIPRateLimiter(10, 5)
	handler := ipRateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 5 个突发请求应该全部通过
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rr.Code)
		}
	}
}

func TestIPRateLimit_BlocksAfterBurst(t *testing.T) {
	limiter := NewIPRateLimiter(1, 2) // 1 rps, burst=2
	handler := ipRateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 2 个突发请求通过
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.2:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rr.Code)
		}
	}

	// 第 3 个请求应该被限流
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.2:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rr.Code)
	}
	if got := rr.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("expected Retry-After: 1, got %q", got)
	}
}

func TestIPRateLimit_RefillsOverTime(t *testing.T) {
	limiter := NewIPRateLimiter(100, 1) // 100 rps, burst=1
	handler := ipRateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 消耗令牌
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.3:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rr.Code)
	}

	// 立即请求应该被限流
	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.3:12345"
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429, got %d", rr.Code)
	}

	// 等待令牌补充
	time.Sleep(20 * time.Millisecond) // 100 rps → 约 10ms 补充 1 个令牌

	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.3:12345"
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("third request after refill: expected 200, got %d", rr.Code)
	}
}

func TestIPRateLimit_DifferentIPsIndependent(t *testing.T) {
	limiter := NewIPRateLimiter(1, 1)
	handler := ipRateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// IP1 消耗令牌
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("IP1 first: expected 200, got %d", rr.Code)
	}

	// IP2 应该不受影响
	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.2:12345"
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("IP2 first: expected 200, got %d", rr.Code)
	}
}

func TestIPRateLimit_ConcurrentAccess(t *testing.T) {
	limiter := NewIPRateLimiter(1000, 100)
	handler := ipRateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	var wg sync.WaitGroup
	var mu sync.Mutex
	var blocked int

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "10.0.0.5:12345"
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code == http.StatusTooManyRequests {
				mu.Lock()
				blocked++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// 100 个令牌桶，200 个并发请求，应该有一些被限流
	mu.Lock()
	defer mu.Unlock()
	if blocked == 0 {
		t.Error("expected some requests to be rate limited with 200 concurrent requests and burst=100")
	}
}

// ========== 令牌桶单元测试 ==========

func TestTokenBucket_Allow(t *testing.T) {
	limiter := NewIPRateLimiter(10, 2)

	// 2 个突发应该通过
	if !limiter.allow("1.2.3.4") {
		t.Error("first request should be allowed")
	}
	if !limiter.allow("1.2.3.4") {
		t.Error("second request should be allowed")
	}
	// 第 3 个应该被拒绝
	if limiter.allow("1.2.3.4") {
		t.Error("third request should be denied")
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	limiter := NewIPRateLimiter(100, 1)

	if !limiter.allow("5.6.7.8") {
		t.Error("first request should be allowed")
	}
	if limiter.allow("5.6.7.8") {
		t.Error("second request should be denied")
	}

	time.Sleep(15 * time.Millisecond) // 100 rps → ~10ms per token

	if !limiter.allow("5.6.7.8") {
		t.Error("request after refill should be allowed")
	}
}

// ========== 综合安全中间件测试 ==========

func TestApplySecurityMiddlewares_FullChain(t *testing.T) {
	cfg := SecurityConfig{
		AllowedOrigins: []string{"https://trusted.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		MaxRequestSize: 1024,
		RateLimitRPS:   0, // 不限流
	}

	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	handler := applySecurityMiddlewares(base, cfg)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://trusted.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// 验证 CORS 头
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://trusted.com" {
		t.Errorf("expected CORS origin https://trusted.com, got %q", got)
	}

	// 验证安全头
	if got := rr.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("expected X-Frame-Options DENY, got %q", got)
	}
	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("expected X-Content-Type-Options nosniff, got %q", got)
	}
}

func TestApplySecurityMiddlewares_WithRateLimit(t *testing.T) {
	cfg := SecurityConfig{
		AllowedOrigins: []string{},
		AllowedMethods: []string{"GET"},
		AllowedHeaders: []string{"Content-Type"},
		MaxRequestSize: 1024,
		RateLimitRPS:   10,
		RateLimitBurst: 2,
	}

	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := applySecurityMiddlewares(base, cfg)

	// 2 个请求通过
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.10:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rr.Code)
		}
	}

	// 第 3 个被限流
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.10:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rr.Code)
	}
}

func TestDefaultSecurityConfig(t *testing.T) {
	cfg := DefaultSecurityConfig()

	if cfg.MaxRequestSize != 10*1024*1024 {
		t.Errorf("expected MaxRequestSize 10MB, got %d", cfg.MaxRequestSize)
	}
	if cfg.RateLimitRPS != 0 {
		t.Errorf("expected RateLimitRPS 0, got %f", cfg.RateLimitRPS)
	}
	if len(cfg.AllowedMethods) == 0 {
		t.Error("expected non-empty AllowedMethods")
	}
	if len(cfg.AllowedHeaders) == 0 {
		t.Error("expected non-empty AllowedHeaders")
	}
}
