package preprocess

import "sync"

type Config struct {
	NormalizeEnabled         bool   `json:"normalize_enabled"`
	SummaryEnabled           bool   `json:"summary_enabled"`
	ClassificationEnabled    bool   `json:"classification_enabled"`
	SummaryTriggerMessages   int    `json:"summary_trigger_messages,omitempty"`
	SummaryMaxRecentTurns    int    `json:"summary_max_recent_turns,omitempty"`
	SummaryModel             string `json:"summary_model,omitempty"`
	SummaryProvider          string `json:"summary_provider,omitempty"`
	ClassifierModel          string `json:"classifier_model,omitempty"`
	ClassifierProvider       string `json:"classifier_provider,omitempty"`
}

type ConfigStore struct {
	mu  sync.RWMutex
	cfg Config
}

func NewConfigStore(cfg Config) *ConfigStore {
	return &ConfigStore{cfg: cfg}
}

func (s *ConfigStore) Get() Config {
	if s == nil {
		return DefaultConfig()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *ConfigStore) Set(cfg Config) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
}

func DefaultConfig() Config {
	return Config{
		NormalizeEnabled:       false,
		SummaryEnabled:         false,
		ClassificationEnabled:  false,
		SummaryTriggerMessages: 20,
		SummaryMaxRecentTurns:  6,
	}
}
