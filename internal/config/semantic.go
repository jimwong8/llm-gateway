package config

import (
	"os"
	"strconv"
	"strings"
)

// SemanticCacheConfig holds semantic cache settings
type SemanticCacheConfig struct {
	Enabled     bool
	Threshold   float64
	VectorSize  int
	PersistPath string
	MaxPoints   int
	TTLHours    int
}

// EmbeddingServiceConfig holds embedding service settings
type EmbeddingServiceConfig struct {
	URL        string
	Model      string
	Dimensions int
}

// GetSemanticCacheConfig returns semantic cache configuration
func (c Config) GetSemanticCacheConfig() SemanticCacheConfig {
	return SemanticCacheConfig{
		Enabled:     c.SemanticCacheEnabled,
		Threshold:   c.SemanticCacheThreshold,
		VectorSize:  c.SemanticVectorSize,
		PersistPath: os.Getenv("SEMANTIC_CACHE_PERSIST_PATH"),
		MaxPoints:   intOrDefault(os.Getenv("SEMANTIC_CACHE_MAX_POINTS"), 10000),
		TTLHours:    intOrDefault(os.Getenv("SEMANTIC_CACHE_TTL_HOURS"), 24),
	}
}

// GetEmbeddingServiceConfig returns embedding service configuration
func (c Config) GetEmbeddingServiceConfig() EmbeddingServiceConfig {
	return EmbeddingServiceConfig{
		URL:        c.EmbeddingServiceURL,
		Model:      c.EmbeddingServiceModel,
		Dimensions: c.EmbeddingServiceDimensions,
	}
}

func intOrDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}
