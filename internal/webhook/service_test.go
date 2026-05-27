package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWebhookRegistry_Register(t *testing.T) {
	reg := NewWebhookRegistry()

	wh := Webhook{
		URL:     "https://example.com/webhook",
		Events:  []string{"preset.created", "preset.updated"},
		Secret:  "test-secret",
		Enabled: true,
	}

	id := reg.Register(wh)
	if id == "" {
		t.Fatal("expected non-empty webhook id")
	}

	if reg.Count() != 1 {
		t.Fatalf("expected 1 webhook, got %d", reg.Count())
	}

	stored, ok := reg.Get(id)
	if !ok {
		t.Fatal("expected to find registered webhook")
	}
	if stored.URL != wh.URL {
		t.Fatalf("expected URL %s, got %s", wh.URL, stored.URL)
	}
	if !stored.Enabled {
		t.Fatal("expected webhook to be enabled")
	}
	if stored.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
}

func TestWebhookRegistry_RegisterAutoID(t *testing.T) {
	reg := NewWebhookRegistry()

	wh := Webhook{
		URL:    "https://example.com/hook",
		Events: []string{"*"},
	}

	id := reg.Register(wh)
	if id == "" {
		t.Fatal("expected auto-generated id")
	}

	// 重复注册应生成不同 ID
	id2 := reg.Register(wh)
	if id == id2 {
		t.Fatal("expected different IDs for different registrations")
	}
	if reg.Count() != 2 {
		t.Fatalf("expected 2 webhooks, got %d", reg.Count())
	}
}

func TestWebhookRegistry_Unregister(t *testing.T) {
	reg := NewWebhookRegistry()

	wh := Webhook{URL: "https://example.com/hook", Events: []string{"*"}}
	id := reg.Register(wh)

	if !reg.Unregister(id) {
		t.Fatal("expected Unregister to return true")
	}
	if reg.Count() != 0 {
		t.Fatalf("expected 0 webhooks, got %d", reg.Count())
	}

	// 再次删除应返回 false
	if reg.Unregister(id) {
		t.Fatal("expected Unregister to return false for non-existent id")
	}
}

func TestWebhookRegistry_List(t *testing.T) {
	reg := NewWebhookRegistry()

	wh1 := Webhook{URL: "https://a.com/hook", Events: []string{"preset.*"}}
	wh2 := Webhook{URL: "https://b.com/hook", Events: []string{"mask.*"}}
	reg.Register(wh1)
	reg.Register(wh2)

	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 webhooks, got %d", len(list))
	}
}

func TestWebhookRegistry_Get(t *testing.T) {
	reg := NewWebhookRegistry()

	wh := Webhook{URL: "https://example.com/hook", Events: []string{"*"}, Secret: "my-secret"}
	id := reg.Register(wh)

	stored, ok := reg.Get(id)
	if !ok {
		t.Fatal("expected to find webhook")
	}
	if stored.Secret != "my-secret" {
		t.Fatalf("expected secret 'my-secret', got '%s'", stored.Secret)
	}

	_, ok = reg.Get("non-existent")
	if ok {
		t.Fatal("expected not found for non-existent id")
	}
}

func TestHTTPWebhook_MatchesEvent(t *testing.T) {
	tests := []struct {
		name     string
		events   []string
		event    string
		expected bool
	}{
		{"exact match", []string{"preset.created"}, "preset.created", true},
		{"no match", []string{"preset.created"}, "mask.created", false},
		{"wildcard", []string{"*"}, "anything.event", true},
		{"prefix no wildcard", []string{"preset."}, "preset.created", false},
		{"empty events", []string{}, "preset.created", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &HTTPWebhook{
				webhook: Webhook{Events: tt.events},
			}
			if got := h.matchesEvent(tt.event); got != tt.expected {
				t.Fatalf("matchesEvent(%q) = %v, want %v", tt.event, got, tt.expected)
			}
		})
	}
}

