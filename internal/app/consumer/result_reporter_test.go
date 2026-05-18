package consumer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

func TestResultReporterReportWithRetryEventuallySucceeds(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&attempts, 1)
		if current < 3 {
			http.Error(w, "temporary failure", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	reporter := NewResultReporter(ReporterConfig{
		ReportURL:  server.URL,
		NodeID:     "node-test",
		Timeout:    time.Second,
		BufferSize: 1,
		RetryConfig: &config.RetryConfig{
			MaxRetries:    2,
			InitialDelay:  5 * time.Millisecond,
			MaxDelay:      5 * time.Millisecond,
			BackoffFactor: 1,
		},
	}, logrus.New())
	reporter.ctx = context.Background()

	reporter.processResult(&TaskResult{
		TaskID:     10,
		Status:     "success",
		Message:    "ok",
		NodeID:     reporter.nodeID,
		Timestamp:  time.Now().UnixMilli(),
		RetryCount: 0,
	})

	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("attempts = %d, want 3", got)
	}
	stats := reporter.GetStats()
	if stats.RetryReports != 2 {
		t.Fatalf("RetryReports = %d, want 2", stats.RetryReports)
	}
	if stats.SuccessReports != 1 {
		t.Fatalf("SuccessReports = %d, want 1", stats.SuccessReports)
	}
	if stats.FailedReports != 0 {
		t.Fatalf("FailedReports = %d, want 0", stats.FailedReports)
	}
}
