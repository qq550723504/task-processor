package submission

import (
	"errors"
	"time"

	"task-processor/internal/infra/worker"
)

const (
	EnqueueRetryDelay    = 250 * time.Millisecond
	EnqueueRetryMaxDelay = 5 * time.Second
)

func RetryEnqueueSubmit(taskID string, maxWait time.Duration, submit func(string) error) error {
	if submit == nil {
		return errors.New("submit func is not configured")
	}

	deadline := time.Now().Add(maxWait)
	delay := EnqueueRetryDelay
	for {
		err := submit(taskID)
		if err == nil {
			return nil
		}
		if !errors.Is(err, worker.ErrQueueFull) {
			return err
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(delay)
		delay = NextEnqueueRetryDelay(delay)
	}
}

func NextEnqueueRetryDelay(delay time.Duration) time.Duration {
	if delay <= 0 {
		return EnqueueRetryDelay
	}
	if delay >= EnqueueRetryMaxDelay {
		return EnqueueRetryMaxDelay
	}
	delay *= 2
	if delay > EnqueueRetryMaxDelay {
		return EnqueueRetryMaxDelay
	}
	return delay
}

func BoundedEnqueueRetryDelay(attempt int) time.Duration {
	if attempt <= 1 {
		return EnqueueRetryDelay
	}
	delay := EnqueueRetryDelay
	for i := 1; i < attempt; i++ {
		delay = NextEnqueueRetryDelay(delay)
		if delay >= EnqueueRetryMaxDelay {
			return EnqueueRetryMaxDelay
		}
	}
	return delay
}
