package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"llm-gateway/gateway/internal/config"
	"llm-gateway/gateway/internal/memory"
)

type mockPresetStore struct {
	presets   []memory.PromptPreset
	masks     []memory.MaskRule
	nextID    int64
}

func (m *mockPresetStore) CreatePreset(_ context.Context, userID int64, tenantID, name, description, template string, variables []string, tags []string, isPublic bool) (*memory.PromptPreset, error) {
	m.nextID++
	p := memory.PromptPreset{ID: m.nextID, UserID: userID, Name: name, Description: description, Template: template, IsPublic: isPublic}
	m.presets = append(m.presets, p)
	return &p, nil
}

func (m *mockPresetStore) ListPresets(_ context.Context, userID int64, tenantID string, includePublic bool) ([]memory.PromptPreset, error) {
	return m.presets, nil
}

func (m *mockPresetStore) GetPreset(_ context.Context, presetID int64, tenantID string) (*memory.PromptPreset, error) {
	for i := range m.presets {
		if m.presets[i].ID == presetID {
			return &m.presets[i], nil
		}
	}
	return nil, errNotFound
}

func (m *mockPresetStore) UpdatePreset(_ context.Context, presetID int64, tenantID, name, description, template string, variables []string, tags []string) (*memory.PromptPreset, error) {
	for i := range m.presets {
		if m.presets[i].ID == presetID {
			m.presets[i].Name = name
			m.presets[i].Description = description
			m.presets[i].Template = template
			return &m.presets[i], nil
		}
	}
	return nil, errNotFound
}

func (m *mockPresetStore) DeletePreset(_ context.Context, presetID, userID int64, tenantID string) error {
	for i, p := range m.presets {
		if p.ID == presetID && p.UserID == userID {
			m.presets = append(m.presets[:i], m.presets[i+1:]...)
			return nil
		}
	}
	return errNotFound
}

func (m *mockPresetStore) CreateMaskRule(_ context.Context, userID int64, tenantID, name, pattern, replace string) (*memory.MaskRule, error) {
	m.nextID++
	r := memory.MaskRule{ID: m.nextID, UserID: userID, Name: name, Pattern: pattern, Replace: replace, IsActive: true}
	m.masks = append(m.masks, r)
	return &r, nil
}

func (m *mockPresetStore) ListMaskRules(_ context.Context, userID int64, tenantID string) ([]memory.MaskRule, error) {
	return m.masks, nil
}

func (m *mockPresetStore) DeleteMaskRule(_ context.Context, ruleID, userID int64, tenantID string) error {
	for i, r := range m.masks {
		if r.ID == ruleID && r.UserID == userID {
			m.masks = append(m.masks[:i], m.masks[i+1:]...)
			return nil
		}
	}
	return errNotFound
}

func (m *mockPresetStore) UpdateMaskRule(_ context.Context, ruleID, userID int64, tenantID, name, pattern, replace string, enabled bool) error {
	for i := range m.masks {
		if m.masks[i].ID == ruleID && m.masks[i].UserID == userID {
			m.masks[i].Name = name
			m.masks[i].Pattern = pattern
			m.masks[i].Replace = replace
			m.masks[i].IsActive = enabled
			return nil
		}
	}
	return errNotFound
}

func newTestServerWithPresetStore(store presetStore) *Server {
	s := &Server{
		presetStore: store,
		userStore:   &noopUserStore{},
		cfg:         config.Config{JWTSecret: testJWTSecret},
	}
	return s
}

