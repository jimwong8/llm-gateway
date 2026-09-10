package runtime

import "time"

type ConfigChangeEvent struct {
	Module     string    `json:"module"`
	Scope      string    `json:"scope"`
	TenantID   string    `json:"tenant_id"`
	Environment string    `json:"environment,omitempty"`
	ProjectID   string    `json:"project_id,omitempty"`
	Version    string    `json:"version"`
	ChangedAt  time.Time `json:"changed_at"`
	PayloadRef string    `json:"payload_ref"`
}

type Bus interface {
	PublishConfigChange(event ConfigChangeEvent) error
	SubscribeConfigChange(handler func(ConfigChangeEvent))
}

type InProcessBus struct {
	handlers []func(ConfigChangeEvent)
}

func NewInProcessBus() *InProcessBus {
	return &InProcessBus{}
}

func (b *InProcessBus) PublishConfigChange(event ConfigChangeEvent) error {
	for _, handler := range b.handlers {
		handler(event)
	}
	return nil
}

func (b *InProcessBus) SubscribeConfigChange(handler func(ConfigChangeEvent)) {
	if handler == nil {
		return
	}
	b.handlers = append(b.handlers, handler)
}
