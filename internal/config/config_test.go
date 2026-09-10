package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	clearEnv(t, allConfigKeys)

	cfg := Load()

	if cfg.AppEnv != "development" {
		t.Errorf("AppEnv = %q, want %q", cfg.AppEnv, "development")
	}
	if cfg.AppName != "llm-gateway" {
		t.Errorf("AppName = %q, want %q", cfg.AppName, "llm-gateway")
	}
	if cfg.AppPort != "8080" {
		t.Errorf("AppPort = %q, want %q", cfg.AppPort, "8080")
	}
	if cfg.RedisAddr != "127.0.0.1:6379" {
		t.Errorf("RedisAddr = %q, want %q", cfg.RedisAddr, "127.0.0.1:6379")
	}
	if cfg.L1CacheTTLSeconds != 600 {
		t.Errorf("L1CacheTTLSeconds = %d, want %d", cfg.L1CacheTTLSeconds, 600)
	}
	if !cfg.SemanticCacheEnabled {
		t.Errorf("SemanticCacheEnabled = %v, want true", cfg.SemanticCacheEnabled)
	}
	if cfg.SemanticCacheThreshold != 0.80 {
		t.Errorf("SemanticCacheThreshold = %f, want 0.80", cfg.SemanticCacheThreshold)
	}
	if cfg.SemanticVectorSize != 64 {
		t.Errorf("SemanticVectorSize = %d, want 64", cfg.SemanticVectorSize)
	}
	if !cfg.MemoryEnabled {
		t.Errorf("MemoryEnabled = %v, want true", cfg.MemoryEnabled)
	}
	if cfg.MemoryMaxItems != 3 {
		t.Errorf("MemoryMaxItems = %d, want 3", cfg.MemoryMaxItems)
	}
	if cfg.DefaultProvider != "openai" {
		t.Errorf("DefaultProvider = %q, want %q", cfg.DefaultProvider, "openai")
	}
	if cfg.DefaultModel != "gpt-4o-mini" {
		t.Errorf("DefaultModel = %q, want %q", cfg.DefaultModel, "gpt-4o-mini")
	}
	if !cfg.MockMode {
		t.Errorf("MockMode = %v, want true", cfg.MockMode)
	}
	if cfg.OpenAIBaseURL != "https://api.openai.com/v1" {
		t.Errorf("OpenAIBaseURL = %q, want %q", cfg.OpenAIBaseURL, "https://api.openai.com/v1")
	}
	if cfg.TenantRPM != 60 {
		t.Errorf("TenantRPM = %d, want 60", cfg.TenantRPM)
	}
	if cfg.AdminAPIKey != "ok0115ok" {
		t.Errorf("AdminAPIKey = %q, want %q", cfg.AdminAPIKey, "ok0115ok")
	}
	if cfg.ProviderMaxRetries != 1 {
		t.Errorf("ProviderMaxRetries = %d, want 1", cfg.ProviderMaxRetries)
	}
	if cfg.ProviderFailureThreshold != 2 {
		t.Errorf("ProviderFailureThreshold = %d, want 2", cfg.ProviderFailureThreshold)
	}
	if cfg.ProviderOpenSeconds != 30 {
		t.Errorf("ProviderOpenSeconds = %d, want 30", cfg.ProviderOpenSeconds)
	}
	if cfg.ProviderHealthTimeoutSec != 5 {
		t.Errorf("ProviderHealthTimeoutSec = %d, want 5", cfg.ProviderHealthTimeoutSec)
	}
	if !cfg.ModelGovernanceEnabled {
		t.Errorf("ModelGovernanceEnabled = %v, want true", cfg.ModelGovernanceEnabled)
	}
	if cfg.ModelGovernanceCacheTTLSeconds != 60 {
		t.Errorf("ModelGovernanceCacheTTLSeconds = %d, want 60", cfg.ModelGovernanceCacheTTLSeconds)
	}
	if cfg.ModelGovernanceRolloutMaxErrorRate != 0.02 {
		t.Errorf("ModelGovernanceRolloutMaxErrorRate = %f, want 0.02", cfg.ModelGovernanceRolloutMaxErrorRate)
	}
	if cfg.ModelGovernanceRolloutMaxP95MS != 1200 {
		t.Errorf("ModelGovernanceRolloutMaxP95MS = %d, want 1200", cfg.ModelGovernanceRolloutMaxP95MS)
	}
	if cfg.ModelGovernanceRolloutMaxFallbackRate != 0.15 {
		t.Errorf("ModelGovernanceRolloutMaxFallbackRate = %f, want 0.15", cfg.ModelGovernanceRolloutMaxFallbackRate)
	}
	if cfg.ModelGovernanceMinSampleCount != 200 {
		t.Errorf("ModelGovernanceMinSampleCount = %d, want 200", cfg.ModelGovernanceMinSampleCount)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_PORT", "9090")
	t.Setenv("REDIS_ADDR", "redis.example.com:6380")
	t.Setenv("MOCK_MODE", "false")
	t.Setenv("DEFAULT_MODEL", "claude-3")
	t.Setenv("L1_CACHE_TTL_SECONDS", "300")
	t.Setenv("SEMANTIC_CACHE_THRESHOLD", "0.95")
	t.Setenv("TENANT_RPM", "120")
	t.Setenv("PROVIDER_MAX_RETRIES", "3")

	cfg := Load()

	if cfg.AppEnv != "production" {
		t.Errorf("AppEnv = %q, want %q", cfg.AppEnv, "production")
	}
	if cfg.AppPort != "9090" {
		t.Errorf("AppPort = %q, want %q", cfg.AppPort, "9090")
	}
	if cfg.RedisAddr != "redis.example.com:6380" {
		t.Errorf("RedisAddr = %q, want %q", cfg.RedisAddr, "redis.example.com:6380")
	}
	if cfg.MockMode {
		t.Errorf("MockMode = %v, want false", cfg.MockMode)
	}
	if cfg.DefaultModel != "claude-3" {
		t.Errorf("DefaultModel = %q, want %q", cfg.DefaultModel, "claude-3")
	}
	if cfg.L1CacheTTLSeconds != 300 {
		t.Errorf("L1CacheTTLSeconds = %d, want %d", cfg.L1CacheTTLSeconds, 300)
	}
	if cfg.SemanticCacheThreshold != 0.95 {
		t.Errorf("SemanticCacheThreshold = %f, want %f", cfg.SemanticCacheThreshold, 0.95)
	}
	if cfg.TenantRPM != 120 {
		t.Errorf("TenantRPM = %d, want %d", cfg.TenantRPM, 120)
	}
	if cfg.ProviderMaxRetries != 3 {
		t.Errorf("ProviderMaxRetries = %d, want %d", cfg.ProviderMaxRetries, 3)
	}
}