func TestPresetHandler_List(t *testing.T) {
	store := &mockPresetStore{
		presets: []memory.PromptPreset{
			{ID: 1, UserID: 1, Name: "Test Preset", Template: "Hello {{name}}"},
		},
	}
	s := newTestServerWithPresetStore(store)

	req := httptest.NewRequest(http.MethodGet, "/api/memory/presets", nil)
	req.Header.Set("Authorization", "Bearer "+validChatToken())
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []memory.PromptPreset `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 preset, got %d", len(resp.Data))
	}
	if resp.Data[0].Name != "Test Preset" {
		t.Fatalf("expected 'Test Preset', got %q", resp.Data[0].Name)
	}
}

func TestPresetHandler_Create(t *testing.T) {
	store := &mockPresetStore{}
	s := newTestServerWithPresetStore(store)

	body := `{"name":"New Preset","template":"Hello {{user}}"}`
	req := httptest.NewRequest(http.MethodPost, "/api/memory/presets", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+validChatToken())
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var p memory.PromptPreset
	if err := json.Unmarshal(rr.Body.Bytes(), &p); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if p.Name != "New Preset" {
		t.Fatalf("expected 'New Preset', got %q", p.Name)
	}
	if p.ID != 1 {
		t.Fatalf("expected ID 1, got %d", p.ID)
	}
}

func TestPresetHandler_CreateMissingName(t *testing.T) {
	store := &mockPresetStore{}
	s := newTestServerWithPresetStore(store)

	body := `{"template":"Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/memory/presets", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+validChatToken())
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestPresetHandler_Delete(t *testing.T) {
	store := &mockPresetStore{
		presets: []memory.PromptPreset{{ID: 1, UserID: 1, Name: "To Delete", Template: "test"}},
	}
	s := newTestServerWithPresetStore(store)

	req := httptest.NewRequest(http.MethodDelete, "/api/memory/presets/1", nil)
	req.Header.Set("Authorization", "Bearer "+validChatToken())
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	if len(store.presets) != 0 {
		t.Fatalf("expected 0 presets, got %d", len(store.presets))
	}
}

func TestPresetHandler_Unauthenticated(t *testing.T) {
	store := &mockPresetStore{}
	s := newTestServerWithPresetStore(store)

	req := httptest.NewRequest(http.MethodGet, "/api/memory/presets", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestMaskHandler_List(t *testing.T) {
	store := &mockPresetStore{
		masks: []memory.MaskRule{
			{ID: 1, UserID: 1, Name: "Phone Mask", Pattern: `1[3-9]\d{9}`, Replace: "[PHONE]", IsActive: true},
		},
	}
	s := newTestServerWithPresetStore(store)

	req := httptest.NewRequest(http.MethodGet, "/api/memory/masks", nil)
	req.Header.Set("Authorization", "Bearer "+validChatToken())
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []memory.MaskRule `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 mask, got %d", len(resp.Data))
	}
	if resp.Data[0].Name != "Phone Mask" {
		t.Fatalf("expected 'Phone Mask', got %q", resp.Data[0].Name)
	}
}

func TestMaskHandler_Create(t *testing.T) {
	store := &mockPresetStore{}
	s := newTestServerWithPresetStore(store)

	body := `{"name":"Email Mask","pattern":"[\\w.]+@[\\w.]+","replace":"[EMAIL]"}`
	req := httptest.NewRequest(http.MethodPost, "/api/memory/masks", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+validChatToken())
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var r memory.MaskRule
	if err := json.Unmarshal(rr.Body.Bytes(), &r); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if r.Name != "Email Mask" {
		t.Fatalf("expected 'Email Mask', got %q", r.Name)
	}
	if !r.IsActive {
		t.Fatal("expected mask to be active")
	}
}

func TestMaskHandler_Delete(t *testing.T) {
	store := &mockPresetStore{
		masks: []memory.MaskRule{{ID: 1, UserID: 1, Name: "To Delete", Pattern: "test", Replace: "[REDACTED]", IsActive: true}},
	}
	s := newTestServerWithPresetStore(store)

	req := httptest.NewRequest(http.MethodDelete, "/api/memory/masks/1", nil)
	req.Header.Set("Authorization", "Bearer "+validChatToken())
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	if len(store.masks) != 0 {
		t.Fatalf("expected 0 masks, got %d", len(store.masks))
	}
}
