package httpserver

import (
	"context"
	"log/slog"
	"time"

	"llm-gateway/gateway/internal/auth"
)

func (s *Server) recordAPIKeyUsage(keyID int64, userID int64, requestID, model, provider string,
	promptTokens, completionTokens, totalTokens int, estimatedCost float64, latencyMs int, success bool) {
	if s.apiKeyUsageStore == nil || keyID <= 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		event := auth.APIKeyUsageEvent{
			KeyID:            keyID,
			UserID:           userID,
			RequestID:        requestID,
			Model:            model,
			Provider:         provider,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
			EstimatedCost:    estimatedCost,
			LatencyMs:        latencyMs,
			Success:          success,
		}
		if err := s.apiKeyUsageStore.Insert(ctx, event); err != nil {
			slog.Warn("api key usage insert failed", "key_id", keyID, "err", err)
		}
	}()
}
