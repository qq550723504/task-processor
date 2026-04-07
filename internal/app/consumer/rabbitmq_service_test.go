package consumer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"

	"github.com/sirupsen/logrus"
)

type noopRabbitHandler struct{}

func (noopRabbitHandler) HandleMessage(_ context.Context, _ *rabbitmq.Message) error {
	return nil
}

func TestRabbitMQServiceFilterQueueConfigsByRole(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		expected []string
	}{
		{
			name:     "task role keeps only task queues",
			role:     config.NodeRoleTask,
			expected: []string{"amazon.tasks.store.*", "shein.tasks.store.*"},
		},
		{
			name:     "crawler role keeps only crawler queues",
			role:     config.NodeRoleCrawler,
			expected: []string{"amazon.crawler", "1688.crawler"},
		},
		{
			name:     "hybrid role keeps all queues",
			role:     config.NodeRoleHybrid,
			expected: []string{"amazon.tasks.store.*", "amazon.crawler", "1688.crawler", "shein.tasks.store.*"},
		},
	}

	source := []config.QueueConfig{
		{Name: "amazon.tasks.store.*"},
		{Name: "amazon.crawler"},
		{Name: "1688.crawler"},
		{Name: "shein.tasks.store.*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewRabbitMQService(&config.RabbitMQConfig{
				URL: "amqp://guest:guest@localhost:5672/",
				Node: config.NodeConfig{
					Role: tt.role,
				},
			}, logrus.New())

			filtered := svc.filterQueueConfigsByRole(source)
			if len(filtered) != len(tt.expected) {
				t.Fatalf("expected %d queue configs, got %d", len(tt.expected), len(filtered))
			}

			for i, expected := range tt.expected {
				if filtered[i].Name != expected {
					t.Fatalf("expected queue %q at index %d, got %q", expected, i, filtered[i].Name)
				}
			}
		})
	}
}

func TestHTTPServerHealthWhenRabbitMQDisconnected(t *testing.T) {
	logger := logrus.New()
	cfg := &config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role:            config.NodeRoleTask,
			HealthCheckPort: 8081,
			MetricsPort:     8082,
		},
	}
	svc := NewRabbitMQService(cfg, logger)
	loadMonitor := rabbitmq.NewLoadMonitor(config.LoadMonitorConfig{}, logger)
	server := NewHTTPServerManager(cfg, loadMonitor, svc, nil, logger)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	server.handleHealth(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected /health to return 200 when RabbitMQ is disconnected, got %d", recorder.Code)
	}
}

func TestHTTPServerReadinessWhenRabbitMQDisconnected(t *testing.T) {
	logger := logrus.New()
	cfg := &config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role:            config.NodeRoleTask,
			HealthCheckPort: 8081,
			MetricsPort:     8082,
		},
	}
	svc := NewRabbitMQService(cfg, logger)
	loadMonitor := rabbitmq.NewLoadMonitor(config.LoadMonitorConfig{}, logger)
	server := NewHTTPServerManager(cfg, loadMonitor, svc, nil, logger)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	recorder := httptest.NewRecorder()

	server.handleReady(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected /ready to return 503 when RabbitMQ is disconnected, got %d", recorder.Code)
	}
}

func TestRabbitMQServiceReportsUnhealthyRequiredConsumers(t *testing.T) {
	logger := logrus.New()
	cfg := &config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role:            config.NodeRoleTask,
			HealthCheckPort: 8081,
			MetricsPort:     8082,
		},
	}
	svc := NewRabbitMQService(cfg, logger)

	svc.GetConsumer().RegisterHandler("shein.tasks", noopRabbitHandler{})
	svc.GetConsumer().RegisterHandler("amazon.tasks", noopRabbitHandler{})

	if svc.HasHealthyRequiredConsumers() {
		t.Fatal("expected registered queues without running consumers to be unhealthy")
	}

	svc.GetConsumer().GetStateManager("shein.tasks").SetState(rabbitmq.ConsumerStateRunning, "shein.tasks")
	svc.GetConsumer().GetStateManager("amazon.tasks").SetError(errors.New("worker stopped"), "amazon.tasks")

	unhealthy := svc.GetUnhealthyRequiredQueues()
	if len(unhealthy) != 1 || unhealthy[0] != "amazon.tasks" {
		t.Fatalf("unexpected unhealthy queues: %#v", unhealthy)
	}
}

func TestStartupRetryDelayCapsAtThirtySeconds(t *testing.T) {
	tests := []struct {
		name     string
		attempt  int
		base     time.Duration
		expected time.Duration
	}{
		{name: "first attempt uses base delay", attempt: 0, base: 5 * time.Second, expected: 5 * time.Second},
		{name: "delay grows exponentially", attempt: 2, base: 5 * time.Second, expected: 20 * time.Second},
		{name: "delay is capped", attempt: 5, base: 5 * time.Second, expected: 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := startupRetryDelay(tt.attempt, tt.base)
			if got != tt.expected {
				t.Fatalf("expected delay %v, got %v", tt.expected, got)
			}
		})
	}
}
