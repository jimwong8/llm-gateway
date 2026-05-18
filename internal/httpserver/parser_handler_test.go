package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"llm-gateway/gateway/internal/config"
)

func newTestParserServer() *Server {
	return &Server{
		userStore: &noopUserStore{},
		cfg:       config.Config{JWTSecret: testJWTSecret},
	}
}

func TestParserHandler_Unauthenticated(t *testing.T) {
	s := newTestParserServer()
	req := httptest.NewRequest(http.MethodPost, "/api/files/parse", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestParserHandler_WrongMethod(t *testing.T) {
	s := newTestParserServer()
	req := httptest.NewRequest(http.MethodGet, "/api/files/parse", nil)
	req.Header.Set("Authorization", "Bearer "+validChatToken())
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestParserHandler_MissingFile(t *testing.T) {
	s := newTestParserServer()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/files/parse", &buf)
	req.Header.Set("Authorization", "Bearer "+validChatToken())
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestParserHandler_MarkdownSuccess(t *testing.T) {
	s := newTestParserServer()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", "test.md")
	io.WriteString(fw, "# Hello\n\nThis is **markdown** content.")
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/files/parse", &buf)
	req.Header.Set("Authorization", "Bearer "+validChatToken())
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Text     string `json:"text"`
		Filename string `json:"filename"`
		Size     int    `json:"size"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Filename != "test.md" {
		t.Fatalf("expected filename 'test.md', got %q", resp.Filename)
	}
	if !strings.Contains(resp.Text, "Hello") {
		t.Fatalf("expected text to contain 'Hello', got %q", resp.Text)
	}
	if resp.Size <= 0 {
		t.Fatalf("expected positive size, got %d", resp.Size)
	}
}

func TestParserHandler_UnsupportedExtension(t *testing.T) {
	s := newTestParserServer()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", "test.xyz")
	io.WriteString(fw, "some content")
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/files/parse", &buf)
	req.Header.Set("Authorization", "Bearer "+validChatToken())
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rr.Code, rr.Body.String())
	}
}
