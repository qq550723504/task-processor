# Infrastructure Open Source Replacement Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the highest-value self-built infrastructure pieces in phase 1 by introducing official Prometheus metrics, a shared resilience layer for retry/ratelimit/circuit breaker, and a safer self-update adapter strategy.

**Architecture:** This phase keeps all business behavior and public APIs stable. The code changes are limited to infrastructure seams: metrics emission moves behind Prometheus collectors, retry/ratelimit/circuit-breaker logic moves behind a shared `internal/infra/resilience` package, and updater code is split behind an adapter so the current updater can be preserved while validating a third-party implementation. The migration is incremental and each subsystem remains deployable on its own.

**Tech Stack:** Go, Gin/net/http, `prometheus/client_golang`, `golang.org/x/time/rate`, `github.com/sony/gobreaker`, `github.com/cenkalti/backoff/v5`, existing updater package

---

## File Structure

### Metrics migration

- Modify: `D:\code\task-processor\internal\app\consumer\http_servers.go`
- Create: `D:\code\task-processor\internal\infra\metrics\consumer_registry.go`
- Create: `D:\code\task-processor\internal\infra\metrics\consumer_registry_test.go`
- Create: `D:\code\task-processor\internal\app\consumer\http_servers_test.go`

Responsibility split:

- `consumer_registry.go` owns Prometheus collector registration and snapshot-to-metric mapping.
- `http_servers.go` keeps HTTP routing and health/stats JSON responses, but delegates `/metrics` to Prometheus handler.
- `http_servers_test.go` verifies HTTP integration and basic metric surface.

### Resilience convergence

- Create: `D:\code\task-processor\internal\infra\resilience\ratelimit.go`
- Create: `D:\code\task-processor\internal\infra\resilience\breaker.go`
- Create: `D:\code\task-processor\internal\infra\resilience\retry.go`
- Create: `D:\code\task-processor\internal\infra\resilience\retry_test.go`
- Create: `D:\code\task-processor\internal\infra\resilience\ratelimit_test.go`
- Create: `D:\code\task-processor\internal\infra\resilience\breaker_test.go`
- Modify: `D:\code\task-processor\internal\amazon\api\ratelimit.go`
- Modify: `D:\code\task-processor\internal\app\taskstatus\service.go`
- Modify: `D:\code\task-processor\internal\app\consumer\result_reporter.go`
- Modify: `D:\code\task-processor\go.mod`

Responsibility split:

- `internal/infra/resilience` wraps third-party libraries behind local types.
- `internal/amazon/api/ratelimit.go` becomes an adapter over the shared resilience package.
- `taskstatus/service.go` and `result_reporter.go` reuse the shared retry policy instead of bespoke retry loops.

### Updater adapter and library validation seam

- Create: `D:\code\task-processor\internal\app\updater\autoupdate_adapter.go`
- Create: `D:\code\task-processor\internal\app\updater\autoupdate_adapter_test.go`
- Modify: `D:\code\task-processor\internal\app\updater\update_manager.go`
- Modify: `D:\code\task-processor\internal\app\updater\updater.go`
- Modify: `D:\code\task-processor\internal\app\updater\version_manager.go`
- Modify: `D:\code\task-processor\internal\app\updater\file_downloader.go`
- Modify: `D:\code\task-processor\internal\app\updater\file_manager.go`
- Create: `D:\code\task-processor\docs\architecture\updater-third-party-validation.md`

Responsibility split:

- `autoupdate_adapter.go` defines the seam between business-level update flow and the underlying self-update implementation.
- existing updater files keep current behavior but call through the adapter.
- `updater-third-party-validation.md` records criteria for adopting `go-selfupdate`-style libraries safely on Windows.

## Task 1: Switch `/metrics` to official Prometheus collectors

**Files:**
- Create: `D:\code\task-processor\internal\infra\metrics\consumer_registry.go`
- Create: `D:\code\task-processor\internal\infra\metrics\consumer_registry_test.go`
- Modify: `D:\code\task-processor\internal\app\consumer\http_servers.go`
- Create: `D:\code\task-processor\internal\app\consumer\http_servers_test.go`
- Modify: `D:\code\task-processor\go.mod`

