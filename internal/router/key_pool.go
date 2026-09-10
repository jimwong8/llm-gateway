package router

import (
	"context"
	"errors"
	"sort"
	"strings"
)

var ErrNoProviderKey = errors.New("no eligible provider key")

type ProviderKey struct {
	ID       string
	Provider string
	Channel  string
	SecretRef string
	Weight   int
	Enabled  bool
}

type KeyPool interface {
	Next(ctx context.Context, provider string, channel string, used map[string]bool) (ProviderKey, error)
}

type StaticKeyPool struct {
	Keys []ProviderKey
}

func (p *StaticKeyPool) Next(ctx context.Context, provider string, channel string, used map[string]bool) (ProviderKey, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	var eligible []ProviderKey
	for _, k := range p.Keys {
		if !k.Enabled {
			continue
		}
		if k.Weight <= 0 {
			continue
		}
		if strings.ToLower(strings.TrimSpace(k.Provider)) != provider {
			continue
		}
		if strings.TrimSpace(k.Channel) != "" && strings.TrimSpace(channel) != "" && strings.ToLower(strings.TrimSpace(k.Channel)) != strings.ToLower(strings.TrimSpace(channel)) {
			continue
		}
		eligible = append(eligible, k)
	}
	if len(eligible) == 0 {
		return ProviderKey{}, ErrNoProviderKey
	}
	allUsed := true
	for _, k := range eligible {
		if !used[k.ID] {
			allUsed = false
			break
		}
	}
	if allUsed {
		for _, k := range eligible {
			delete(used, k.ID)
		}
	}
	sort.Slice(eligible, func(i, j int) bool {
		if eligible[i].Weight != eligible[j].Weight {
			return eligible[i].Weight > eligible[j].Weight
		}
		return eligible[i].ID < eligible[j].ID
	})
	for _, k := range eligible {
		if !used[k.ID] {
			return k, nil
		}
	}
	return eligible[0], nil
}
