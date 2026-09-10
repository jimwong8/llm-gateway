package providers

import (
	"context"
	"fmt"
)

// StreamTruncatedError represents an empty or truncated response from the provider.
// This can happen when NVIDIA NIM returns a 200 response with no content and no finish_reason.
type StreamTruncatedError struct {
	Model string
}

func (e *StreamTruncatedError) Error() string {
	return fmt.Sprintf("stream truncated: empty response from provider for model %s", e.Model)
}

// IsTruncationError checks if an error is a stream truncation error.
func IsTruncationError(err error) bool {
	_, ok := err.(*StreamTruncatedError)
	return ok
}

// ValidateResponse checks if a response is valid (has content or finish_reason).
// Returns a StreamTruncatedError if the response is empty/truncated.
func ValidateResponse(model string, resp *ChatCompletionResponse) error {
	if len(resp.Choices) == 0 {
		return &StreamTruncatedError{Model: model}
	}
	for _, choice := range resp.Choices {
		if choice.Message.Content != "" || choice.FinishReason != "" {
			return nil
		}
	}
	return &StreamTruncatedError{Model: model}
}

// TruncationRetryProvider wraps a Provider with automatic truncation detection and retry.
type TruncationRetryProvider struct {
	provider  Provider
	maxRetries int
}

func NewTruncationRetryProvider(provider Provider, maxRetries int) *TruncationRetryProvider {
	if maxRetries < 0 {
		maxRetries = 0
	}
	return &TruncationRetryProvider{
		provider:   provider,
		maxRetries: maxRetries,
	}
}

func (p *TruncationRetryProvider) Name() string {
	return p.provider.Name()
}

func (p *TruncationRetryProvider) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	var lastResp ChatCompletionResponse
	var lastErr error
	attempts := p.maxRetries + 1
	for attempt := 0; attempt < attempts; attempt++ {
		resp, err := p.provider.ChatCompletion(ctx, req)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return resp, err
			}
			continue
		}
		// Check for empty/truncated response
		if valErr := ValidateResponse(req.Model, &resp); valErr != nil {
			lastErr = valErr
			lastResp = resp
			if ctx.Err() != nil {
				return resp, valErr
			}
			continue
		}
		return resp, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("all attempts failed for provider %s", p.Name())
	}
	return lastResp, lastErr
}
