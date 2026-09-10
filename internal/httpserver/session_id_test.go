package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveOrCreateSessionIDExplicit(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set(sessionIDHeader, "header-session")
	req.AddCookie(makeCookie(sessionIDCookie, "cookie-session"))

	sessionID, source := resolveOrCreateSessionID("body-session", req)
	if sessionID != "body-session" {
		t.Fatalf("expected body session id, got %q", sessionID)
	}
	if source != sessionIDSourceExplicit {
		t.Fatalf("expected explicit source, got %q", source)
	}
}

func TestResolveOrCreateSessionIDHeader(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set(sessionIDHeader, "header-session")
	req.AddCookie(makeCookie(sessionIDCookie, "cookie-session"))

	sessionID, source := resolveOrCreateSessionID("", req)
	if sessionID != "header-session" {
		t.Fatalf("expected header session id, got %q", sessionID)
	}
	if source != sessionIDSourceHeader {
		t.Fatalf("expected header source, got %q", source)
	}
}

func TestResolveOrCreateSessionIDCookie(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.AddCookie(makeCookie(sessionIDCookie, "cookie-session"))

	sessionID, source := resolveOrCreateSessionID("", req)
	if sessionID != "cookie-session" {
		t.Fatalf("expected cookie session id, got %q", sessionID)
	}
	if source != sessionIDSourceCookie {
		t.Fatalf("expected cookie source, got %q", source)
	}
}

func TestResolveOrCreateSessionIDGenerated(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)

	sessionID, source := resolveOrCreateSessionID("", req)
	if !strings.HasPrefix(sessionID, "oc_") {
		t.Fatalf("expected generated session id prefix oc_, got %q", sessionID)
	}
	if source != sessionIDSourceGenerated {
		t.Fatalf("expected generated source, got %q", source)
	}
	if len(sessionID) != len("oc_")+36 {
		t.Fatalf("expected generated session id length %d, got %d", len("oc_")+36, len(sessionID))
	}
}

func makeCookie(name, value string) *http.Cookie {
	return &http.Cookie{Name: name, Value: value}
}
