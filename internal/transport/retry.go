package transport

import (
	"context"
	"math/rand/v2"
	"time"
)

// RetryPolicy describes how failed requests are retried.
type RetryPolicy struct {
	// MaxRetries is the number of retries after the first attempt
	// (default 2 → at most 3 attempts total).
	MaxRetries int
	// BaseDelay is the first backoff delay (default 200ms).
	BaseDelay time.Duration
	// MaxDelay caps exponential growth (default 5s).
	MaxDelay time.Duration
	// Budget is the total time allowed for retries (default 15s).
	Budget time.Duration
	// RetryNonIdempotent allows retrying requests marked NoRetry.
	RetryNonIdempotent bool

	// sleeper abstracts time for tests.
	sleeper func(ctx context.Context, d time.Duration) error
}

// DefaultRetryPolicy is the spec-defined default: 3 attempts total,
// 200ms base, 2x multiplier, full jitter, 5s cap, 15s budget.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries: 2,
		BaseDelay:  200 * time.Millisecond,
		MaxDelay:   5 * time.Second,
		Budget:     15 * time.Second,
	}
}

// backoff computes the jittered delay for attempt n (0-based retry no).
// delay = min(cap, base * 2^attempt) * random(0.5..1.0)
func (p RetryPolicy) backoff(attempt int) time.Duration {
	d := p.BaseDelay
	for i := 0; i < attempt; i++ {
		d *= 2
		if d >= p.MaxDelay {
			d = p.MaxDelay
			break
		}
	}
	if d > p.MaxDelay {
		d = p.MaxDelay
	}
	return time.Duration(float64(d) * (0.5 + rand.Float64()*0.5))
}

func (p RetryPolicy) sleep(ctx context.Context, d time.Duration) error {
	if p.sleeper != nil {
		return p.sleeper(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
