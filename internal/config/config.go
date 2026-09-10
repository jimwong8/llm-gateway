package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	AppEnv                                string
	AppName                               string
	AppPort                               string
	RouterBootstrapPath                   string
	PostgresDSN                           string
	RedisAddr                             string
	RedisPassword                         string
	RedisDB                               int
	L1CacheTTLSeconds                     int
	QdrantURL                             string
	QdrantAPIKey                          string
	QdrantCollection                      string
	SemanticCacheEnabled                  bool
	EmbeddingServiceURL                   string
	EmbeddingServiceModel                 string
	EmbeddingServiceDimensions            int
	DefaultModelPool                      string
	ModelContextWindows                   map[string]int
	ContextSafetyRatio                    float64
	ContextAutoCompressEnabled            bool
	ContextCompressModel                  string
	ContextCompressMaxRecentTurns         int
	ProviderChannelRPM                    int
	ProviderChannelMaxConcurrency         int
	ProviderChannelMinIntervalMS          int
	SemanticCacheThreshold                float64
	SemanticVectorSize                    int
	MemoryEnabled                         bool
	MemoryMaxItems                        int
	DefaultProvider                       string
	DefaultModel                          string
	MockMode                              bool
	OpenAIBaseURL                         string
	OpenAIAPIKey                          string
	OpenAITimeoutSec                      int
	XSTXBaseURL                           string
	XSTXAPIKey                            string
	XSTXTimeoutSec                        int
	AnthropicBaseURL                      string
	AnthropicAPIKey                       string
	AnthropicTimeoutSec                   int
	AuditLogEnabled                       bool
	BillingEnabled                        bool
	TenantRPM                             int
	AdminAPIKey                           string
	JWTSecret                             string
	ProviderMaxRetries                    int
	ProviderFailureThreshold              int
	ProviderOpenSeconds                   int
	ProviderHealthTimeoutSec              int
	ModelGovernanceEnabled                bool
	ModelGovernanceCacheTTLSeconds        int
	ModelGovernanceRolloutMaxErrorRate    float64
	ModelGovernanceRolloutMaxP95MS        int
	ModelGovernanceRolloutMaxFallbackRate float64
	ModelGovernanceMinSampleCount         int
	FallbackMaxDepth                      int
	FallbackMinScoreRatio                 float64
	AuditRetentionDays                    int
	DefaultAPIKeyRPM                      int
	LongContextEnabled                    bool
	LongContextMaxInputBytes              int
	LongContextWorkerChannel              string
	LongContextWorkerModel                string
	LongContextWorkerReaderChannel        string
	LongContextWorkerReaderModel          string
	LongContextWorkerReaderTopK           int
	LongContextWorkers                    int
	LongContextWorkerChannels             string
	LongContextWorkerReaderCandidates     string
	LongContextEmbeddingURL               string
	LongContextEmbeddingModel             string
	LongContextEmbeddingDimensions        int
	LongContextEmbeddingKey               string
	MaskEnabled                           bool
	GitHubClientID                        string
	GitHubClientSecret                    string
	SMTPHost                              string
	SMTPPort                              int
	SMTPUser                              string
	SMTPPassword                          string
	SMTPFrom                              string
	CouncilCandidates                    string
	CouncilSynthesizer                    string
	ReasoningAutoOffModels                string
	ReasoningStripContent                 bool
}

