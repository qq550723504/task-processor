// Package resilience provides the infrastructure-facing resilience layer built
// on mature third-party libraries.
//
// This package is the preferred seam for new callers under internal/infra and
// infrastructure-adjacent application code that needs shared retry, rate limit,
// or circuit breaker behavior backed by:
//   - golang.org/x/time/rate
//   - github.com/sony/gobreaker
//   - github.com/cenkalti/backoff/v5
//
package resilience
