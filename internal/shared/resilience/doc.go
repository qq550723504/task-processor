// Package resilience provides runtime-neutral resilience primitives built on
// mature third-party libraries.
//
// The package is shared by domains and application code that need retry, rate
// limit, or circuit-breaker behavior without depending on runtime infrastructure.
// Implementations are backed by:
//   - golang.org/x/time/rate
//   - github.com/sony/gobreaker
//   - github.com/cenkalti/backoff/v5
package resilience
