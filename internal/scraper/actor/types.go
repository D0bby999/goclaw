package actor

import (
	"encoding/json"
	"time"
)

type ErrorCategory string

const (
	ErrNetwork    ErrorCategory = "network"
	ErrAuth       ErrorCategory = "auth"
	ErrRateLimit  ErrorCategory = "rate-limit"
	ErrParse      ErrorCategory = "parse"
	ErrValidation ErrorCategory = "validation"
	ErrUnknown    ErrorCategory = "unknown"
)

type Error struct {
	Message   string            `json:"message"`
	Category  ErrorCategory     `json:"category"`
	Retryable bool              `json:"retryable"`
	Timestamp time.Time         `json:"timestamp"`
	Context   map[string]string `json:"context,omitempty"`
}

type RunStats struct {
	StartedAt      time.Time `json:"started_at"`
	CompletedAt    time.Time `json:"completed_at"`
	DurationMs     int64     `json:"duration_ms"`
	ItemsScraped   int       `json:"items_scraped"`
	RequestsTotal  int       `json:"requests_total"`
	RequestsFailed int       `json:"requests_failed"`
	Retries        int       `json:"retries"`
}

type Run struct {
	Status string            `json:"status"` // succeeded, failed, timed-out
	Items  []json.RawMessage `json:"items"`
	Errors []Error           `json:"errors"`
	Stats  RunStats          `json:"stats"`
}

type Config struct {
	MaxRetries    int
	RequestDelay  time.Duration
	Timeout       time.Duration
	SessionPoolOn bool
}

func DefaultConfig() Config {
	return Config{
		MaxRetries:    3,
		RequestDelay:  2 * time.Second,
		Timeout:       5 * time.Minute,
		SessionPoolOn: true,
	}
}