func TestHTTPWebhook_Send(t *testing.T) {
	var (
		mu       sync.Mutex
		received *Event
		headers  http.Header
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		headers = r.Header.Clone()
		mu.Unlock()

		var evt Event
		if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		mu.Lock()
		received = &evt
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	h := NewHTTPWebhook(Webhook{
		URL:     server.URL,
		Events:  []string{"preset.created"},
		Secret:  "test-secret",
		Enabled: true,
	})

	payload := map[string]any{"id": 1, "name": "test-preset"}
	ctx := context.Background()
	if err := h.Send(ctx, "preset.created", payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证事件内容
	mu.Lock()
	defer mu.Unlock()
	if received == nil {
		t.Fatal("expected to receive event")
	}
	if received.Event != "preset.created" {
		t.Fatalf("expected event 'preset.created', got '%s'", received.Event)
	}
	if received.ID == "" {
		t.Fatal("expected event ID to be set")
	}
	if received.Payload == nil {
		t.Fatal("expected payload to be set")
	}

	// 验证签名头
	sig := headers.Get("X-Webhook-Signature")
	if sig == "" {
		t.Fatal("expected X-Webhook-Signature header")
	}
	if !strings.HasPrefix(sig, "sha256=") {
		t.Fatalf("expected signature to start with 'sha256=', got '%s'", sig)
	}

	// 验证事件类型头
	if evt := headers.Get("X-Webhook-Event"); evt != "preset.created" {
		t.Fatalf("expected X-Webhook-Event 'preset.created', got '%s'", evt)
	}
}

func TestHTTPWebhook_SendDisabled(t *testing.T) {
	h := NewHTTPWebhook(Webhook{
		URL:     "https://example.com/webhook",
		Events:  []string{"*"},
		Enabled: false,
	})

	ctx := context.Background()
	if err := h.Send(ctx, "test.event", nil); err != nil {
		t.Fatalf("expected no error for disabled webhook, got: %v", err)
	}
}

func TestHTTPWebhook_SendServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer server.Close()

	h := NewHTTPWebhook(Webhook{
		URL:     server.URL,
		Events:  []string{"*"},
		Enabled: true,
	})

	ctx := context.Background()
	err := h.Send(ctx, "test.event", nil)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected error to contain status code, got: %v", err)
	}
}

func TestWebhookRegistry_Send(t *testing.T) {
	var (
		mu       sync.Mutex
		events   []string
		payloads []any
		wg       sync.WaitGroup
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var evt Event
		if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		mu.Lock()
		events = append(events, evt.Event)
		payloads = append(payloads, evt.Payload)
		mu.Unlock()
		wg.Done()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	reg := NewWebhookRegistry()
	reg.Register(Webhook{URL: server.URL, Events: []string{"preset.*"}, Enabled: true})
	reg.Register(Webhook{URL: server.URL, Events: []string{"mask.*"}, Enabled: true})
	reg.Register(Webhook{URL: server.URL, Events: []string{"*"}, Enabled: true})

	// preset.created → preset.* + * = 2, mask.deleted → mask.* + * = 2, 共 4
	wg.Add(4)
	ctx := context.Background()
	reg.Send(ctx, "preset.created", map[string]any{"id": 1})
	reg.Send(ctx, "mask.deleted", map[string]any{"id": 2})

	// 等待所有事件发送完成
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		// ok
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for webhook events")
	}

	mu.Lock()
	defer mu.Unlock()

	if len(events) != 4 {
		t.Fatalf("expected 4 events sent, got %d: %v", len(events), events)
	}

	presetCount := 0
	maskCount := 0
	for _, e := range events {
		if e == "preset.created" {
			presetCount++
		}
		if e == "mask.deleted" {
			maskCount++
		}
	}
	if presetCount != 2 {
		t.Fatalf("expected 2 preset.created events, got %d", presetCount)
	}
	if maskCount != 2 {
		t.Fatalf("expected 2 mask.deleted events, got %d", maskCount)
	}
}

func TestWebhookRegistry_SendNoMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not receive any event")
	}))
	defer server.Close()

	reg := NewWebhookRegistry()
	reg.Register(Webhook{URL: server.URL, Events: []string{"preset.*"}, Enabled: true})

	ctx := context.Background()
	reg.Send(ctx, "mask.created", nil)

	// 等待确保不会触发
	time.Sleep(500 * time.Millisecond)
}

func TestWebhookRegistry_ConcurrentAccess(t *testing.T) {
	reg := NewWebhookRegistry()

	// 并发注册
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			reg.Register(Webhook{
				URL:    "https://example.com/hook",
				Events: []string{"*"},
			})
		}(i)
	}
	wg.Wait()

	if reg.Count() != 100 {
		t.Fatalf("expected 100 webhooks, got %d", reg.Count())
	}

	// 并发读取
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = reg.List()
		}()
	}
	wg.Wait()
}
