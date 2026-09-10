package auth

import (
	"strings"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	key, prefix, hash := GenerateAPIKey()
	if key == "" {
		t.Fatal("key must not be empty")
	}
	if len(prefix) != 8 {
		t.Fatalf("expected prefix length 8, got %d", len(prefix))
	}
	if hash == "" || hash == key {
		t.Fatal("hash must not be empty or equal to plaintext")
	}
	if !strings.HasPrefix(key, prefix) {
		t.Fatalf("key %s should start with prefix %s", key, prefix)
	}
}

func TestVerifyAPIKey(t *testing.T) {
	key, _, hash := GenerateAPIKey()
	if !VerifyAPIKey(key, hash) {
		t.Fatal("expected key to verify")
	}
	if VerifyAPIKey("wrong-key", hash) {
		t.Fatal("expected wrong key to fail")
	}
}