func TestLoadBoolVariants(t *testing.T) {
	tests := []struct {
		envValue string
		expected bool
	}{
		{"true", true},
		{"TRUE", true},
		{"1", true},
		{"yes", true},
		{"on", true},
		{"false", false},
		{"FALSE", false},
		{"0", false},
		{"no", false},
		{"off", false},
		{"", true},
		{"invalid", true},
	}

	for _, tt := range tests {
		t.Run("MOCK_MODE="+tt.envValue, func(t *testing.T) {
			if tt.envValue == "" {
				os.Unsetenv("MOCK_MODE")
			} else {
				t.Setenv("MOCK_MODE", tt.envValue)
			}
			cfg := Load()
			if cfg.MockMode != tt.expected {
				t.Errorf("MockMode with env=%q: got %v, want %v", tt.envValue, cfg.MockMode, tt.expected)
			}
		})
	}
}

func TestLoadIntFallback(t *testing.T) {
	t.Setenv("L1_CACHE_TTL_SECONDS", "not-a-number")
	cfg := Load()
	if cfg.L1CacheTTLSeconds != 600 {
		t.Errorf("L1CacheTTLSeconds = %d, want 600 (fallback)", cfg.L1CacheTTLSeconds)
	}
}

func TestLoadFloatFallback(t *testing.T) {
	t.Setenv("SEMANTIC_CACHE_THRESHOLD", "not-a-float")
	cfg := Load()
	if cfg.SemanticCacheThreshold != 0.80 {
		t.Errorf("SemanticCacheThreshold = %f, want 0.80 (fallback)", cfg.SemanticCacheThreshold)
	}
}