- [ ] **Step 1: Add Prometheus dependency**

```bash
go get github.com/prometheus/client_golang@latest
go mod tidy
```

Expected result:

- `go.mod` and `go.sum` include `github.com/prometheus/client_golang`

- [ ] **Step 2: Write failing registry test for exported metrics**

Create `D:\code\task-processor\internal\infra\metrics\consumer_registry_test.go`:

```go
package metrics

import (
	"testing"

	promtest "github.com/prometheus/client_golang/prometheus/testutil"
)

func TestConsumerRegistryExportsTaskCounters(t *testing.T) {
	reg := NewConsumerRegistry()
	reg.UpdateConsumerSnapshot(ConsumerSnapshot{
		TasksProcessed: 12,
		TasksSucceeded: 9,
		TasksFailed:    3,
	})

	if got := promtest.ToFloat64(reg.tasksProcessedTotal); got != 12 {
		t.Fatalf("tasks_processed_total = %v, want 12", got)
	}
	if got := promtest.ToFloat64(reg.tasksSucceededTotal); got != 9 {
		t.Fatalf("tasks_succeeded_total = %v, want 9", got)
	}
	if got := promtest.ToFloat64(reg.tasksFailedTotal); got != 3 {
		t.Fatalf("tasks_failed_total = %v, want 3", got)
	}
}
```

- [ ] **Step 3: Implement the registry wrapper**

Create `D:\code\task-processor\internal\infra\metrics\consumer_registry.go`:

```go
package metrics

import "github.com/prometheus/client_golang/prometheus"

type ConsumerSnapshot struct {
	TasksProcessed int64
	TasksSucceeded int64
	TasksFailed    int64
}

type ConsumerRegistry struct {
	registry            *prometheus.Registry
	tasksProcessedTotal prometheus.Gauge
	tasksSucceededTotal prometheus.Gauge
	tasksFailedTotal    prometheus.Gauge
}

func NewConsumerRegistry() *ConsumerRegistry {
	registry := prometheus.NewRegistry()
	r := &ConsumerRegistry{
		registry: prometheus.NewRegistry(),
		tasksProcessedTotal: prometheus.NewGauge(prometheus.GaugeOpts{Name: "tasks_processed_total", Help: "Total number of tasks processed"}),
		tasksSucceededTotal: prometheus.NewGauge(prometheus.GaugeOpts{Name: "tasks_succeeded_total", Help: "Total number of tasks succeeded"}),
		tasksFailedTotal:    prometheus.NewGauge(prometheus.GaugeOpts{Name: "tasks_failed_total", Help: "Total number of tasks failed"}),
	}
	r.registry.MustRegister(r.tasksProcessedTotal, r.tasksSucceededTotal, r.tasksFailedTotal)
	_ = registry
	return r
}

func (r *ConsumerRegistry) UpdateConsumerSnapshot(snapshot ConsumerSnapshot) {
	r.tasksProcessedTotal.Set(float64(snapshot.TasksProcessed))
	r.tasksSucceededTotal.Set(float64(snapshot.TasksSucceeded))
	r.tasksFailedTotal.Set(float64(snapshot.TasksFailed))
}

func (r *ConsumerRegistry) Registry() *prometheus.Registry {
	return r.registry
}
```

- [ ] **Step 4: Run the unit test**

Run:

```bash
go test ./internal/infra/metrics -run TestConsumerRegistryExportsTaskCounters
```

Expected:

- PASS

- [ ] **Step 5: Replace manual `/metrics` handler with Prometheus handler**

Modify `D:\code\task-processor\internal\app\consumer\http_servers.go`:

