package rabbitmq

import (
	"fmt"
	"testing"
)

type stubRetryableError struct {
	retryable bool
}

func (e stubRetryableError) Error() string {
	if e.retryable {
		return "retryable"
	}
	return "non-retryable"
}

func (e stubRetryableError) IsRetryable() bool {
	return e.retryable
}

func TestQueueConsumerShouldRetryHonorsRetryableError(t *testing.T) {
	qc := &QueueConsumer{}

	msg := &Message{RetryCount: 1, MaxRetries: 3}
	if !qc.shouldRetry(msg, stubRetryableError{retryable: true}) {
		t.Fatal("expected retryable error to be retried")
	}
}

func TestQueueConsumerShouldRetryRejectsNonRetryableError(t *testing.T) {
	qc := &QueueConsumer{}

	msg := &Message{RetryCount: 1, MaxRetries: 3}
	if qc.shouldRetry(msg, stubRetryableError{retryable: false}) {
		t.Fatal("expected non-retryable error to skip requeue")
	}
}

func TestQueueConsumerShouldRetryChecksWrappedRetryableError(t *testing.T) {
	qc := &QueueConsumer{}

	msg := &Message{RetryCount: 1, MaxRetries: 3}
	err := fmt.Errorf("outer: %w", stubRetryableError{retryable: false})
	if qc.shouldRetry(msg, err) {
		t.Fatal("expected wrapped non-retryable error to skip requeue")
	}
}

func TestQueueConsumerShouldRetryStopsAtMaxRetries(t *testing.T) {
	qc := &QueueConsumer{}

	msg := &Message{RetryCount: 3, MaxRetries: 3}
	if qc.shouldRetry(msg, stubRetryableError{retryable: true}) {
		t.Fatal("expected retry to stop once max retries is reached")
	}
}
