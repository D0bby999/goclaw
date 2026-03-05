package actor

import (
	"context"
	"time"
)

// Delay blocks for d or until ctx is cancelled.
func Delay(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// HandleURLError classifies err, appends to errors slice, increments failed
// stats, and returns whether the error is retryable.
func HandleURLError(err error, url string, errors *[]Error, stats *RunStats) bool {
	cat := ClassifyError(err)
	e := NewError(err.Error(), cat)
	e.Context = map[string]string{"url": url}
	*errors = append(*errors, e)
	IncrementFailed(stats)
	return IsRetryable(cat)
}

// IncrementRequests adds one to the total request counter.
func IncrementRequests(stats *RunStats) {
	stats.RequestsTotal++
}

// IncrementFailed adds one to both total and failed request counters.
func IncrementFailed(stats *RunStats) {
	stats.RequestsTotal++
	stats.RequestsFailed++
}