```go
import (
	...
	appmetrics "task-processor/internal/infra/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type HTTPServerManager struct {
	...
	consumerMetrics *appmetrics.ConsumerRegistry
}

func NewHTTPServerManager(...) *HTTPServerManager {
	return &HTTPServerManager{
		...
		consumerMetrics: appmetrics.NewConsumerRegistry(),
	}
}

func (h *HTTPServerManager) startMetricsServer() {
	defer h.wg.Done()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(h.consumerMetrics.Registry(), promhttp.HandlerOpts{}))
	mux.HandleFunc("/stats", h.handleStats)

	h.metricsServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", h.config.Node.MetricsPort),
		Handler: mux,
	}
	runServer(h.metricsServer, "指标服务器", h.config.Node.MetricsPort, h.logger)
}

func (h *HTTPServerManager) refreshMetricsSnapshot() {
	stats := h.loadMonitor.GetStats()
	h.consumerMetrics.UpdateConsumerSnapshot(appmetrics.ConsumerSnapshot{
		TasksProcessed: stats.TasksProcessed,
		TasksSucceeded: stats.TasksSucceeded,
		TasksFailed:    stats.TasksFailed,
	})
}
```

- [ ] **Step 6: Add failing HTTP integration test for `/metrics`**

Create `D:\code\task-processor\internal\app\consumer\http_servers_test.go`:

```go
package consumer

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsEndpointIncludesPrometheusMetricNames(t *testing.T) {
	manager := newTestHTTPServerManager(t)
	manager.refreshMetricsSnapshot()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		manager.consumerMetrics.Registry()
	})

	handler.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "tasks_processed_total") {
		t.Fatalf("metrics body missing tasks_processed_total: %s", rr.Body.String())
	}
}
```

- [ ] **Step 7: Make the integration test pass with a real handler**

Update the test to use the Prometheus HTTP handler:

```go
handler := promhttp.HandlerFor(manager.consumerMetrics.Registry(), promhttp.HandlerOpts{})
handler.ServeHTTP(rr, req)
```

Run:

```bash
go test ./internal/app/consumer -run TestMetricsEndpointIncludesPrometheusMetricNames
```

Expected:

- PASS

- [ ] **Step 8: Commit metrics migration**

```bash
git add go.mod go.sum internal/infra/metrics/consumer_registry.go internal/infra/metrics/consumer_registry_test.go internal/app/consumer/http_servers.go internal/app/consumer/http_servers_test.go
git commit -m "refactor: move consumer metrics to prometheus collectors"
```

## Task 2: Converge retry, rate limit, and circuit breaker into `internal/infra/resilience`

**Files:**
- Create: `D:\code\task-processor\internal\infra\resilience\ratelimit.go`
- Create: `D:\code\task-processor\internal\infra\resilience\breaker.go`
- Create: `D:\code\task-processor\internal\infra\resilience\retry.go`
- Create: `D:\code\task-processor\internal\infra\resilience\ratelimit_test.go`
- Create: `D:\code\task-processor\internal\infra\resilience\breaker_test.go`
- Create: `D:\code\task-processor\internal\infra\resilience\retry_test.go`
- Modify: `D:\code\task-processor\internal\amazon\api\ratelimit.go`
- Modify: `D:\code\task-processor\internal\app\taskstatus\service.go`
- Modify: `D:\code\task-processor\internal\app\consumer\result_reporter.go`
- Modify: `D:\code\task-processor\go.mod`

- [ ] **Step 1: Add resilience dependencies**

```bash
go get golang.org/x/time/rate@latest
go get github.com/sony/gobreaker@latest
go get github.com/cenkalti/backoff/v5@latest
go mod tidy
```

- [ ] **Step 2: Write failing retry test**

Create `D:\code\task-processor\internal\infra\resilience\retry_test.go`:

```go
package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryEventuallySucceeds(t *testing.T) {
	attempts := 0
	err := Retry(context.Background(), RetryConfig{
		MaxAttempts: 3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay: 20 * time.Millisecond,
		Multiplier: 1,
	}, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("try again")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}
```

- [ ] **Step 3: Implement retry wrapper**

Create `D:\code\task-processor\internal\infra\resilience\retry.go`:

```go
package resilience

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v5"
)

type RetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

func Retry(ctx context.Context, cfg RetryConfig, fn func() error) error {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = cfg.InitialDelay
	b.MaxInterval = cfg.MaxDelay
	b.Multiplier = cfg.Multiplier

	operation := backoff.WithMaxRetries(backoff.WithContext(b, ctx), uint64(cfg.MaxAttempts-1))
	return backoff.Retry(fn, operation)
}
```

