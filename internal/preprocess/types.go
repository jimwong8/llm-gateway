package preprocess

import "llm-gateway/gateway/internal/providers"

type NormalizeMeta struct {
	Applied         bool   `json:"applied"`
	CanonicalHash   string `json:"canonical_hash,omitempty"`
	TemplateVersion string `json:"template_version,omitempty"`
}

type SummaryMeta struct {
	Applied               bool    `json:"applied"`
	OriginalTokenEstimate int     `json:"original_token_estimate,omitempty"`
	ReducedTokenEstimate  int     `json:"reduced_token_estimate,omitempty"`
	CompressionRatio      float64 `json:"compression_ratio,omitempty"`
	SummaryText           string  `json:"summary_text,omitempty"`
}

type ClassificationMeta struct {
	Applied    bool    `json:"applied"`
	TaskHint   string  `json:"task_hint,omitempty"`
	Complexity string  `json:"complexity,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

type Result struct {
	OriginalRequest  providers.ChatCompletionRequest `json:"original_request"`
	ProcessedRequest providers.ChatCompletionRequest `json:"processed_request"`

	Normalize      NormalizeMeta       `json:"normalize"`
	Summary        SummaryMeta         `json:"summary"`
	Classification ClassificationMeta `json:"classification"`
}
