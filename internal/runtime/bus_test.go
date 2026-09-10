package runtime

import (
	"testing"
	"time"
)

func TestConfigChangeEventShape(t *testing.T) {
	e := ConfigChangeEvent{
		Module:     "policy",
		Scope:      "tenant",
		TenantID:   "t1",
		Version:    "v1",
		ChangedAt:  time.Now(),
		PayloadRef: "policy:t1:v1",
	}
	if e.Module == "" || e.Scope == "" || e.Version == "" {
		t.Fatalf("unexpected event shape: %+v", e)
	}
}

func TestInProcessBusPublishSubscribe(t *testing.T) {
	bus := NewInProcessBus()
	called := false
	bus.SubscribeConfigChange(func(event ConfigChangeEvent) {
		called = true
		if event.Module != "quota" {
			t.Fatalf("unexpected module: %+v", event)
		}
	})
	err := bus.PublishConfigChange(ConfigChangeEvent{Module: "quota", Scope: "tenant", TenantID: "t1", Version: "v1", ChangedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected subscriber to be called")
	}
}
