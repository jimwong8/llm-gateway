package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Webhook 定义单个 webhook 订阅
type Webhook struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Secret    string    `json:"secret"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// Event 表示一个 webhook 事件
type Event struct {
	ID        string    `json:"id"`
	Event     string    `json:"event"`
	Payload   any       `json:"payload"`
	Timestamp time.Time `json:"timestamp"`
}

// Dispatcher 定义 webhook 发送接口
type Dispatcher interface {
	Send(ctx context.Context, event string, payload any) error
}

// HTTPWebhook 将事件 POST 到配置的 URL，带 HMAC-SHA256 签名
type HTTPWebhook struct {
	webhook Webhook
	client  *http.Client
}

// NewHTTPWebhook 创建 HTTPWebhook 实例
func NewHTTPWebhook(wh Webhook) *HTTPWebhook {
	return &HTTPWebhook{
		webhook: wh,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Send 发送事件到 webhook URL，带 HMAC-SHA256 签名
func (h *HTTPWebhook) Send(ctx context.Context, event string, payload any) error {
	if !h.webhook.Enabled {
		return nil
	}

	evt := Event{
		ID:        uuid.New().String(),
		Event:     event,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	}

	body, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal webhook event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.webhook.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Event", event)
	req.Header.Set("X-Webhook-ID", evt.ID)

	// HMAC-SHA256 签名
	if h.webhook.Secret != "" {
		mac := hmac.New(sha256.New, []byte(h.webhook.Secret))
		mac.Write(body)
		signature := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Webhook-Signature", "sha256="+signature)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// matchesEvent 检查 webhook 是否订阅了指定事件
// 支持精确匹配和 * 通配符（如 "preset.*" 匹配 "preset.created"）
func (h *HTTPWebhook) matchesEvent(event string) bool {
	for _, pattern := range h.webhook.Events {
		if pattern == event || pattern == "*" {
			return true
		}
		// 前缀通配符：如 "preset.*" 匹配 "preset.created"
		if strings.HasSuffix(pattern, ".*") {
			prefix := pattern[:len(pattern)-1] // "preset."
			if strings.HasPrefix(event, prefix) {
				return true
			}
		}
	}
	return false
}

// WebhookRegistry 管理多个 webhook 订阅
type WebhookRegistry struct {
	mu       sync.RWMutex
	webhooks map[string]*HTTPWebhook
}

// NewWebhookRegistry 创建新的 WebhookRegistry
func NewWebhookRegistry() *WebhookRegistry {
	return &WebhookRegistry{
		webhooks: make(map[string]*HTTPWebhook),
	}
}

// Register 注册一个 webhook 订阅
func (r *WebhookRegistry) Register(wh Webhook) string {
	if wh.ID == "" {
		wh.ID = uuid.New().String()
	}
	if wh.CreatedAt.IsZero() {
		wh.CreatedAt = time.Now().UTC()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.webhooks[wh.ID] = NewHTTPWebhook(wh)
	return wh.ID
}

// Unregister 删除一个 webhook 订阅
func (r *WebhookRegistry) Unregister(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.webhooks[id]; ok {
		delete(r.webhooks, id)
		return true
	}
	return false
}

// List 列出所有 webhook 订阅
func (r *WebhookRegistry) List() []Webhook {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Webhook, 0, len(r.webhooks))
	for _, h := range r.webhooks {
		result = append(result, h.webhook)
	}
	return result
}

// Get 获取指定 ID 的 webhook
func (r *WebhookRegistry) Get(id string) (Webhook, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.webhooks[id]
	if !ok {
		return Webhook{}, false
	}
	return h.webhook, true
}

// Send 向所有匹配事件的 webhook 发送事件（异步）
func (r *WebhookRegistry) Send(ctx context.Context, event string, payload any) {
	r.mu.RLock()
	matched := make([]*HTTPWebhook, 0)
	for _, h := range r.webhooks {
		if h.matchesEvent(event) {
			matched = append(matched, h)
		}
	}
	r.mu.RUnlock()

	for _, h := range matched {
		hook := h
		evt := event
		p := payload
		go func() {
			sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			_ = hook.Send(sendCtx, evt, p)
		}()
	}
}

// Count 返回注册的 webhook 数量
func (r *WebhookRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.webhooks)
}