- [ ] **Step 4: Implement rate limiter and breaker wrappers**

Create `D:\code\task-processor\internal\infra\resilience\ratelimit.go`:

```go
package resilience

import (
	"context"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	limiter *rate.Limiter
}

func NewRateLimiter(r rate.Limit, burst int) *RateLimiter {
	return &RateLimiter{limiter: rate.NewLimiter(r, burst)}
}

func (r *RateLimiter) Wait(ctx context.Context) error {
	return r.limiter.Wait(ctx)
}

func (r *RateLimiter) Allow() bool {
	return r.limiter.Allow()
}
```

Create `D:\code\task-processor\internal\infra\resilience\breaker.go`:

```go
package resilience

import (
	"time"

	"github.com/sony/gobreaker"
)

type CircuitBreaker struct {
	breaker *gobreaker.CircuitBreaker
}

func NewCircuitBreaker(name string, failureThreshold uint32, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		breaker: gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name: name,
			Timeout: timeout,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				return counts.ConsecutiveFailures >= failureThreshold
			},
		}),
	}
}

func (c *CircuitBreaker) Execute(fn func() error) error {
	_, err := c.breaker.Execute(func() (any, error) {
		return nil, fn()
	})
	return err
}
```

- [ ] **Step 5: Run resilience unit tests**

Run:

```bash
go test ./internal/infra/resilience
```

Expected:

- PASS

- [ ] **Step 6: Replace bespoke retry loop in task status service**

Modify `D:\code\task-processor\internal\app\taskstatus\service.go`:

```go
import infraresilience "task-processor/internal/infra/resilience"

func (s *Service) updateSync(input UpdateInput) error {
	...
	retryErr := infraresilience.Retry(context.Background(), infraresilience.RetryConfig{
		MaxAttempts:  s.maxRetries,
		InitialDelay: 1 * time.Second,
		MaxDelay:     3 * time.Second,
		Multiplier:   1,
	}, func() error {
		err := client.UpdateTaskStatus(req)
		if isNonRetriableUpdateErr(err) {
			return backoff.Permanent(err)
		}
		return err
	})
	...
}
```

- [ ] **Step 7: Replace bespoke retry loop in result reporter**

Modify `D:\code\task-processor\internal\app\consumer\result_reporter.go`:

```go
import infraresilience "task-processor/internal/infra/resilience"

func (rr *ResultReporter) reportWithRetry(result *TaskResult) error {
	return infraresilience.Retry(rr.ctx, infraresilience.RetryConfig{
		MaxAttempts:  rr.retryConfig.MaxRetries + 1,
		InitialDelay: rr.retryConfig.InitialDelay,
		MaxDelay:     rr.retryConfig.MaxDelay,
		Multiplier:   rr.retryConfig.BackoffFactor,
	}, func() error {
		return rr.doReport(result)
	})
}
```

- [ ] **Step 8: Convert Amazon limiter implementation into an adapter**

Modify `D:\code\task-processor\internal\amazon\api\ratelimit.go` so the public shape stays stable:

```go
import (
	...
	infraresilience "task-processor/internal/infra/resilience"
	"golang.org/x/time/rate"
)

type RateLimiter interface {
	Wait(ctx context.Context) error
	Allow() bool
}

func NewTokenBucketLimiter(rateValue, capacity float64) *infraresilience.RateLimiter {
	return infraresilience.NewRateLimiter(rate.Limit(rateValue), int(capacity))
}

func NewCircuitBreaker(failureThreshold, successThreshold int, timeout time.Duration) *infraresilience.CircuitBreaker {
	_ = successThreshold
	return infraresilience.NewCircuitBreaker("amazon-sp-api", uint32(failureThreshold), timeout)
}
```

- [ ] **Step 9: Run targeted tests**

Run:

```bash
go test ./internal/amazon/api ./internal/app/taskstatus ./internal/app/consumer ./internal/infra/resilience
```

Expected:

- PASS

- [ ] **Step 10: Commit resilience convergence**

