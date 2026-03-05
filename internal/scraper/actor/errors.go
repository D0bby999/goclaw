package actor

import (
	"strings"
	"time"
)

// ClassifyError maps an error message to an ErrorCategory via string matching.
func ClassifyError(err error) ErrorCategory {
	if err == nil {
		return ErrUnknown
	}
	msg := strings.ToLower(err.Error())

	switch {
	case containsAny(msg, "timeout", "deadline", "connection refused", "no such host"):
		return ErrNetwork
	case containsAny(msg, "401", "403", "unauthorized", "forbidden"):
		return ErrAuth
	case containsAny(msg, "429", "rate limit", "too many"):
		return ErrRateLimit
	case containsAny(msg, "json", "unmarshal", "parse", "unexpected"):
		return ErrParse
	case containsAny(msg, "invalid", "required", "missing"):
		return ErrValidation
	default:
		return ErrUnknown
	}
}

// IsRetryable returns true for categories that warrant a retry.
func IsRetryable(cat ErrorCategory) bool {
	return cat == ErrNetwork || cat == ErrRateLimit
}

// NewError constructs a classified Error with current timestamp.
func NewError(msg string, cat ErrorCategory) Error {
	return Error{
		Message:   msg,
		Category:  cat,
		Retryable: IsRetryable(cat),
		Timestamp: time.Now(),
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
