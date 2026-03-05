package stealth

import (
	"context"
	"math/rand/v2"
	"time"
)

// Jitter returns a duration randomly scaled between 0.5x and 1.5x of base.
func Jitter(base time.Duration) time.Duration {
	// scale in [0.5, 1.5]
	scale := 0.5 + rand.Float64()
	return time.Duration(float64(base) * scale)
}

// WaitWithJitter sleeps for a jittered duration based on base.
// Returns ctx.Err() if the context is cancelled before the sleep completes.
func WaitWithJitter(ctx context.Context, base time.Duration) error {
	d := Jitter(base)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
