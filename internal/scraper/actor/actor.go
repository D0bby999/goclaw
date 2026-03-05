package actor

import (
	"context"
	"encoding/json"
)

// Actor defines the lifecycle for any platform scraper.
type Actor interface {
	Initialize(ctx context.Context) error
	Execute(ctx context.Context, stats *RunStats) ([]json.RawMessage, error)
	Cleanup()
}

// ErrorReporter is an optional interface actors can implement to expose
// per-URL errors collected during execution.
type ErrorReporter interface {
	CollectedErrors() []Error
}