func TestConfigAddr(t *testing.T) {
	cfg := Config{AppPort: "8080"}
	if addr := cfg.Addr(); addr != ":8080" {
		t.Errorf("Addr() = %q, want %q", addr, ":8080")
	}

	cfg.AppPort = "9090"
	if addr := cfg.Addr(); addr != ":9090" {
		t.Errorf("Addr() = %q, want %q", addr, ":9090")
	}
}

func TestOpenAIBaseURLTrimming(t *testing.T) {
	t.Setenv("OPENAI_BASE_URL", "https://custom.openai.com/v1/")
	cfg := Load()
	if cfg.OpenAIBaseURL != "https://custom.openai.com/v1" {
		t.Errorf("OpenAIBaseURL = %q, want trailing slash trimmed", cfg.OpenAIBaseURL)
	}
}

func TestXSTXBaseURLTrimming(t *testing.T) {
	t.Setenv("XSTX_BASE_URL", "https://api.xstx.info/v1/")
	cfg := Load()
	if cfg.XSTXBaseURL != "https://api.xstx.info/v1" {
		t.Errorf("XSTXBaseURL = %q, want trailing slash trimmed", cfg.XSTXBaseURL)
	}
}

func TestAnthropicBaseURLTrimming(t *testing.T) {
	t.Setenv("ANTHROPIC_BASE_URL", "https://api.anthropic.com/v1/")
	cfg := Load()
	if cfg.AnthropicBaseURL != "https://api.anthropic.com/v1" {
		t.Errorf("AnthropicBaseURL = %q, want trailing slash trimmed", cfg.AnthropicBaseURL)
	}
}

var allConfigKeys = []string{
	"APP_ENV", "APP_NAME", "APP_PORT", "POSTGRES_DSN", "REDIS_ADDR",
	"REDIS_PASSWORD", "REDIS_DB", "L1_CACHE_TTL_SECONDS", "QDRANT_URL",
	"QDRANT_API_KEY", "QDRANT_COLLECTION", "SEMANTIC_CACHE_ENABLED",
	"SEMANTIC_CACHE_THRESHOLD", "SEMANTIC_VECTOR_SIZE", "MEMORY_ENABLED",
	"MEMORY_MAX_ITEMS", "DEFAULT_PROVIDER", "DEFAULT_MODEL", "MOCK_MODE",
	"OPENAI_BASE_URL", "OPENAI_API_KEY", "OPENAI_TIMEOUT_SEC",
	"XSTX_BASE_URL", "XSTX_API_KEY", "XSTX_TIMEOUT_SEC",
	"ANTHROPIC_BASE_URL", "ANTHROPIC_API_KEY", "ANTHROPIC_TIMEOUT_SEC",
	"AUDIT_LOG_ENABLED", "BILLING_ENABLED", "TENANT_RPM", "ADMIN_API_KEY",
	"PROVIDER_MAX_RETRIES", "PROVIDER_FAILURE_THRESHOLD", "PROVIDER_OPEN_SECONDS",
	"PROVIDER_HEALTH_TIMEOUT_SEC", "MODEL_GOVERNANCE_ENABLED",
	"MODEL_GOVERNANCE_CACHE_TTL_SECONDS", "MODEL_GOVERNANCE_ROLLOUT_MAX_ERROR_RATE",
	"MODEL_GOVERNANCE_ROLLOUT_MAX_P95_MS", "MODEL_GOVERNANCE_ROLLOUT_MAX_FALLBACK_RATE",
	"MODEL_GOVERNANCE_MIN_SAMPLE_COUNT", "ROUTER_BOOTSTRAP_PATH",
}

func clearEnv(t *testing.T, keys []string) {
	t.Helper()
	for _, key := range keys {
		os.Unsetenv(key)
	}
}
