package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"llm-gateway/gateway/internal/broadcast"
)

func newTestBroadcastAdminHandler() *BroadcastAdminHandler {
	return NewBroadcastAdminHandler(broadcast.NewMemoryStore())
}

func TestBroadcastAdminCreate(t *testing.T) {
	h := newTestBroadcastAdminHandler()
	body := `{"title":"test title","content":"test content","type":"info","start_at":"2025-01-01T00:00:00Z","end_at":"2026-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/broadcasts", bytes.NewReader([]byte(body)))
	req.Header.Set("X-Admin-Key", "admin-key")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp broadcast.Broadcast
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.Title != "test title" {
		t.Fatalf("expected title 'test title', got %q", resp.Title)
	}
	if resp.Type != broadcast.TypeInfo {
		t.Fatalf("expected type info, got %q", resp.Type)
	}
}

func TestBroadcastAdminCreateValidation(t *testing.T) {
	h := newTestBroadcastAdminHandler()
	tests := []struct {
		name string
		body string
	}{
		{"empty title", `{"title":"","content":"content","type":"info","start_at":"2025-01-01T00:00:00Z","end_at":"2026-01-01T00:00:00Z"}`},
		{"empty content", `{"title":"title","content":"","type":"info","start_at":"2025-01-01T00:00:00Z","end_at":"2026-01-01T00:00:00Z"}`},
		{"invalid time", `{"title":"title","content":"content","type":"info","start_at":"invalid","end_at":"2026-01-01T00:00:00Z"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/admin/broadcasts", bytes.NewReader([]byte(tc.body)))
			req.Header.Set("X-Admin-Key", "admin-key")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestBroadcastAdminList(t *testing.T) {
	h := newTestBroadcastAdminHandler()

	createBody := `{"title":"t1","content":"c1","type":"info","start_at":"2025-01-01T00:00:00Z","end_at":"2026-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/broadcasts", bytes.NewReader([]byte(createBody)))
	req.Header.Set("X-Admin-Key", "admin-key")
	h.ServeHTTP(httptest.NewRecorder(), req)

	req2 := httptest.NewRequest(http.MethodGet, "/admin/broadcasts", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req2)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data []broadcast.Broadcast `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(resp.Data))
	}
}

func TestBroadcastAdminUpdate(t *testing.T) {
	h := newTestBroadcastAdminHandler()

	createBody := `{"title":"original","content":"original content","type":"info","start_at":"2025-01-01T00:00:00Z","end_at":"2026-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/broadcasts", bytes.NewReader([]byte(createBody)))
	req.Header.Set("X-Admin-Key", "admin-key")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	var created broadcast.Broadcast
	json.Unmarshal(rr.Body.Bytes(), &created)

	updateBody := `{"title":"updated","content":"updated content","type":"warning","start_at":"2025-06-01T00:00:00Z","end_at":"2026-06-01T00:00:00Z"}`
	req2 := httptest.NewRequest(http.MethodPut, "/admin/broadcasts/"+strconv.FormatInt(created.ID, 10), bytes.NewReader([]byte(updateBody)))
	req2.Header.Set("X-Admin-Key", "admin-key")
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}
	var updated broadcast.Broadcast
	json.Unmarshal(rr2.Body.Bytes(), &updated)
	if updated.Title != "updated" || updated.Type != broadcast.TypeWarning {
		t.Fatalf("update failed: title=%q type=%q", updated.Title, updated.Type)
	}
}

func TestBroadcastAdminDelete(t *testing.T) {
	h := newTestBroadcastAdminHandler()

	createBody := `{"title":"todel","content":"to delete","type":"info","start_at":"2025-01-01T00:00:00Z","end_at":"2026-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/broadcasts", bytes.NewReader([]byte(createBody)))
	req.Header.Set("X-Admin-Key", "admin-key")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	var created broadcast.Broadcast
	json.Unmarshal(rr.Body.Bytes(), &created)

	delReq := httptest.NewRequest(http.MethodDelete, "/admin/broadcasts/"+strconv.FormatInt(created.ID, 10), nil)
	delReq.Header.Set("X-Admin-Key", "admin-key")
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, delReq)

	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/admin/broadcasts", nil)
	rr3 := httptest.NewRecorder()
	h.ServeHTTP(rr3, getReq)

	var resp struct {
		Data []broadcast.Broadcast `json:"data"`
	}
	json.Unmarshal(rr3.Body.Bytes(), &resp)
	if len(resp.Data) != 0 {
		t.Fatalf("expected 0 after delete, got %d", len(resp.Data))
	}
}

func TestBroadcastAdminNotFound(t *testing.T) {
	h := newTestBroadcastAdminHandler()

	delReq := httptest.NewRequest(http.MethodDelete, "/admin/broadcasts/999", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, delReq)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestBroadcastAdminServiceUnavailable(t *testing.T) {
	h := NewBroadcastAdminHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/admin/broadcasts", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestBroadcastUserListActive(t *testing.T) {
	store := broadcast.NewMemoryStore()
	store.Create(nil, broadcast.BroadcastInput{
		Title: "active", Content: "active content", Type: broadcast.TypeInfo,
		StartAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:   time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
	}, "admin")
	store.Create(nil, broadcast.BroadcastInput{
		Title: "expired", Content: "expired content", Type: broadcast.TypeInfo,
		StartAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:   time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC),
	}, "admin")

	h := NewBroadcastUserHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/api/user/broadcasts", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data    []broadcast.Broadcast `json:"data"`
		ReadIDs []int64              `json:"read_ids"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 active broadcast, got %d", len(resp.Data))
	}
}

func TestBroadcastUserMarkRead(t *testing.T) {
	store := broadcast.NewMemoryStore()
	store.Create(nil, broadcast.BroadcastInput{
		Title: "test", Content: "test", Type: broadcast.TypeInfo,
		StartAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:   time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
	}, "admin")

	h := NewBroadcastUserHandler(store)
	body := `{"user_id":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/user/broadcasts/1/read", bytes.NewReader([]byte(body)))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	reads, _ := store.ListReadByUser(nil, 1)
	if len(reads) != 1 {
		t.Fatalf("expected 1 read record, got %d", len(reads))
	}
}

func TestBroadcastUserMarkReadNotFound(t *testing.T) {
	store := broadcast.NewMemoryStore()
	h := NewBroadcastUserHandler(store)
	body := `{"user_id":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/user/broadcasts/999/read", bytes.NewReader([]byte(body)))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}