```bash
git add go.mod go.sum internal/infra/resilience internal/amazon/api/ratelimit.go internal/app/taskstatus/service.go internal/app/consumer/result_reporter.go
git commit -m "refactor: converge retry and resilience primitives"
```

## Task 3: Introduce updater adapter and validate third-party adoption seam

**Files:**
- Create: `D:\code\task-processor\internal\app\updater\autoupdate_adapter.go`
- Create: `D:\code\task-processor\internal\app\updater\autoupdate_adapter_test.go`
- Modify: `D:\code\task-processor\internal\app\updater\update_manager.go`
- Modify: `D:\code\task-processor\internal\app\updater\version_manager.go`
- Modify: `D:\code\task-processor\internal\app\updater\file_downloader.go`
- Modify: `D:\code\task-processor\internal\app\updater\file_manager.go`
- Create: `D:\code\task-processor\docs\architecture\updater-third-party-validation.md`

- [ ] **Step 1: Write failing adapter test**

Create `D:\code\task-processor\internal\app\updater\autoupdate_adapter_test.go`:

```go
package updater

import "testing"

func TestDefaultAdapterUsesExistingManagers(t *testing.T) {
	adapter := NewDefaultAutoUpdateAdapter(
		NewVersionManager("1.0.0", "https://example.com/version.json"),
		NewFileDownloader(false),
		NewFileManager(),
	)
	if adapter == nil {
		t.Fatal("adapter is nil")
	}
}
```

- [ ] **Step 2: Implement adapter seam**

Create `D:\code\task-processor\internal\app\updater\autoupdate_adapter.go`:

```go
package updater

type AutoUpdateAdapter interface {
	FetchLatestVersion() (*VersionInfo, error)
	IsUpdateAvailable(*VersionInfo) bool
	DownloadAndStage(*VersionInfo) error
	MarkApplied(version string)
	Restart()
}

type defaultAutoUpdateAdapter struct {
	versionManager *VersionManager
	fileDownloader *FileDownloader
	fileManager    *FileManager
}

func NewDefaultAutoUpdateAdapter(vm *VersionManager, fd *FileDownloader, fm *FileManager) AutoUpdateAdapter {
	return &defaultAutoUpdateAdapter{
		versionManager: vm,
		fileDownloader: fd,
		fileManager:    fm,
	}
}
```

- [ ] **Step 3: Refactor update manager to depend on the adapter**

Modify `D:\code\task-processor\internal\app\updater\update_manager.go`:

```go
type UpdateManager struct {
	currentVersion string
	adapter        AutoUpdateAdapter
	fileManager    *FileManager
}

func NewUpdateManager(currentVersion, updateURL string, insecureSkipVerify bool) *UpdateManager {
	vm := NewVersionManager(currentVersion, updateURL)
	fd := NewFileDownloader(insecureSkipVerify)
	fm := NewFileManager()
	return &UpdateManager{
		currentVersion: currentVersion,
		adapter:        NewDefaultAutoUpdateAdapter(vm, fd, fm),
		fileManager:    fm,
	}
}
```

- [ ] **Step 4: Preserve existing runtime behavior behind the adapter**

Implement the adapter methods using existing updater components:

```go
func (a *defaultAutoUpdateAdapter) FetchLatestVersion() (*VersionInfo, error) {
	return a.versionManager.FetchLatestVersion()
}

func (a *defaultAutoUpdateAdapter) IsUpdateAvailable(v *VersionInfo) bool {
	return a.versionManager.IsUpdateAvailable(v)
}

func (a *defaultAutoUpdateAdapter) DownloadAndStage(v *VersionInfo) error {
	tmpFile := a.fileDownloader.GetTempFilePath()
	if err := a.fileDownloader.DownloadWithRetry(v.DownloadURL, tmpFile, v.SHA256, 3); err != nil {
		return err
	}
	return a.fileManager.ReplaceExecutable(tmpFile, v.Version)
}

func (a *defaultAutoUpdateAdapter) MarkApplied(version string) {
	a.fileManager.MarkAsUpdated(version)
}

func (a *defaultAutoUpdateAdapter) Restart() {
	a.fileManager.RestartProgram()
}
```

