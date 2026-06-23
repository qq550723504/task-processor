package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
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

type stubDiscardableError struct{}

func (e stubDiscardableError) Error() string { return "discard" }
func (e stubDiscardableError) ShouldDiscard() bool {
	return true
}

type stubAcknowledger struct {
	acked    bool
	ackCount int
	nacked   bool
	rejected bool
	requeue  bool
}

func (a *stubAcknowledger) Ack(_ uint64, _ bool) error {
	a.acked = true
	a.ackCount++
	return nil
}

func (a *stubAcknowledger) Nack(_ uint64, _ bool, requeue bool) error {
	a.nacked = true
	a.requeue = requeue
	return nil
}

func (a *stubAcknowledger) Reject(_ uint64, requeue bool) error {
	a.rejected = true
	a.requeue = requeue
	return nil
}

func TestQueueConsumerHandleProcessError_DiscardableMessageIsAckedWithoutCollection(t *testing.T) {
	ack := &stubAcknowledger{}
	qc := &QueueConsumer{
		queueName:      "shein.tasks.store.838",
		logger:         logDiscardLogger(),
		stateManager:   NewConsumerStateManager(),
		errorCollector: NewErrorCollector(10),
	}

	delivery := amqp.Delivery{
		Acknowledger: ack,
		DeliveryTag:  1,
		MessageId:    "msg-discard-1",
	}
	msg := &Message{ID: "msg-discard-1"}

	qc.handleProcessError(delivery, msg, stubDiscardableError{})

	if !ack.acked {
		t.Fatal("expected discardable message to be acked")
	}
	if ack.nacked || ack.rejected {
		t.Fatal("did not expect discardable message to be nacked or rejected")
	}
	if got := len(qc.errorCollector.GetErrors()); got != 0 {
		t.Fatalf("expected discardable message not to be collected as error, got %d", got)
	}
	state := qc.stateManager.GetStateInfo()
	if state.SuccessCount != 1 || state.FailureCount != 0 {
		t.Fatalf("unexpected state counts: success=%d failure=%d", state.SuccessCount, state.FailureCount)
	}
}

type earlyAckHandler struct {
	err error
}

type nonRetryableTestError struct {
	message string
}

func (e nonRetryableTestError) Error() string { return e.message }
func (e nonRetryableTestError) IsRetryable() bool {
	return false
}

func (h *earlyAckHandler) HandleMessage(context.Context, *Message) error {
	return nonRetryableTestError{message: "legacy handler should not be called"}
}

func (h *earlyAckHandler) HandleMessageWithAck(_ context.Context, _ *Message, ack func() error) error {
	if err := ack(); err != nil {
		return err
	}
	return h.err
}

func TestQueueConsumerProcessMessage_DoesNotNackPostAckFailure(t *testing.T) {
	ack := &stubAcknowledger{}
	qc := &QueueConsumer{
		queueName:      "shein.tasks.store.976",
		handler:        &earlyAckHandler{err: errors.New("publish failed after claim")},
		logger:         logDiscardLogger(),
		stateManager:   NewConsumerStateManager(),
		errorCollector: NewErrorCollector(10),
	}

	qc.processMessage(amqp.Delivery{
		Acknowledger: ack,
		DeliveryTag:  1,
		MessageId:    "msg-early-ack-failure",
		Type:         "task",
		Body:         []byte(`{"taskId":7812001}`),
	})

	if ack.ackCount != 1 {
		t.Fatalf("expected one early ack, got %d", ack.ackCount)
	}
	if ack.nacked || ack.rejected {
		t.Fatal("post-ack processing failure must not nack or reject the RabbitMQ delivery")
	}
	state := qc.stateManager.GetStateInfo()
	if state.SuccessCount != 0 || state.FailureCount != 1 {
		t.Fatalf("unexpected state counts: success=%d failure=%d", state.SuccessCount, state.FailureCount)
	}
}

func TestQueueConsumerProcessMessage_DoesNotDoubleAckAfterEarlyAckSuccess(t *testing.T) {
	ack := &stubAcknowledger{}
	qc := &QueueConsumer{
		queueName:      "shein.tasks.store.976",
		handler:        &earlyAckHandler{},
		logger:         logDiscardLogger(),
		stateManager:   NewConsumerStateManager(),
		errorCollector: NewErrorCollector(10),
	}

	qc.processMessage(amqp.Delivery{
		Acknowledger: ack,
		DeliveryTag:  1,
		MessageId:    "msg-early-ack-success",
		Type:         "task",
		Body:         []byte(`{"taskId":7812002}`),
	})

	if ack.ackCount != 1 {
		t.Fatalf("expected one ack, got %d", ack.ackCount)
	}
	if ack.nacked || ack.rejected {
		t.Fatal("successful early-acked delivery must not be nacked or rejected")
	}
	state := qc.stateManager.GetStateInfo()
	if state.SuccessCount != 1 || state.FailureCount != 0 {
		t.Fatalf("unexpected state counts: success=%d failure=%d", state.SuccessCount, state.FailureCount)
	}
}

func logDiscardLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	return logger
}
