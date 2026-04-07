package rabbitmq

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
)

type noopHandler struct{}

func (noopHandler) HandleMessage(_ context.Context, _ *Message) error {
	return nil
}

func TestStartConsumersLockedTracksFailuresAndStates(t *testing.T) {
	logger := logrus.New()
	mc := NewMessageConsumer(nil, ConsumerConfig{PrefetchCount: 1}, logger)
	mc.handlers["shein.tasks"] = noopHandler{}
	mc.handlers["shein.tasks.store.1"] = noopHandler{}
	mc.stateManager["shein.tasks"] = NewConsumerStateManager()
	mc.stateManager["shein.tasks.store.1"] = NewConsumerStateManager()

	calls := 0
	failed := mc.startConsumersLockedWithStarter("重启", func(queueName string, _ MessageHandler) error {
		calls++
		if queueName == "shein.tasks" {
			return errors.New("channel unavailable")
		}
		return nil
	})

	if calls != 2 {
		t.Fatalf("expected 2 queues to be started, got %d", calls)
	}
	if len(failed) != 1 || failed[0] != "shein.tasks" {
		t.Fatalf("unexpected failed queues: %#v", failed)
	}
	if got := mc.stateManager["shein.tasks"].GetState(); got != ConsumerStateError {
		t.Fatalf("expected shein.tasks state=error, got %s", got.String())
	}
	if got := mc.stateManager["shein.tasks.store.1"].GetState(); got != ConsumerStateRunning {
		t.Fatalf("expected store queue state=running, got %s", got.String())
	}
}
