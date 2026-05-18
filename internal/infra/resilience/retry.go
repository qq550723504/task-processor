package resilience

import (
	"context"
	"errors"
	"time"

	"github.com/cenkalti/backoff/v5"
)

type RetryAttempt struct {
	Attempt   int
	Err       error
	NextDelay time.Duration
}

type RetryConfig struct {
	MaxAttempts         int
	InitialDelay        time.Duration
	MaxDelay            time.Duration
	Multiplier          float64
	RandomizationFactor float64
	IsRetryable         func(error) bool
	OnRetry             func(context.Context, RetryAttempt)
}

func Retry(ctx context.Context, config RetryConfig, operation func(context.Context) error) error {
	config = normalizeRetryConfig(config)

	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = config.InitialDelay
	bo.MaxInterval = config.MaxDelay
	bo.Multiplier = config.Multiplier
	bo.RandomizationFactor = config.RandomizationFactor
	bo.Reset()

	attempt := 0
	_, err := backoff.Retry(ctx, func() (struct{}, error) {
		attempt++

		err := operation(ctx)
		if err == nil {
			return struct{}{}, nil
		}
		if !config.IsRetryable(err) {
			return struct{}{}, backoff.Permanent(err)
		}

		return struct{}{}, err
	},
		backoff.WithBackOff(bo),
		backoff.WithMaxTries(uint(config.MaxAttempts)),
		backoff.WithNotify(func(err error, nextDelay time.Duration) {
			if config.OnRetry != nil {
				config.OnRetry(ctx, RetryAttempt{
					Attempt:   attempt,
					Err:       err,
					NextDelay: nextDelay,
				})
			}
		}),
	)
	if err == nil {
		return nil
	}

	var permanent *backoff.PermanentError
	if errors.As(err, &permanent) {
		return permanent.Err
	}

	return err
}

func normalizeRetryConfig(config RetryConfig) RetryConfig {
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 3
	}
	if config.InitialDelay <= 0 {
		config.InitialDelay = time.Second
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 30 * time.Second
	}
	if config.Multiplier <= 0 {
		config.Multiplier = 2
	}
	if config.RandomizationFactor < 0 {
		config.RandomizationFactor = 0
	}
	if config.IsRetryable == nil {
		config.IsRetryable = func(error) bool { return true }
	}

	return config
}
