package consumer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"

	"github.com/sirupsen/logrus"
)

func TestMetricsEndpointUsesPrometheusRegistry(t *testing.T) {
	manager := newTestHTTPServerManager()
	manager.loadMonitor.RecordTaskProcessed("shein.tasks", true, 3*time.Second)
	manager.loadMonitor.RecordTaskProcessed("shein.tasks", false, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recorder := httptest.NewRecorder()

	manager.metricsHandler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /metrics status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, "text/plain") {
		t.Fatalf("Content-Type = %q, want text/plain exposition", got)
	}

	body := recorder.Body.String()
	for _, pattern := range []string{
		`(?m)^# HELP go_goroutines `,
		`(?m)^go_goroutines \d+`,
		`(?m)^# HELP process_start_time_seconds `,
		`(?m)^process_start_time_seconds [0-9.e+-]+$`,
		`(?m)^tasks_processed_total 2$`,
		`(?m)^tasks_succeeded_total 1$`,
		`(?m)^tasks_failed_total 1$`,
	} {
		if !regexp.MustCompile(pattern).MatchString(body) {
			t.Fatalf("metrics body missing pattern %q:\n%s", pattern, body)
		}
	}
}

func TestStatsEndpointRemainsJSON(t *testing.T) {
	manager := newTestHTTPServerManager()
	manager.loadMonitor.RecordTaskProcessed("shein.tasks", true, time.Second)

	req := httptest.NewRequest(http.MethodGet, "/stats?format=compact", nil)
	recorder := httptest.NewRecorder()

	manager.handleStats(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /stats status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if _, ok := body["stats"]; !ok {
		t.Fatalf("stats response missing stats field: %v", body)
	}
	if _, ok := body["query"]; !ok {
		t.Fatalf("stats response missing query field: %v", body)
	}
}

func newTestHTTPServerManager() *HTTPServerManager {
	logger := logrus.New()
	cfg := &config.RabbitMQConfig{
		URL: "amqp://guest:guest@localhost:5672/",
		Node: config.NodeConfig{
			Role:            config.NodeRoleTask,
			HealthCheckPort: 8081,
			MetricsPort:     8082,
		},
	}

	return NewHTTPServerManager(
		cfg,
		rabbitmq.NewLoadMonitor(config.LoadMonitorConfig{}, logger),
		NewRabbitMQService(cfg, logger),
		nil,
		logger,
	)
}
