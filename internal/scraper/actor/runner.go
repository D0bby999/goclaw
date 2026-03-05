package actor

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// RunActor executes the full actor lifecycle and returns a Run result.
func RunActor(ctx context.Context, a Actor, cfg Config) Run {
	runCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	stats := RunStats{StartedAt: time.Now()}
	run := Run{
		Items:  []json.RawMessage{},
		Errors: []Error{},
	}

	if err := a.Initialize(runCtx); err != nil {
		stats.CompletedAt = time.Now()
		stats.DurationMs = time.Since(stats.StartedAt).Milliseconds()
		run.Stats = stats
		run.Errors = append(run.Errors, NewError(err.Error(), ClassifyError(err)))
		run.Status = "failed"
		return run
	}

	defer a.Cleanup()

	items, err := a.Execute(runCtx, &stats)

	stats.CompletedAt = time.Now()
	stats.DurationMs = time.Since(stats.StartedAt).Milliseconds()

	if items != nil {
		run.Items = items
		stats.ItemsScraped = len(items)
	}

	// Merge per-URL errors if the actor implements ErrorReporter.
	if reporter, ok := a.(ErrorReporter); ok {
		run.Errors = append(run.Errors, reporter.CollectedErrors()...)
	}

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		run.Status = "timed-out"
		run.Errors = append(run.Errors, NewError(err.Error(), ErrNetwork))
	case err != nil:
		run.Status = "failed"
		run.Errors = append(run.Errors, NewError(err.Error(), ClassifyError(err)))
	default:
		run.Status = "succeeded"
	}

	run.Stats = stats
	return run
}
