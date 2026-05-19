package email

import (
	"testing"
)

func TestNew_WithEmptyConfig(t *testing.T) {
	svc := New(Config{})
	if svc.isEnabled() {
		t.Fatal("expected disabled when no config")
	}
}

func TestNew_WithFullConfig(t *testing.T) {
	svc := New(Config{Host: "smtp.example.com", Port: 587, User: "user", Password: "pass", From: "from@example.com"})
	if !svc.isEnabled() {
		t.Fatal("expected enabled with full config")
	}
}

func TestSendVerificationEmail_Disabled(t *testing.T) {
	svc := New(Config{})
	err := svc.SendVerificationEmail("test@example.com", "token123", "http://localhost")
	if err != nil {
		t.Fatalf("expected no error when disabled, got: %v", err)
	}
}

func TestSendPasswordResetEmail_Disabled(t *testing.T) {
	svc := New(Config{})
	err := svc.SendPasswordResetEmail("test@example.com", "token123", "http://localhost")
	if err != nil {
		t.Fatalf("expected no error when disabled, got: %v", err)
	}
}