func Load() Config {
	cfg := Config{
		AppEnv:                                getenv("APP_ENV", "development"),
		AppName:                               getenv("APP_NAME", "llm-gateway"),
		AppPort:                               getenv("APP_PORT", "8080"),
		RouterBootstrapPath:                   strings.TrimSpace(getenv("ROUTER_BOOTSTRAP_PATH", "")),
		PostgresDSN:                           getenv("POSTGRES_DSN", "postgres://llmadmin:CHANGE_ME@127.0.0.1:5432/llmgateway?sslmode=disable"),
		RedisAddr:                             getenv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:                         getenv("REDIS_PASSWORD", ""),
		RedisDB:                               getenvInt("REDIS_DB", 0),
		L1CacheTTLSeconds:                     getenvInt("L1_CACHE_TTL_SECONDS", 600),
		QdrantURL:                             getenv("QDRANT_URL", "http://127.0.0.1:6333"),
		QdrantAPIKey:                          getenv("QDRANT_API_KEY", "CHANGE_ME"),
		QdrantCollection:                      getenv("QDRANT_COLLECTION", "semantic_cache_v1"),
		SemanticCacheEnabled:                  getenvBool("SEMANTIC_CACHE_ENABLED", true),
		LongContextEnabled:                    getenvBool("LONG_CONTEXT_ENABLED", false),
		LongContextMaxInputBytes:              getenvInt("LONG_CONTEXT_MAX_INPUT_BYTES", 4000000),
		LongContextWorkerChannel:              getenv("LONG_CONTEXT_WORKER_CHANNEL", ""),
		LongContextWorkerModel:                getenv("LONG_CONTEXT_WORKER_MODEL", "deepseek-v4-flash"),
		LongContextWorkerReaderChannel:        getenv("LONG_CONTEXT_WORKER_READER_CHANNEL", ""),
		LongContextWorkerReaderModel:          getenv("LONG_CONTEXT_WORKER_READER_MODEL", "sensenova-6.8-flash-lite"),
		LongContextWorkerReaderTopK:           getenvInt("LONG_CONTEXT_WORKER_READER_TOP_K", 16),
		LongContextWorkers:                    getenvInt("LONG_CONTEXT_WORKERS", 0),
		LongContextWorkerChannels:             getenv("LONG_CONTEXT_WORKER_CHANNELS", ""),
		CouncilCandidates:                    getenv("COUNCIL_CANDIDATES", "deepseek-v4-flash,glm-5.2,qwen3.8-flash"),
		CouncilSynthesizer:                    getenv("COUNCIL_SYNTHESIZER", "sensenova-6.8-flash-lite"),
		ReasoningAutoOffModels:                getenv("REASONING_AUTO_OFF_MODELS", "deepseek-v4-flash,deepseek-v4-flash-0731,glm-5.2,glm-5.3,glm-5.3-flash,minimaxai/minimax-m3,MiniMax-M3,deepseek-ai/deepseek-v4-pro-0813,minimax/minimax-m3:free,nvidia/nemotron-3-ultra-550b-a55b:free,nvidia/nemotron-3.5-lightning:free"),
		ReasoningStripContent:                 getenvBool("REASONING_STRIP_CONTENT", false),
		LongContextWorkerReaderCandidates:     getenv("LONG_CONTEXT_WORKER_READER_CANDIDATES", ""),
		LongContextEmbeddingURL:               getenv("LONG_CONTEXT_EMBEDDING_URL", ""),
		LongContextEmbeddingModel:             getenv("LONG_CONTEXT_EMBEDDING_MODEL", ""),
		LongContextEmbeddingDimensions:        getenvInt("LONG_CONTEXT_EMBEDDING_DIMENSIONS", 0),
		LongContextEmbeddingKey:               getenv("LONG_CONTEXT_EMBEDDING_KEY", ""),
		SemanticCacheThreshold:                getenvFloat("SEMANTIC_CACHE_THRESHOLD", 0.80),
		SemanticVectorSize:                    getenvInt("SEMANTIC_VECTOR_SIZE", 64),
		MemoryEnabled:                         getenvBool("MEMORY_ENABLED", true),
		MemoryMaxItems:                        getenvInt("MEMORY_MAX_ITEMS", 3),
		DefaultProvider:                       getenv("DEFAULT_PROVIDER", "openai"),
		DefaultModel:                          getenv("DEFAULT_MODEL", "gpt-4o-mini"),
		DefaultModelPool:                      getenv("DEFAULT_MODEL_POOL", ""),
		MockMode:                              getenvBool("MOCK_MODE", true),
		OpenAIBaseURL:                         strings.TrimRight(getenv("OPENAI_BASE_URL", "https://api.openai.com/v1"), "/"),
		OpenAIAPIKey:                          getenv("OPENAI_API_KEY", ""),
		OpenAITimeoutSec:                      getenvInt("OPENAI_TIMEOUT_SEC", 120),
		XSTXBaseURL:                           strings.TrimRight(getenv("XSTX_BASE_URL", "https://api.xstx.info/v1"), "/"),
		XSTXAPIKey:                            getenv("XSTX_API_KEY", ""),
		XSTXTimeoutSec:                        getenvInt("XSTX_TIMEOUT_SEC", 120),
		AnthropicBaseURL:                      strings.TrimRight(getenv("ANTHROPIC_BASE_URL", "https://api.anthropic.com/v1"), "/"),
		AnthropicAPIKey:                       getenv("ANTHROPIC_API_KEY", ""),
		AnthropicTimeoutSec:                   getenvInt("ANTHROPIC_TIMEOUT_SEC", 120),
		AuditLogEnabled:                       getenvBool("AUDIT_LOG_ENABLED", true),
		BillingEnabled:                        getenvBool("BILLING_ENABLED", true),
		TenantRPM:                             getenvInt("TENANT_RPM", 60),
		AdminAPIKey:                           getenv("ADMIN_API_KEY", "ok0115ok"),
		JWTSecret:                             getenv("JWT_SECRET", "change-me-jwt-secret-at-least-32-characters"),
		ProviderMaxRetries:                    getenvInt("PROVIDER_MAX_RETRIES", 1),
		ProviderFailureThreshold:              getenvInt("PROVIDER_FAILURE_THRESHOLD", 2),
		ProviderOpenSeconds:                   getenvInt("PROVIDER_OPEN_SECONDS", 30),
		ProviderHealthTimeoutSec:              getenvInt("PROVIDER_HEALTH_TIMEOUT_SEC", 5),
		ModelGovernanceEnabled:                getenvBool("MODEL_GOVERNANCE_ENABLED", true),
		ModelGovernanceCacheTTLSeconds:        getenvInt("MODEL_GOVERNANCE_CACHE_TTL_SECONDS", 60),
		ModelGovernanceRolloutMaxErrorRate:    getenvFloat("MODEL_GOVERNANCE_ROLLOUT_MAX_ERROR_RATE", 0.02),
		ModelGovernanceRolloutMaxP95MS:        getenvInt("MODEL_GOVERNANCE_ROLLOUT_MAX_P95_MS", 1200),
		ModelGovernanceRolloutMaxFallbackRate: getenvFloat("MODEL_GOVERNANCE_ROLLOUT_MAX_FALLBACK_RATE", 0.15),
		ModelGovernanceMinSampleCount:         getenvInt("MODEL_GOVERNANCE_MIN_SAMPLE_COUNT", 200),
		ModelContextWindows:                   getenvJSONIntMap("MODEL_CONTEXT_WINDOWS", nil),
		ContextSafetyRatio:                    getenvFloat("CONTEXT_SAFETY_RATIO", 0.8),
		ContextAutoCompressEnabled:            getenvBool("CONTEXT_AUTO_COMPRESS_ENABLED", true),
		ContextCompressModel:                  getenv("CONTEXT_COMPRESS_MODEL", "virtual-long-1m"),
		ContextCompressMaxRecentTurns:         getenvInt("CONTEXT_COMPRESS_MAX_RECENT_TURNS", 8),
		FallbackMaxDepth:                      getenvInt("FALLBACK_MAX_DEPTH", 3),
		FallbackMinScoreRatio:                 getenvFloat("FALLBACK_MIN_SCORE_RATIO", 0.5),
		AuditRetentionDays:                    getenvInt("AUDIT_RETENTION_DAYS", 90),
		DefaultAPIKeyRPM:                      getenvInt("DEFAULT_API_KEY_RPM", 60),
		MaskEnabled:                           getenvBool("MASK_ENABLED", false),
		GitHubClientID:                        getenv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret:                    getenv("GITHUB_CLIENT_SECRET", ""),
		SMTPHost:                              getenv("SMTP_HOST", ""),
		SMTPPort:                              getenvInt("SMTP_PORT", 587),
		SMTPUser:                              getenv("SMTP_USER", ""),
		SMTPPassword:                          getenv("SMTP_PASSWORD", ""),
		SMTPFrom:                              getenv("SMTP_FROM", ""),
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

func getenvJSONIntMap(key string, fallback map[string]int) map[string]int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var out map[string]int
	if err := json.Unmarshal([]byte(value), &out); err != nil || len(out) == 0 {
		return fallback
	}
	return out
}
