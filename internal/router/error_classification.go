package router

import (
	"context"
	"errors"
	"strings"
)

type ErrorClass string

const (
	ErrorClassNone              ErrorClass = "none"
	ErrorClassRateLimit         ErrorClass = "rate_limit"
	ErrorClassRetryableUpstream ErrorClass = "retryable_upstream"
	ErrorClassClientCancelled   ErrorClass = "client_cancelled"
	ErrorClassAuth              ErrorClass = "auth"
	ErrorClassBadRequest        ErrorClass = "bad_request"
	ErrorClassUnknown           ErrorClass = "unknown"
)

type ProviderHTTPError struct {
	StatusCode int
	Message    string
}

func (e ProviderHTTPError) Error() string {
	if strings.TrimSpace(e.Message) == "" {
		return "provider http error"
	}
	return e.Message
}

type ClassifiedError struct {
	Class      ErrorClass
	StatusCode int
	Provider   string
	Retryable  bool
	RotateKey  bool
	Err        error
}

func ClassifyError(ctx context.Context, provider string, err error) ClassifiedError {
	if err == nil {
		return ClassifiedError{Class: ErrorClassNone, Provider: provider}
	}
	if errors.Is(err, context.Canceled) || ctx.Err() == context.Canceled {
		return ClassifiedError{Class: ErrorClassClientCancelled, Provider: provider, Err: err}
	}
	if errors.Is(err, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded {
		return ClassifiedError{Class: ErrorClassClientCancelled, Provider: provider, Err: err}
	}

	var httpErr ProviderHTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case 429:
			return ClassifiedError{Class: ErrorClassRateLimit, StatusCode: httpErr.StatusCode, Provider: provider, Retryable: true, RotateKey: true, Err: err}
		case 408, 502, 503, 504:
			return ClassifiedError{Class: ErrorClassRetryableUpstream, StatusCode: httpErr.StatusCode, Provider: provider, Retryable: true, Err: err}
		case 401, 403:
			return ClassifiedError{Class: ErrorClassAuth, StatusCode: httpErr.StatusCode, Provider: provider, Err: err}
		case 400:
			return ClassifiedError{Class: ErrorClassBadRequest, StatusCode: httpErr.StatusCode, Provider: provider, Err: err}
		default:
			return ClassifiedError{Class: ErrorClassUnknown, StatusCode: httpErr.StatusCode, Provider: provider, Err: err}
		}
	}

	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests") {
		return ClassifiedError{Class: ErrorClassRateLimit, Provider: provider, Retryable: true, RotateKey: true, Err: err}
	}
	return ClassifiedError{Class: ErrorClassUnknown, Provider: provider, Err: err}
}
