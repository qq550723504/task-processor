package resilience

import (
	"context"

	"golang.org/x/time/rate"
)

// RateLimiter wraps golang.org/x/time/rate with the shape used by callers.
type RateLimiter struct {
	limiter *rate.Limiter
}

// NewRateLimiter creates a token-bucket rate limiter.
func NewRateLimiter(requestsPerSecond float64, burst int) *RateLimiter {
	if burst < 1 {
		burst = 1
	}

	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(requestsPerSecond), burst),
	}
}

// Wait blocks until a token is available or the context is canceled.
func (l *RateLimiter) Wait(ctx context.Context) error {
	return l.limiter.Wait(ctx)
}

// Allow reports whether a token can be consumed immediately.
func (l *RateLimiter) Allow() bool {
	return l.limiter.Allow()
}