- [ ] **Step 5: Update `CheckAndUpdate` to call the adapter**

Modify `D:\code\task-processor\internal\app\updater\update_manager.go`:

```go
latestVersion, err := um.adapter.FetchLatestVersion()
...
if !um.adapter.IsUpdateAvailable(latestVersion) {
	return
}
if err := um.adapter.DownloadAndStage(latestVersion); err != nil {
	...
}
um.adapter.MarkApplied(latestVersion.Version)
um.adapter.Restart()
```

- [ ] **Step 6: Record third-party validation criteria**

Create `D:\code\task-processor\docs\architecture\updater-third-party-validation.md`:

```md
# Updater Third-Party Validation

## Candidate libraries

- `github.com/sanbornm/go-selfupdate`
- `github.com/creativeprojects/go-selfupdate`

## Validation checklist

1. Supports Windows executable replacement without corrupting the running binary
2. Supports checksum verification before swap
3. Can work with the current private version metadata endpoint
4. Does not require GitHub Releases semantics if the current release source is custom
5. Can preserve current delayed-restart behavior on Windows

## Decision rule

- If a library passes all five checks, replace `defaultAutoUpdateAdapter`
- If not, keep the adapter seam and retain the current implementation
```

- [ ] **Step 7: Run updater tests**

Run:

```bash
go test ./internal/app/updater
```

Expected:

- PASS

- [ ] **Step 8: Commit updater seam**

```bash
git add internal/app/updater/autoupdate_adapter.go internal/app/updater/autoupdate_adapter_test.go internal/app/updater/update_manager.go internal/app/updater/version_manager.go internal/app/updater/file_downloader.go internal/app/updater/file_manager.go docs/architecture/updater-third-party-validation.md
git commit -m "refactor: add updater adapter for third-party validation"
```

## Task 4: Final verification and rollout notes

**Files:**
- Modify: `D:\code\task-processor\docs\architecture\open-source-replacement-evaluation.md`
- Modify: `D:\code\task-processor\README.md`

- [ ] **Step 1: Run the focused phase-1 test suite**

```bash
go test ./internal/infra/metrics ./internal/infra/resilience ./internal/app/consumer ./internal/app/taskstatus ./internal/amazon/api ./internal/app/updater
```

Expected:

- PASS

- [ ] **Step 2: Run the broader project safety check**

```bash
go test ./...
```

Expected:

- PASS, or a short written list of unrelated pre-existing failures

- [ ] **Step 3: Update architecture notes to mark phase-1 status**

Add to `D:\code\task-processor\docs\architecture\open-source-replacement-evaluation.md`:

```md
## Phase 1 status

- Prometheus metrics: implemented
- Shared resilience layer: implemented
- Updater adapter seam: implemented
- Third-party updater replacement: pending validation
```

- [ ] **Step 4: Add operator note to README**

Add a short section to `D:\code\task-processor\README.md`:

```md
## Infrastructure Notes

- `/metrics` now uses Prometheus collectors
- retry / ratelimit / circuit-breaker behavior is centralized in `internal/infra/resilience`
- updater behavior is routed through an adapter to allow safe third-party validation
```

- [ ] **Step 5: Commit documentation and verification**

```bash
git add docs/architecture/open-source-replacement-evaluation.md README.md
git commit -m "docs: record phase 1 infrastructure replacement status"
```

## Self-Review

### Spec coverage

- Prometheus metrics migration: covered by Task 1
- Retry / rate-limit / circuit-breaker convergence: covered by Task 2
- Updater third-party adoption seam: covered by Task 3
- Verification and operator-facing status notes: covered by Task 4

No phase-1 requirement is left without a task.

### Placeholder scan

- No `TODO`, `TBD`, or deferred implementation markers remain in the plan steps.
- Each code-bearing step includes concrete file targets and concrete snippet direction.

### Type consistency

- Shared resilience package name is consistently `internal/infra/resilience`
- Prometheus registry wrapper is consistently `ConsumerRegistry`
- Updater seam is consistently `AutoUpdateAdapter`

Plan complete and saved to `docs/superpowers/plans/2026-05-17-infra-open-source-replacement-phase1.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
