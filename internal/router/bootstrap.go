package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type BootstrapConfig struct {
	Channels  []Channel       `json:"channels"`
	Abilities []Ability       `json:"abilities"`
	Policy    json.RawMessage `json:"policy"`
}

func (r *Router) BootstrapFromFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read router bootstrap: %w", err)
	}

	return r.BootstrapFromJSON(raw)
}

func (r *Router) BootstrapFromJSON(raw []byte) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil
	}

	var cfg BootstrapConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("unmarshal router bootstrap: %w", err)
	}

	if cfg.Channels != nil {
		r.SetChannels(cfg.Channels)
	}
	if cfg.Abilities != nil {
		r.SetAbilities(cfg.Abilities)
	}
	if len(cfg.Policy) > 0 {
		p, err := ParsePolicyConfig(cfg.Policy)
		if err != nil {
			return fmt.Errorf("parse router bootstrap policy: %w", err)
		}
		if p != nil {
			r.SetGlobalPolicy(p)
		}
	}

	return nil
}
