package rabbitmq

import (
	"context"
	"errors"
	"testing"
	"time"

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

func TestStopClearsConsumerRegistry(t *testing.T) {
	logger := logrus.New()
	mc := NewMessageConsumer(nil, ConsumerConfig{PrefetchCount: 1}, logger)
	mc.consumers["shein.tasks"] = &QueueConsumer{
		queueName: "shein.tasks",
		cancel:    func() {},
	}
	mc.stateManager["shein.tasks"] = NewConsumerStateManager()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := mc.Stop(ctx); err != nil {
		t.Fatalf("expected stop to succeed, got %v", err)
	}
	if len(mc.consumers) != 0 {
		t.Fatalf("expected consumers to be cleared after stop, got %d", len(mc.consumers))
	}
}

func TestNewMessageConsumerCreatesGlobalConcurrencyLimiter(t *testing.T) {
	logger := logrus.New()
	mc := NewMessageConsumer(nil, ConsumerConfig{PrefetchCount: 1, MaxConcurrency: 4}, logger)

	if mc.workTokens == nil {
		t.Fatal("expected work token bucket to be initialized")
	}
	if cap(mc.workTokens) != 4 {
		t.Fatalf("expected work token bucket capacity 4, got %d", cap(mc.workTokens))
	}
}

func TestCreateQueueConsumerSharesGlobalConcurrencyLimiter(t *testing.T) {
	logger := logrus.New()
	mc := NewMessageConsumer(nil, ConsumerConfig{PrefetchCount: 1, MaxConcurrency: 3}, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mc.ctx = ctx
	mc.stateManager["shein.tasks.store.181"] = NewConsumerStateManager()
	queueConfig := &QueueConfig{Name: "shein.tasks.store.181", Prefetch: 1, Priority: 8}

	consumer := mc.createQueueConsumer("shein.tasks.store.181", "tag", noopHandler{}, nil, nil, queueConfig)
	if consumer.workTokens != mc.workTokens {
		t.Fatal("expected queue consumer to share message consumer work token bucket")
	}
}

func TestRequiredConsumerHealthUsesBrokerConsumerCount(t *testing.T) {
	logger := logrus.New()
	mc := NewMessageConsumer(nil, ConsumerConfig{PrefetchCount: 1}, logger)
	mc.RegisterHandler("shein.tasks.store.976", noopHandler{})
	mc.GetStateManager("shein.tasks.store.976").SetState(ConsumerStateRunning, "shein.tasks.store.976")
	mc.SetBrokerConsumerInspector(fakeBrokerConsumerInspector{
		counts: map[string]int{"shein.tasks.store.976": 0},
	})

	if mc.HasHealthyRequiredConsumers() {
		t.Fatal("expected broker consumer count 0 to make required consumer unhealthy")
	}
	unhealthy := mc.GetUnhealthyRequiredQueues()
	if len(unhealthy) != 1 || unhealthy[0] != "shein.tasks.store.976" {
		t.Fatalf("unhealthy queues = %#v, want shein.tasks.store.976", unhealthy)
	}
}

type fakeBrokerConsumerInspector struct {
	counts map[string]int
	err    error
}

func (f fakeBrokerConsumerInspector) BrokerConsumerCount(queueName string) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.counts[queueName], nil
}
