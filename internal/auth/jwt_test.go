package auth

import (
	"testing"
	"time"
)

func TestGenerateAndValidateToken(t *testing.T) {
	secret := "test-secret-key-at-least-32-characters-long"
	token, err := GenerateToken(1, "test@example.com", "user", secret, 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}
	if token == "" {
		t.Fatal("token must not be empty")
	}

	claims, err := ValidateToken(token, secret)
	if err != nil {
		t.Fatalf("ValidateToken error: %v", err)
	}
	if claims.UserID != 1 {
		t.Fatalf("expected user ID 1, got %d", claims.UserID)
	}
	if claims.Email != "test@example.com" {
		t.Fatalf("expected email test@example.com, got %s", claims.Email)
	}
	if claims.Role != "user" {
		t.Fatalf("expected role user, got %s", claims.Role)
	}
}

func TestValidateToken_InvalidSecret(t *testing.T) {
	token, _ := GenerateToken(1, "test@example.com", "user", "secret-a", 24*time.Hour)
	_, err := ValidateToken(token, "secret-b")
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestValidateToken_Expired(t *testing.T) {
	token, _ := GenerateToken(1, "test@example.com", "user", "test-secret", 1*time.Nanosecond)
	time.Sleep(time.Millisecond)
	_, err := ValidateToken(token, "test-secret")
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}
