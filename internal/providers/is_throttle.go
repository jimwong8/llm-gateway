package providers

import "strings"

// isThrottleError: 429/限流类错误判定 (streamretry registry依赖).
func isThrottleError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "rpm exhausted") ||
		strings.Contains(msg, "tpm exhausted") ||
		strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "throttl")
}
