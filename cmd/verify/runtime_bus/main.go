package main

import (
	"fmt"
	"time"

	"llm-gateway/gateway/internal/runtime"
)

func main() {
	bus := runtime.NewInProcessBus()
	manager := runtime.NewManager()

	bus.SubscribeConfigChange(func(event runtime.ConfigChangeEvent) {
		manager.HandleConfigChange(event, func() error {
			return nil
		})
	})

	event := runtime.ConfigChangeEvent{
		Module:     "policy",
		Scope:      "tenant",
		TenantID:   "t_demo",
		Version:    "v1",
		ChangedAt:  time.Now().UTC(),
		PayloadRef: "policy:t_demo:v1",
	}

	if err := bus.PublishConfigChange(event); err != nil {
		panic(err)
	}

	status := manager.GetStatus("policy")
	fmt.Println("module:", status.Name)
	fmt.Println("last_seen_event_version:", status.LastSeenEventVersion)
	fmt.Println("last_reload_status:", status.LastReloadStatus)
	fmt.Println("last_reload_error:", status.LastReloadError)
}
