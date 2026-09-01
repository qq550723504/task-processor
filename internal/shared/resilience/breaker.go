package resilience

import (
	"context"
	"time"

	"github.com/sony/gobreaker"
)

var (
	ErrCircuitOpen     = gobreaker.ErrOpenState
	ErrTooManyRequests = gobreaker.ErrTooManyRequests
)

type CircuitState int

const (
	StateClosed CircuitState = iota
	StateHalfOpen
	StateOpen
)

type CircuitBreakerCounts struct {
	Requests             uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}

type CircuitBreakerConfig struct {
	Name             string
	MaxRequests      uint32
	Interval         time.Duration
	OpenTimeout      time.Duration
	ReadyToTripAfter uint32
	ReadyToTrip      func(CircuitBreakerCounts) bool
	OnStateChange    func(from, to CircuitState)
	IsSuccessful     func(error) bool
}

type CircuitBreaker struct {
	breaker *gobreaker.CircuitBreaker
}

func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	settings := gobreaker.Settings{
		Name:        config.Name,
		MaxRequests: config.MaxRequests,
		Interval:    config.Interval,
		Timeout:     config.OpenTimeout,
		IsSuccessful: func(err error) bool {
			if config.IsSuccessful != nil {
				return config.IsSuccessful(err)
			}
			return err == nil
		},
	}

	if config.ReadyToTrip != nil {
		settings.ReadyToTrip = func(counts gobreaker.Counts) bool {
			return config.ReadyToTrip(convertCounts(counts))
		}
	} else if config.ReadyToTripAfter > 0 {
		settings.ReadyToTrip = func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= config.ReadyToTripAfter
		}
	}

	if config.OnStateChange != nil {
		settings.OnStateChange = func(_ string, from gobreaker.State, to gobreaker.State) {
			config.OnStateChange(convertState(from), convertState(to))
		}
	}

	return &CircuitBreaker{
		breaker: gobreaker.NewCircuitBreaker(settings),
	}
}

func (cb *CircuitBreaker) Execute(ctx context.Context, operation func(context.Context) error) error {
	_, err := cb.breaker.Execute(func() (any, error) {
		return nil, operation(ctx)
	})
	return err
}

func (cb *CircuitBreaker) State() CircuitState {
	return convertState(cb.breaker.State())
}

func (cb *CircuitBreaker) Counts() CircuitBreakerCounts {
	return convertCounts(cb.breaker.Counts())
}

func convertCounts(counts gobreaker.Counts) CircuitBreakerCounts {
	return CircuitBreakerCounts{
		Requests:             counts.Requests,
		TotalSuccesses:       counts.TotalSuccesses,
		TotalFailures:        counts.TotalFailures,
		ConsecutiveSuccesses: counts.ConsecutiveSuccesses,
		ConsecutiveFailures:  counts.ConsecutiveFailures,
	}
}

func convertState(state gobreaker.State) CircuitState {
	switch state {
	case gobreaker.StateClosed:
		return StateClosed
	case gobreaker.StateHalfOpen:
		return StateHalfOpen
	case gobreaker.StateOpen:
		return StateOpen
	default:
		return StateOpen
	}
}
