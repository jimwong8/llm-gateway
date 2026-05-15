package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	AppEnv                   string
	AppName                  string
	AppPort                  string
	RouterBootstrapPath      string
	PostgresDSN              string
	RedisAddr                string
	RedisPassword            string
	RedisDB                  int
	L1CacheTTLSeconds        int
	QdrantURL                string
	QdrantAPIKey             string
	QdrantCollection         string
	SemanticCacheEnabled     bool
	SemanticCacheThreshold   float64
	SemanticVectorSize       int
	MemoryEnabled            bool
	MemoryMaxItems           int
	DefaultProvider          string
	DefaultModel             string
	MockMode                 bool
	OpenAIBaseURL            string
	OpenAIAPIKey             string
	OpenAITimeoutSec         int
	XSTXBaseURL              string
	XSTXAPIKey               string
	XSTXTimeoutSec         int
	AnthropicBaseURL        string
	AnthropicAPIKey         string
	AnthropicTimeoutSec     int
	AuditLogEnabled          bool
	BillingEnabled           bool
	TenantRPM                int
	AdminAPIKey              string
	ProviderMaxRetries       int
	ProviderFailureThreshold int
	ProviderOpenSeconds      int
	ProviderHealthTimeoutSec int
	ModelGovernanceEnabled   bool
	ModelGovernanceCacheTTLSeconds int
	ModelGovernanceRolloutMaxErrorRate float64
	ModelGovernanceRolloutMaxP95MS int
	ModelGovernanceRolloutMaxFallbackRate float64
	ModelGovernanceMinSampleCount int
}

func Load() Config {
	cfg := Config{
		AppEnv:                   getenv("APP_ENV", "development"),
		AppName:                  getenv("APP_NAME", "llm-gateway"),
		AppPort:                  getenv("APP_PORT", "8080"),
		RouterBootstrapPath:      strings.TrimSpace(getenv("ROUTER_BOOTSTRAP_PATH", "")),
		PostgresDSN:              getenv("POSTGRES_DSN", "postgres://llmadmin:CHANGE_ME@127.0.0.1:5432/llmgateway?sslmode=disable"),
		RedisAddr:                getenv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:            getenv("REDIS_PASSWORD", ""),
		RedisDB:                  getenvInt("REDIS_DB", 0),
		L1CacheTTLSeconds:        getenvInt("L1_CACHE_TTL_SECONDS", 600),
		QdrantURL:                getenv("QDRANT_URL", "http://127.0.0.1:6333"),
		QdrantAPIKey:             getenv("QDRANT_API_KEY", "CHANGE_ME"),
		QdrantCollection:         getenv("QDRANT_COLLECTION", "semantic_cache_v1"),
		SemanticCacheEnabled:     getenvBool("SEMANTIC_CACHE_ENABLED", true),
		SemanticCacheThreshold:   getenvFloat("SEMANTIC_CACHE_THRESHOLD", 0.80),
		SemanticVectorSize:       getenvInt("SEMANTIC_VECTOR_SIZE", 64),
		MemoryEnabled:            getenvBool("MEMORY_ENABLED", true),
		MemoryMaxItems:           getenvInt("MEMORY_MAX_ITEMS", 3),
		DefaultProvider:          getenv("DEFAULT_PROVIDER", "openai"),
		DefaultModel:             getenv("DEFAULT_MODEL", "gpt-4o-mini"),
		MockMode:                 getenvBool("MOCK_MODE", true),
		OpenAIBaseURL:            strings.TrimRight(getenv("OPENAI_BASE_URL", "https://api.openai.com/v1"), "/"),
		OpenAIAPIKey:             getenv("OPENAI_API_KEY", ""),
		OpenAITimeoutSec:         getenvInt("OPENAI_TIMEOUT_SEC", 120),
		XSTXBaseURL:              strings.TrimRight(getenv("XSTX_BASE_URL", "https://api.xstx.info/v1"), "/"),
		XSTXAPIKey:               getenv("XSTX_API_KEY", ""),
		XSTXTimeoutSec:         getenvInt("XSTX_TIMEOUT_SEC", 120),
		AnthropicBaseURL:    strings.TrimRight(getenv("ANTHROPIC_BASE_URL", "https://api.anthropic.com/v1"), "/"),
		AnthropicAPIKey:     getenv("ANTHROPIC_API_KEY", ""),
		AnthropicTimeoutSec: getenvInt("ANTHROPIC_TIMEOUT_SEC", 120),
		AuditLogEnabled:          getenvBool("AUDIT_LOG_ENABLED", true),
		BillingEnabled:           getenvBool("BILLING_ENABLED", true),
		TenantRPM:                getenvInt("TENANT_RPM", 60),
		AdminAPIKey:              getenv("ADMIN_API_KEY", "ok0115ok"),
		ProviderMaxRetries:       getenvInt("PROVIDER_MAX_RETRIES", 1),
		ProviderFailureThreshold: getenvInt("PROVIDER_FAILURE_THRESHOLD", 2),
		ProviderOpenSeconds:      getenvInt("PROVIDER_OPEN_SECONDS", 30),
		ProviderHealthTimeoutSec: getenvInt("PROVIDER_HEALTH_TIMEOUT_SEC", 5),
		ModelGovernanceEnabled:   getenvBool("MODEL_GOVERNANCE_ENABLED", true),
		ModelGovernanceCacheTTLSeconds: getenvInt("MODEL_GOVERNANCE_CACHE_TTL_SECONDS", 60),
		ModelGovernanceRolloutMaxErrorRate: getenvFloat("MODEL_GOVERNANCE_ROLLOUT_MAX_ERROR_RATE", 0.02),
		ModelGovernanceRolloutMaxP95MS: getenvInt("MODEL_GOVERNANCE_ROLLOUT_MAX_P95_MS", 1200),
		ModelGovernanceRolloutMaxFallbackRate: getenvFloat("MODEL_GOVERNANCE_ROLLOUT_MAX_FALLBACK_RATE", 0.15),
		ModelGovernanceMinSampleCount: getenvInt("MODEL_GOVERNANCE_MIN_SAMPLE_COUNT", 200),
	}
	return cfg
}

func (c Config) Addr() string { return fmt.Sprintf(":%s", c.AppPort) }

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func getenvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return fallback
	}
	return parsed
}

func getenvFloat(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var parsed float64
	if _, err := fmt.Sscanf(value, "%f", &parsed); err != nil {
		return fallback
	}
	return parsed
}
