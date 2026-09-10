package router

import (
	"testing"
)

func TestStaticKeyPool_NextSkipsUsedDisabledAndZeroWeight(t *testing.T) {
	pool := &StaticKeyPool{
		Keys: []ProviderKey{
			{ID: "k1", Provider: "openai", Weight: 1, Enabled: true},
			{ID: "k2", Provider: "openai", Weight: 0, Enabled: true},
			{ID: "k3", Provider: "openai", Weight: 1, Enabled: false},
		},
	}
	used := map[string]bool{"k1": true}
	got, err := pool.Next(nil, "openai", "", used)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "k1" {
		t.Fatalf("expected k1 after reset, got %s", got.ID)
	}
}

func TestStaticKeyPool_NextResetsWhenAllEligibleKeysUsed(t *testing.T) {
	pool := &StaticKeyPool{
		Keys: []ProviderKey{
			{ID: "a", Provider: "openai", Weight: 1, Enabled: true},
			{ID: "b", Provider: "openai", Weight: 1, Enabled: true},
		},
	}
	used := map[string]bool{"a": true, "b": true}
	got, err := pool.Next(nil, "openai", "", used)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "a" && got.ID != "b" {
		t.Fatalf("expected a or b, got %s", got.ID)
	}
}

func TestStaticKeyPool_NextFiltersByProviderAndChannel(t *testing.T) {
	pool := &StaticKeyPool{
		Keys: []ProviderKey{
			{ID: "x1", Provider: "openai", Channel: "c1", Weight: 1, Enabled: true},
			{ID: "x2", Provider: "openai", Channel: "c2", Weight: 1, Enabled: true},
			{ID: "x3", Provider: "anthropic", Channel: "c1", Weight: 1, Enabled: true},
		},
	}
	got, err := pool.Next(nil, "openai", "c1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "x1" {
		t.Fatalf("expected x1, got %s", got.ID)
	}
}

func TestStaticKeyPool_NextReturnsErrorWhenNoEligibleKeys(t *testing.T) {
	pool := &StaticKeyPool{
		Keys: []ProviderKey{
			{ID: "z1", Provider: "openai", Weight: 1, Enabled: false},
		},
	}
	_, err := pool.Next(nil, "openai", "", nil)
	if err == nil {
		t.Fatal("expected error for no eligible keys")
	}
}

func TestStaticKeyPool_NextPrefersHigherWeightDeterministically(t *testing.T) {
	pool := &StaticKeyPool{
		Keys: []ProviderKey{
			{ID: "h1", Provider: "openai", Weight: 3, Enabled: true},
			{ID: "h2", Provider: "openai", Weight: 1, Enabled: true},
		},
	}
	got, err := pool.Next(nil, "openai", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "h1" {
		t.Fatalf("expected h1 (higher weight), got %s", got.ID)
	}
}
