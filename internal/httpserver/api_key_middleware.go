package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type APIKeyRateLimiter struct {
	client     *redis.Client
	defaultRPM int
}

func NewAPIKeyRateLimiter(addr string, defaultRPM int) *APIKeyRateLimiter {
	return &APIKeyRateLimiter{
		client:     redis.NewClient(&redis.Options{Addr: addr}),
		defaultRPM: defaultRPM,
	}
}

func NewAPIKeyRateLimiterWithClient(client *redis.Client, defaultRPM int) *APIKeyRateLimiter {
	return &APIKeyRateLimiter{
		client:     client,
		defaultRPM: defaultRPM,
	}
}

func (l *APIKeyRateLimiter) Allow(ctx context.Context, keyID int64, rpmLimit int) (bool, int64, error) {
	if rpmLimit <= 0 {
		rpmLimit = l.defaultRPM
	}
	if rpmLimit <= 0 {
		return true, 0, nil
	}

	now := time.Now().UTC()
	minuteKey := now.Format("200601021504")
	redisKey := fmt.Sprintf("apikey:rpm:%d:%s", keyID, minuteKey)

	n, err := l.client.Incr(ctx, redisKey).Result()
	if err != nil {
		return false, 0, err
	}
	if n == 1 {
		_ = l.client.Expire(ctx, redisKey, time.Minute+5*time.Second).Err()
	}

	allowed := n <= int64(rpmLimit)
	return allowed, n, nil
}

func (s *Server) apiKeyRateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.apiKeyRateLimiter == nil {
			next(w, r)
			return
		}

		claims := getUserClaims(r.Context())
		if claims == nil {
			next(w, r)
			return
		}

		apiKeyID := getAPIKeyID(r.Context())
		if apiKeyID <= 0 {
			next(w, r)
			return
		}

		rpmLimit := s.defaultAPIKeyRPM
		if key, err := s.userStore.GetAPIKeyByID(r.Context(), apiKeyID); err == nil {
			if key.RPMILimit > 0 {
				rpmLimit = key.RPMILimit
			}
		}

		allowed, used, err := s.apiKeyRateLimiter.Allow(r.Context(), apiKeyID, rpmLimit)
		if err != nil {
			next(w, r)
			return
		}

		w.Header().Set("X-ApiKey-RateLimit-Limit", fmt.Sprintf("%d", rpmLimit))
		w.Header().Set("X-ApiKey-RateLimit-Used", fmt.Sprintf("%d", used))

		if !allowed {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error": map[string]any{
					"message": "API key rate limit exceeded",
					"type":    "rate_limit_error",
					"limit":   rpmLimit,
					"used":    used,
				},
			})
			return
		}

		next(w, r)
	}
}