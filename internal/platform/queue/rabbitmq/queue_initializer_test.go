package rabbitmq

import (
	"errors"
	"strings"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

type fakeQueueInitializerClient struct {
	exchanges   []ExchangeConfig
	queues      []QueueDeclareConfig
	deletes     []string
	bindings    []BindingConfig
	exchangeErr error
	queueErrs   []error
	deleteErr   error
	bindErr     error
}

func (f *fakeQueueInitializerClient) DeclareExchange(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	f.exchanges = append(f.exchanges, ExchangeConfig{Name: name, Type: kind, Durable: durable, AutoDelete: autoDelete, Internal: internal, NoWait: noWait, Args: args})
	return f.exchangeErr
}

func (f *fakeQueueInitializerClient) DeclareQueue(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) error {
	f.queues = append(f.queues, QueueDeclareConfig{Name: name, Durable: durable, AutoDelete: autoDelete, Exclusive: exclusive, NoWait: noWait, Args: args})
	if len(f.queueErrs) == 0 {
		return nil
	}
	err := f.queueErrs[0]
	f.queueErrs = f.queueErrs[1:]
	return err
}

func (f *fakeQueueInitializerClient) DeleteQueue(name string, _, _, _ bool) error {
	f.deletes = append(f.deletes, name)
	return f.deleteErr
}

func (f *fakeQueueInitializerClient) BindQueue(queueName, routingKey, exchangeName string, noWait bool, args amqp.Table) error {
	f.bindings = append(f.bindings, BindingConfig{QueueName: queueName, RoutingKey: routingKey, ExchangeName: exchangeName, NoWait: noWait, Args: args})
	return f.bindErr
}

func TestQueueInitializerInitializeAllPreservesTopology(t *testing.T) {
	fake := &fakeQueueInitializerClient{}
	initializer := newQueueInitializer(fake, logrus.New())

	if err := initializer.InitializeAll(); err != nil {
		t.Fatalf("InitializeAll() error = %v", err)
	}
	if len(fake.exchanges) != 4 {
		t.Fatalf("exchange declarations = %d, want 4", len(fake.exchanges))
	}
	if len(fake.queues) != 5 {
		t.Fatalf("queue declarations = %d, want 5", len(fake.queues))
	}
	if len(fake.bindings) != 3 {
		t.Fatalf("bindings = %d, want 3", len(fake.bindings))
	}
	if fake.exchanges[0].Name != "tasks.exchange" || fake.exchanges[0].Type != "topic" || !fake.exchanges[0].Durable {
		t.Fatalf("first exchange = %#v, want durable tasks.exchange topic", fake.exchanges[0])
	}
	if !containsQueueDeclaration(fake.queues, "tasks.dlq") || !containsBinding(fake.bindings, "tasks.dlq", "failed", "tasks.dlx") {
		t.Fatalf("topology missing dead-letter route: queues=%#v bindings=%#v", fake.queues, fake.bindings)
	}
}

func TestQueueInitializerWrapsStageErrors(t *testing.T) {
	sentinel := errors.New("broker rejected declaration")
	tests := []struct {
		name       string
		configure  func(*fakeQueueInitializerClient)
		wantPrefix string
	}{
		{name: "exchange", configure: func(f *fakeQueueInitializerClient) { f.exchangeErr = sentinel }, wantPrefix: "初始化交换机失败"},
		{name: "queue", configure: func(f *fakeQueueInitializerClient) { f.queueErrs = []error{sentinel} }, wantPrefix: "初始化队列失败"},
		{name: "binding", configure: func(f *fakeQueueInitializerClient) { f.bindErr = sentinel }, wantPrefix: "初始化绑定失败"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeQueueInitializerClient{}
			tc.configure(fake)
			err := newQueueInitializer(fake, logrus.New()).InitializeAll()
			if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), tc.wantPrefix) {
				t.Fatalf("InitializeAll() error = %v, want wrapped sentinel containing %q", err, tc.wantPrefix)
			}
		})
	}
}

func TestQueueInitializerPreconditionFailureDeletesAndRedeclares(t *testing.T) {
	fake := &fakeQueueInitializerClient{
		queueErrs: []error{&amqp.Error{Code: 406, Reason: "arguments differ"}, nil},
		deleteErr: errors.New("delete raced with broker cleanup"),
	}
	initializer := newQueueInitializer(fake, logrus.New())
	cfg := QueueDeclareConfig{Name: "tasks.repair", Durable: true, Args: amqp.Table{"x-max-priority": int32(10)}}

	if err := initializer.declareQueueWithRetry(cfg); err != nil {
		t.Fatalf("declareQueueWithRetry() error = %v", err)
	}
	if len(fake.queues) != 2 || len(fake.deletes) != 1 || fake.deletes[0] != cfg.Name {
		t.Fatalf("queue calls = %d deletes = %#v, want declare/delete/redeclare", len(fake.queues), fake.deletes)
	}
}

func TestQueueInitializerDynamicQueuePaths(t *testing.T) {
	t.Run("region crawler", func(t *testing.T) {
		fake := &fakeQueueInitializerClient{}
		if err := newQueueInitializer(fake, logrus.New()).InitializeRegionCrawlerQueues([]string{"US"}); err != nil {
			t.Fatal(err)
		}
		if !containsQueueDeclaration(fake.queues, "amazon.crawler.us") || !containsQueueDeclaration(fake.queues, "1688.crawler.us") {
			t.Fatalf("region queues = %#v", fake.queues)
		}
	})

	t.Run("store", func(t *testing.T) {
		fake := &fakeQueueInitializerClient{}
		if err := newQueueInitializer(fake, logrus.New()).InitializeStoreQueues("temu", []int64{42}); err != nil {
			t.Fatal(err)
		}
		if !containsQueueDeclaration(fake.queues, "temu.tasks.store.42") || !containsBinding(fake.bindings, "temu.tasks.store.42", "temu.tasks.store.42", "tasks.exchange") {
			t.Fatalf("store topology = queues:%#v bindings:%#v", fake.queues, fake.bindings)
		}
	})

	t.Run("platform", func(t *testing.T) {
		fake := &fakeQueueInitializerClient{}
		if err := newQueueInitializer(fake, logrus.New()).InitializePlatformQueues([]string{"temu"}); err != nil {
			t.Fatal(err)
		}
		if !containsQueueDeclaration(fake.queues, "temu.tasks") || !containsBinding(fake.bindings, "temu.tasks", "temu.tasks.store.*", "tasks.exchange") {
			t.Fatalf("platform topology = queues:%#v bindings:%#v", fake.queues, fake.bindings)
		}
	})
}

func containsQueueDeclaration(calls []QueueDeclareConfig, name string) bool {
	for _, call := range calls {
		if call.Name == name {
			return true
		}
	}
	return false
}

func containsBinding(calls []BindingConfig, queue, routingKey, exchange string) bool {
	for _, call := range calls {
		if call.QueueName == queue && call.RoutingKey == routingKey && call.ExchangeName == exchange {
			return true
		}
	}
	return false
}
