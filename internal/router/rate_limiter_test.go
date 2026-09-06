package router

import (
	"context"
	"testing"
	"time"
)

// TestRetryAfterExtraction verifies that Retry-After header is extracted from errors.
func TestRetryAfterExtraction(t *testing.T) {
	err := &upstreamHTTPErrorWithHeaders{
		statusCode: 429,
		message:    "rate limited",
		headers:    map[string]string{"Retry-After": "30"},
	}

	delay := extractRetryAfter(err)
	expected := 32 * time.Second // 30 + 2 buffer
	if delay != expected {
		t.Fatalf("expected delay %v, got %v", expected, delay)
	}
}

// TestRetryAfterExtractionNoHeader verifies fallback when no Retry-After header.
func TestRetryAfterExtractionNoHeader(t *testing.T) {
	err := &upstreamHTTPErrorWithHeaders{
		statusCode: 429,
		message:    "rate limited",
		headers:    map[string]string{},
	}

	delay := extractRetryAfter(err)
	if delay != 0 {
		t.Fatalf("expected 0 delay, got %v", delay)
	}
}

// TestIsRateLimitError verifies 429 detection.
func TestIsRateLimitError(t *testing.T) {
	err := &upstreamHTTPErrorWithHeaders{
		statusCode: 429,
		message:    "rate limited",
	}

	if !isRateLimitError(err) {
		t.Fatal("expected 429 to be detected as rate limit error")
	}
}

// TestModelCooldownIsolation verifies model cooldown only affects the specific model.
func TestModelCooldownIsolation(t *testing.T) {
	mc := NewModelCooldown()

	// Model A 429 → set cooldown
	mc.SetCooldown("model-a", 7200*time.Second)

	// Model A should be cooling
	if !mc.IsCooling("model-a") {
		t.Fatal("model-a should be on cooldown")
	}

	// Model B should NOT be cooling
	if mc.IsCooling("model-b") {
		t.Fatal("model-b should NOT be on cooldown")
	}

	// Clear model A
	mc.Clear("model-a")
	if mc.IsCooling("model-a") {
		t.Fatal("model-a should no longer be on cooldown")
	}
}

// TestModelCircuitBreaker verifies circuit opens after 3 failures.
func TestModelCircuitBreaker(t *testing.T) {
	cb := NewModelCircuitBreaker()

	// 2 failures → circuit still closed
	cb.RecordFailure("model-x")
	cb.RecordFailure("model-x")
	if cb.IsCircuitOpen("model-x") {
		t.Fatal("circuit should not be open after 2 failures")
	}

	// 3rd failure → circuit opens
	cb.RecordFailure("model-x")
	if !cb.IsCircuitOpen("model-x") {
		t.Fatal("circuit should be open after 3 failures")
	}

	// Success → circuit closes
	cb.RecordSuccess("model-x")
	if cb.IsCircuitOpen("model-x") {
		t.Fatal("circuit should be closed after success")
	}
}

// TestAdaptiveRateLimiterDecrement verifies RPM cap decreases on 429.
func TestAdaptiveRateLimiterDecrement(t *testing.T) {
	rl := NewAdaptiveRateLimiter(40)

	// Send 20 requests (half of limit)
	for i := 0; i < 20; i++ {
		rl.Record()
	}

	// Should still be able to send (under 40 limit)
	if !rl.CanSend() {
		t.Fatal("should be able to send after 20 requests")
	}

	// Simulate 429 at 20 requests → cap to 20
	rl.OnRateLimited()

	// RPM should have decreased to 20
	if rl.CurrentRPM() != 20 {
		t.Fatalf("RPM should have decreased to 20, got %d", rl.CurrentRPM())
	}

	// Send 20 more requests to fill the new cap
	for i := 0; i < 20; i++ {
		rl.Record()
	}

	// Should not be able to send
	if rl.CanSend() {
		t.Fatal("should not be able to send after filling new cap")
	}

	// Another 429 → cap decreases by 1
	rl.OnRateLimited()
	if rl.CurrentRPM() != 19 {
		t.Fatalf("RPM should have decreased to 19, got %d", rl.CurrentRPM())
	}
}

// TestKeyRPMTrackerLearnFromRetryAfter verifies learning from Retry-After header.
func TestKeyRPMTrackerLearnFromRetryAfter(t *testing.T) {
	kt := NewKeyRPMTracker()

	// Retry-After: 30s → implies 2 RPM
	kt.LearnFromRetryAfter(30)

	// Should be able to send 2 requests
	kt.Record()
	kt.Record()

	// 3rd should be blocked
	if kt.CanSend() {
		t.Fatal("should not be able to send 3rd request after learning 2 RPM")
	}
}

// TestAccountWideDetector verifies aggregate rate limit detection.
func TestAccountWideDetector(t *testing.T) {
	ad := NewAccountWideDetector()

	// 2 different keys 429 → not aggregate yet
	ad.NoteRateLimited("key-1")
	ad.NoteRateLimited("key-2")
	if ad.IsPoolThrottled() {
		t.Fatal("should not be pool-throttled with only 2 keys")
	}

	// 3rd different key → aggregate detection
	ad.NoteRateLimited("key-3")
	if !ad.IsPoolThrottled() {
		t.Fatal("should be pool-throttled after 3 distinct keys")
	}
}

// TestExecuteDecisionWithCooldown verifies cooldown-aware execution.
func TestExecuteDecisionWithCooldown(t *testing.T) {
	mc := NewModelCooldown()
	cb := NewModelCircuitBreaker()

	// Set model-a on cooldown
	mc.SetCooldown("model-a", 7200*time.Second)

	decision := Decision{
		Model:    "model-a",
		Provider: "test-provider",
		FallbackChain: []FallbackRoute{
			{Model: "model-b", Provider: "test-provider", Reason: "fallback"},
		},
	}

	attemptedModels := []string{}
	mockFn := func(ctx context.Context, target RouteTarget, key ProviderKey) (string, error) {
		attemptedModels = append(attemptedModels, target.Model)
		return "success", nil
	}

	result, _, err := ExecuteDecisionWithCooldown(
		context.Background(),
		decision,
		RetryConfig{MaxAttempts: 3, BaseDelay: 100 * time.Millisecond},
		nil,
		mockFn,
		mc,
		cb,
	)

	if err != nil {
		t.Fatalf("expected success from fallback, got error: %v", err)
	}
	if result != "success" {
		t.Fatalf("expected 'success', got %v", result)
	}

	// model-a should have been skipped (on cooldown), model-b attempted
	if len(attemptedModels) != 1 || attemptedModels[0] != "model-b" {
		t.Fatalf("expected only model-b to be attempted, got %v", attemptedModels)
	}
}

// mock error type for testing
type upstreamHTTPErrorWithHeaders struct {
	statusCode int
	message    string
	headers    map[string]string
}

func (e *upstreamHTTPErrorWithHeaders) Error() string {
	return e.message
}

func (e *upstreamHTTPErrorWithHeaders) HTTPStatusCode() int {
	return e.statusCode
}

func (e *upstreamHTTPErrorWithHeaders) HTTPResponseHeaders() map[string]string {
	return e.headers
}
