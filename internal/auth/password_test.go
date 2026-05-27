package auth

import "testing"

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("secure-password-123")
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}
	if hash == "" || hash == "secure-password-123" {
		t.Fatal("hash must not be empty or plaintext")
	}
}

func TestVerifyPassword(t *testing.T) {
	hash, _ := HashPassword("secure-password-123")
	if !VerifyPassword(hash, "secure-password-123") {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatal("expected wrong password to fail")
	}
}
