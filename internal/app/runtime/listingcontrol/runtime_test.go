package listingcontrol

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"task-processor/internal/core/config"
	controllib "task-processor/internal/listingcontrol"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestResolveConfigPathAndParseFlags(t *testing.T) {
	if got := ResolveConfigPath(""); got != "config/config-prod.yaml" {
		t.Fatalf("default config path = %q", got)
	}
	if got := ResolveConfigPath("config/custom.yaml"); got != "config/custom.yaml" {
		t.Fatalf("config path precedence = %q", got)
	}

	fs := flag.NewFlagSet("listing-control-plane", flag.ContinueOnError)
	opts := ParseFlagsFrom(fs,
		"--config", "config/runtime.yaml",
		"--log-level", "debug",
		"--force",
	)
	if opts.Config != "config/runtime.yaml" || opts.LogLevel != "debug" || !opts.Force {
		t.Fatalf("unexpected parsed options: %+v", opts)
	}
}

func TestRunReturnsNilWhenDisabledWithoutInitializingDependencies(t *testing.T) {
	configPath := writeRuntimeConfig(t, `
openai:
  apiKey: "test-key"
listingControlPlane:
  enabled: false
`)
	deps := newFakeRuntimeDeps()

	if err := runWithDependencies(context.Background(), Options{Config: configPath, LogLevel: "error"}, deps.runtimeDependencies); err != nil {
		t.Fatalf("runWithDependencies returned error: %v", err)
	}
	if deps.dbOpened || deps.redisOpened || deps.rabbitConnected {
		t.Fatalf("dependencies initialized for disabled control plane: %+v", deps)
	}
}

func TestRunErrorsWhenEnabledAndRequiredConfigsMissing(t *testing.T) {
	configPath := writeRuntimeConfig(t, `
openai:
  apiKey: "test-key"
listingControlPlane:
  enabled: true
`)
	deps := newFakeRuntimeDeps()

	err := runWithDependencies(context.Background(), Options{Config: configPath, LogLevel: "error"}, deps.runtimeDependencies)
	if err == nil {
		t.Fatal("expected missing dependency config error")
	}
	if !strings.Contains(err.Error(), "RabbitMQ") {
		t.Fatalf("expected RabbitMQ config error first, got %v", err)
	}
	if deps.dbOpened || deps.redisOpened || deps.rabbitConnected {
		t.Fatalf("dependencies initialized before config validation: %+v", deps)
	}
}

func TestRefreshableAMQPPublisherRetriesAfterClosedChannel(t *testing.T) {
	closedErr := errors.New(`Exception (504) Reason: "channel/connection is not open"`)
	first := &fakeRuntimeAMQPPublisher{err: closedErr}
	second := &fakeRuntimeAMQPPublisher{}
	refreshes := 0
	publisher := newRefreshableAMQPPublisher(first, func() (controllib.AMQPPublisher, error) {
		refreshes++
		return second, nil
	})

	msg := amqp.Publishing{
		ContentType: "application/json",
		MessageId:   "task-1",
		Body:        []byte(`{"taskId":"task-1"}`),
	}
	err := publisher.PublishWithContext(context.Background(), "", "shein.tasks.store.976", false, false, msg)
	if err != nil {
		t.Fatalf("PublishWithContext returned error: %v", err)
	}

	if refreshes != 1 {
		t.Fatalf("expected one channel refresh, got %d", refreshes)
	}
	if first.calls != 1 {
		t.Fatalf("expected first channel to be tried once, got %d", first.calls)
	}
	if second.calls != 1 {
		t.Fatalf("expected refreshed channel to be tried once, got %d", second.calls)
	}
	if second.key != "shein.tasks.store.976" || second.msg.MessageId != "task-1" {
		t.Fatalf("refreshed publish got key=%q messageId=%q", second.key, second.msg.MessageId)
	}
}

type fakeRuntimeAMQPPublisher struct {
	exchange  string
	key       string
	mandatory bool
	immediate bool
	msg       amqp.Publishing
	err       error
	calls     int
}

func (f *fakeRuntimeAMQPPublisher) PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	f.calls++
	f.exchange = exchange
	f.key = key
	f.mandatory = mandatory
	f.immediate = immediate
	f.msg = msg
	return f.err
}

func TestRunMigratesImportTaskSchemaBeforeConnectingRedisAndRabbitMQ(t *testing.T) {
	configPath := writeRuntimeConfig(t, `
openai:
  apiKey: "test-key"
database:
  host: "postgres"
  port: 5432
  user: "postgres"
  database: "ruoyi-vue-pro"
redis:
  host: "redis"
  port: 6379
rabbitmq:
  enabled: true
  url: "amqp://guest:guest@rabbitmq:5672/"
listingControlPlane:
  enabled: true
`)
	deps := newFakeRuntimeDeps()
	migrationErr := errors.New("migration failed")
	var migrated bool
	deps.MigrateImportTask = func(db *gorm.DB) error {
		migrated = true
		return migrationErr
	}

	err := runWithDependencies(context.Background(), Options{Config: configPath, LogLevel: "error"}, deps.runtimeDependencies)
	if !errors.Is(err, migrationErr) {
		t.Fatalf("expected migration error, got %v", err)
	}
	if !deps.dbOpened || !migrated {
		t.Fatalf("expected DB open and migration before failure: dbOpened=%v migrated=%v", deps.dbOpened, migrated)
	}
	if deps.redisOpened || deps.rabbitConnected {
		t.Fatalf("redis/rabbit should not initialize after migration failure: %+v", deps)
	}
}

func TestDirectStoreSourceMapsListingStoreRowsWithoutOwnerScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE listing_store (
		id integer primary key,
		tenant_id integer not null,
		owner_user_id text,
		name text,
		platform text not null,
		status integer not null,
		enable_auto_listing boolean,
		deleted integer not null default 0
	)`).Error; err != nil {
		t.Fatalf("create listing_store: %v", err)
	}
	if err := db.Exec(`INSERT INTO listing_store (id, tenant_id, owner_user_id, name, platform, status, enable_auto_listing, deleted) VALUES
		(100, 10, 'owner-a', 'ready', 'SHEIN', 0, true, 0),
		(101, 20, 'owner-b', 'disabled', 'shein', 1, false, 0),
		(102, 30, 'owner-c', 'deleted', 'shein', 0, true, 1),
		(103, 40, 'owner-d', 'other', 'temu', 0, true, 0)`).Error; err != nil {
		t.Fatalf("seed listing_store: %v", err)
	}

	source := NewDirectStoreSource(db)
	stores, err := source.ListEnabledAutoListingStores(context.Background(), "shein")
	if err != nil {
		t.Fatalf("ListEnabledAutoListingStores returned error: %v", err)
	}

	if len(stores) != 2 {
		t.Fatalf("expected 2 non-deleted shein rows, got %d: %+v", len(stores), stores)
	}
	if stores[0].TenantID != 10 || stores[0].StoreID != 100 || stores[0].Name != "ready" || stores[0].Platform != "SHEIN" {
		t.Fatalf("unexpected first store mapping: %+v", stores[0])
	}
	if stores[0].EnableAutoListing == nil || !*stores[0].EnableAutoListing {
		t.Fatalf("expected first auto-listing flag true: %+v", stores[0])
	}
	if stores[1].TenantID != 20 || stores[1].StoreID != 101 || stores[1].Status != 1 {
		t.Fatalf("expected disabled row to be included for readiness reasoning: %+v", stores[1])
	}
	if stores[1].EnableAutoListing == nil || *stores[1].EnableAutoListing {
		t.Fatalf("expected second auto-listing flag false: %+v", stores[1])
	}
}

func TestRabbitQueueDepthSourceDeclaresMissingStoreQueueAndReturnsZero(t *testing.T) {
	declarer := &fakeStoreQueueDeclarer{}
	source := newRabbitQueueDepthSource(
		func(name string) (amqp.Queue, error) {
			if name != "shein.tasks.store.100" {
				t.Fatalf("unexpected inspected queue name: %s", name)
			}
			return amqp.Queue{}, &amqp.Error{Code: 404, Reason: "NOT_FOUND - no queue"}
		},
		declarer,
		"shein",
	)

	depth, err := source.QueueDepth(context.Background(), 10, 100)
	if err != nil {
		t.Fatalf("QueueDepth returned error: %v", err)
	}
	if depth != 0 {
		t.Fatalf("expected missing queue depth 0, got %d", depth)
	}
	if !reflect.DeepEqual(declarer.declared, []string{"shein.tasks.store.100"}) {
		t.Fatalf("declared queues = %v", declarer.declared)
	}
	if !reflect.DeepEqual(declarer.bound, []string{"shein.tasks.store.100|shein.tasks.store.100|tasks.exchange"}) {
		t.Fatalf("bound queues = %v", declarer.bound)
	}
}

func TestRabbitQueueDepthSourceReturnsDeclarationErrorForMissingStoreQueue(t *testing.T) {
	declarationErr := errors.New("declare failed")
	declarer := &fakeStoreQueueDeclarer{declareErr: declarationErr}
	source := newRabbitQueueDepthSource(
		func(name string) (amqp.Queue, error) {
			return amqp.Queue{}, &amqp.Error{Code: 404, Reason: "NOT_FOUND - no queue"}
		},
		declarer,
		"shein",
	)

	_, err := source.QueueDepth(context.Background(), 10, 100)
	if !errors.Is(err, declarationErr) {
		t.Fatalf("expected declaration error, got %v", err)
	}
	if len(declarer.bound) != 0 {
		t.Fatalf("queue should not be bound after declaration failure: %v", declarer.bound)
	}
}

func TestControlPlaneServiceRunsRecoveryBeforeDispatchAndStopsOnContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	recovery := &fakeOnceRunner{name: "recovery"}
	var order []string
	service := controlPlaneService{
		Recovery: recovery.run(&order),
		Dispatch: func(ctx context.Context) (controllib.DispatchSummary, error) {
			order = append(order, "dispatch")
			cancel()
			return controllib.DispatchSummary{
				Candidates: 1,
				Dispatched: 1,
				Decisions: []controllib.DispatchDecision{{
					TenantID: 10,
					StoreID:  976,
					Action:   controllib.DispatchActionDispatched,
					Capacity: 4,
				}},
			}, nil
		},
		ScanInterval: time.Hour,
		Status:       NewStatusTracker(time.Date(2026, 6, 23, 1, 0, 0, 0, time.UTC)),
	}

	if err := service.Run(ctx); err != nil {
		t.Fatalf("service Run returned error: %v", err)
	}
	if !reflect.DeepEqual(order, []string{"recovery", "dispatch"}) {
		t.Fatalf("execution order = %v", order)
	}
	snapshot := service.Status.Snapshot()
	if !snapshot.Ready || snapshot.Status != "ok" || snapshot.Dispatch.Dispatched != 1 {
		t.Fatalf("unexpected status snapshot after success: %+v", snapshot)
	}
	if len(snapshot.Stores) != 1 || snapshot.Stores[0].StoreID != 976 || snapshot.Stores[0].Capacity != 4 {
		t.Fatalf("unexpected store status: %+v", snapshot.Stores)
	}
}

func TestControlPlaneServiceRunsPausedTaskRecoveryAtHourlyInterval(t *testing.T) {
	var order []string
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	service := controlPlaneService{
		PausedTaskRecovery: newIntervalRunner(time.Hour, func(ctx context.Context) error {
			order = append(order, "paused")
			return nil
		}, func() time.Time { return now }),
		Recovery: func(ctx context.Context) (controllib.RecoverySummary, error) {
			order = append(order, "recovery")
			return controllib.RecoverySummary{}, nil
		},
		Dispatch: func(ctx context.Context) (controllib.DispatchSummary, error) {
			order = append(order, "dispatch")
			return controllib.DispatchSummary{}, nil
		},
		Status: NewStatusTracker(now),
	}

	if err := service.runOnce(context.Background()); err != nil {
		t.Fatalf("first runOnce returned error: %v", err)
	}
	now = now.Add(59 * time.Minute)
	if err := service.runOnce(context.Background()); err != nil {
		t.Fatalf("second runOnce returned error: %v", err)
	}
	now = now.Add(time.Minute)
	if err := service.runOnce(context.Background()); err != nil {
		t.Fatalf("third runOnce returned error: %v", err)
	}

	want := []string{"paused", "recovery", "dispatch", "recovery", "dispatch", "paused", "recovery", "dispatch"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
}

func TestControlPlaneServiceSkipsCycleWhenLeaderLockIsHeldElsewhere(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var called bool
	lock := &fakeLeaderLock{
		snapshot: LeaderSnapshot{
			Key:      "listing:control-plane:leader:shein",
			Owner:    "other-node",
			IsLeader: false,
		},
	}
	service := controlPlaneService{
		LeaderLock: lock,
		Recovery: func(ctx context.Context) (controllib.RecoverySummary, error) {
			called = true
			return controllib.RecoverySummary{}, nil
		},
		Dispatch: func(ctx context.Context) (controllib.DispatchSummary, error) {
			called = true
			return controllib.DispatchSummary{}, nil
		},
		ScanInterval: time.Hour,
		Status:       NewStatusTracker(time.Date(2026, 6, 24, 1, 0, 0, 0, time.UTC)),
	}

	if err := service.runOnce(ctx); err != nil {
		t.Fatalf("runOnce returned error: %v", err)
	}
	if called {
		t.Fatal("recovery/dispatch should not run when another instance holds the leader lock")
	}
	if lock.acquireCalls != 1 {
		t.Fatalf("leader lock acquire calls = %d, want 1", lock.acquireCalls)
	}
	snapshot := service.Status.Snapshot()
	if !snapshot.Ready || snapshot.Status != "standby" || snapshot.Leader.IsLeader || snapshot.Leader.Owner != "other-node" {
		t.Fatalf("unexpected standby status: %+v", snapshot)
	}
}

func TestControlPlaneServiceRecordsLeaderWhenLockAcquired(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lock := &fakeLeaderLock{
		acquired: true,
		snapshot: LeaderSnapshot{
			Key:      "listing:control-plane:leader:shein",
			Owner:    "node-a",
			IsLeader: true,
		},
	}
	service := controlPlaneService{
		LeaderLock: lock,
		Recovery: func(ctx context.Context) (controllib.RecoverySummary, error) {
			return controllib.RecoverySummary{}, nil
		},
		Dispatch: func(ctx context.Context) (controllib.DispatchSummary, error) {
			return controllib.DispatchSummary{Dispatched: 1}, nil
		},
		ScanInterval: time.Hour,
		Status:       NewStatusTracker(time.Date(2026, 6, 24, 1, 0, 0, 0, time.UTC)),
	}

	if err := service.runOnce(ctx); err != nil {
		t.Fatalf("runOnce returned error: %v", err)
	}
	snapshot := service.Status.Snapshot()
	if !snapshot.Ready || snapshot.Status != "ok" || !snapshot.Leader.IsLeader || snapshot.Leader.Owner != "node-a" {
		t.Fatalf("unexpected leader status: %+v", snapshot)
	}
}

func TestControlPlaneServiceRenewsLeaderLockDuringLongCycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	renewed := make(chan struct{}, 1)
	lock := &fakeLeaderLock{
		acquired: true,
		snapshot: LeaderSnapshot{
			Key:      "listing:control-plane:leader:shein",
			Owner:    "node-a",
			IsLeader: true,
		},
		renewed: renewed,
	}
	service := controlPlaneService{
		LeaderLock:          lock,
		LeaderRenewInterval: 10 * time.Millisecond,
		Recovery: func(ctx context.Context) (controllib.RecoverySummary, error) {
			return controllib.RecoverySummary{}, nil
		},
		Dispatch: func(ctx context.Context) (controllib.DispatchSummary, error) {
			select {
			case <-renewed:
				return controllib.DispatchSummary{Dispatched: 1}, nil
			case <-ctx.Done():
				return controllib.DispatchSummary{}, ctx.Err()
			}
		},
		ScanInterval: time.Hour,
		Status:       NewStatusTracker(time.Date(2026, 6, 24, 1, 0, 0, 0, time.UTC)),
	}

	if err := service.runOnce(ctx); err != nil {
		t.Fatalf("runOnce returned error: %v", err)
	}
	if lock.acquireCalls < 2 {
		t.Fatalf("leader lock acquire calls = %d, want renewal during cycle", lock.acquireCalls)
	}
}

func TestControlPlaneServiceConcurrentInstancesShareLeaderLock(t *testing.T) {
	ctx := context.Background()
	lockState := &sharedLeaderLockState{}
	start := make(chan struct{})
	ready := make(chan struct{}, 2)
	errs := make(chan error, 2)
	var recoveryCalls atomic.Int32
	var dispatchCalls atomic.Int32

	statusA := NewStatusTracker(time.Date(2026, 6, 24, 1, 0, 0, 0, time.UTC))
	statusB := NewStatusTracker(time.Date(2026, 6, 24, 1, 0, 0, 0, time.UTC))
	runInstance := func(owner string, status *StatusTracker) {
		ready <- struct{}{}
		<-start
		service := controlPlaneService{
			LeaderLock: sharedLeaderLock{
				state: lockState,
				key:   "listing:control-plane:leader:shein",
				owner: owner,
			},
			Recovery: func(ctx context.Context) (controllib.RecoverySummary, error) {
				recoveryCalls.Add(1)
				return controllib.RecoverySummary{ProcessingRecovered: 1}, nil
			},
			Dispatch: func(ctx context.Context) (controllib.DispatchSummary, error) {
				dispatchCalls.Add(1)
				return controllib.DispatchSummary{Dispatched: 1}, nil
			},
			ScanInterval: time.Hour,
			Status:       status,
		}
		errs <- service.runOnce(ctx)
	}

	go runInstance("node-a", statusA)
	go runInstance("node-b", statusB)
	<-ready
	<-ready
	close(start)
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("runOnce returned error: %v", err)
		}
	}

	if recoveryCalls.Load() != 1 || dispatchCalls.Load() != 1 {
		t.Fatalf("concurrent instances ran recovery/dispatch more than once: recovery=%d dispatch=%d", recoveryCalls.Load(), dispatchCalls.Load())
	}
	snapshotA := statusA.Snapshot()
	snapshotB := statusB.Snapshot()
	if sameStatusCount(snapshotA, snapshotB, "ok") != 1 || sameStatusCount(snapshotA, snapshotB, "standby") != 1 {
		t.Fatalf("expected one leader cycle and one standby cycle: a=%+v b=%+v", snapshotA, snapshotB)
	}
	if snapshotA.Leader.Owner == "" || snapshotA.Leader.Owner != snapshotB.Leader.Owner {
		t.Fatalf("instances disagree on leader owner: a=%+v b=%+v", snapshotA.Leader, snapshotB.Leader)
	}
}

func TestControlPlaneServiceDoesNotDispatchAfterRecoveryError(t *testing.T) {
	recoveryErr := errors.New("recovery failed")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var order []string
	service := controlPlaneService{
		Recovery: func(ctx context.Context) (controllib.RecoverySummary, error) {
			order = append(order, "recovery")
			cancel()
			return controllib.RecoverySummary{}, recoveryErr
		},
		Dispatch: func(ctx context.Context) (controllib.DispatchSummary, error) {
			order = append(order, "dispatch")
			return controllib.DispatchSummary{}, nil
		},
		ScanInterval: time.Hour,
		Status:       NewStatusTracker(time.Now()),
	}

	if err := service.Run(ctx); err != nil {
		t.Fatalf("service Run returned error: %v", err)
	}
	if !reflect.DeepEqual(order, []string{"recovery"}) {
		t.Fatalf("execution order = %v", order)
	}
	snapshot := service.Status.Snapshot()
	if snapshot.Ready || snapshot.Status != "error" || snapshot.LastError != recoveryErr.Error() || snapshot.ConsecutiveErrors != 1 {
		t.Fatalf("unexpected status snapshot after error: %+v", snapshot)
	}
}

func TestControlPlaneServiceSkipsFirstCycleWhenContextAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var called bool
	service := controlPlaneService{
		Recovery: func(ctx context.Context) (controllib.RecoverySummary, error) {
			called = true
			return controllib.RecoverySummary{}, nil
		},
		Dispatch: func(ctx context.Context) (controllib.DispatchSummary, error) {
			called = true
			return controllib.DispatchSummary{}, nil
		},
	}

	if err := service.Run(ctx); err != nil {
		t.Fatalf("service Run returned error: %v", err)
	}
	if called {
		t.Fatal("service should not run a cycle after context cancellation")
	}
}

func TestStatusHandlerReadinessAndSummary(t *testing.T) {
	tracker := NewStatusTracker(time.Date(2026, 6, 23, 1, 0, 0, 0, time.UTC))
	handler := newStatusHandler(tracker)

	readyBefore := httptest.NewRecorder()
	handler.ServeHTTP(readyBefore, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if readyBefore.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready before success status = %d", readyBefore.Code)
	}

	tracker.RecordSuccess(controllib.RecoverySummary{
		ProcessingRecovered: 2,
	}, controllib.DispatchSummary{
		Candidates: 2,
		Skipped:    1,
		Failed:     1,
		Decisions: []controllib.DispatchDecision{
			{TenantID: 10, StoreID: 976, Action: controllib.DispatchActionSkipped, Reason: controllib.ReasonNoCapacity, Capacity: 4, Queued: 4},
			{TenantID: 10, StoreID: 1030, Action: controllib.DispatchActionFailed, Reason: "publish dispatch: failed"},
		},
	}, time.Date(2026, 6, 23, 1, 1, 0, 0, time.UTC))

	statusResponse := httptest.NewRecorder()
	handler.ServeHTTP(statusResponse, httptest.NewRequest(http.MethodGet, "/status", nil))
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("status response code = %d", statusResponse.Code)
	}
	body := statusResponse.Body.String()
	for _, want := range []string{`"ready":true`, `"no_capacity":1`, `"storeId":976`, `"processingRecovered":2`} {
		if !strings.Contains(body, want) {
			t.Fatalf("status body missing %s: %s", want, body)
		}
	}

	readyAfter := httptest.NewRecorder()
	handler.ServeHTTP(readyAfter, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if readyAfter.Code != http.StatusOK {
		t.Fatalf("ready after success status = %d", readyAfter.Code)
	}

	tracker.RecordStandby(LeaderSnapshot{
		Key:      "listing:control-plane:leader:shein",
		Owner:    "other-node",
		IsLeader: false,
	}, time.Date(2026, 6, 23, 1, 2, 0, 0, time.UTC))

	readyStandby := httptest.NewRecorder()
	handler.ServeHTTP(readyStandby, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if readyStandby.Code != http.StatusOK {
		t.Fatalf("ready during standby status = %d", readyStandby.Code)
	}
}

func TestStatusHandlerReportsLeaderLastSuccessAndConfigState(t *testing.T) {
	startedAt := time.Date(2026, 6, 24, 1, 0, 0, 0, time.UTC)
	successAt := time.Date(2026, 6, 24, 1, 1, 0, 0, time.UTC)
	standbyAt := time.Date(2026, 6, 24, 1, 2, 0, 0, time.UTC)
	tracker := NewStatusTracker(startedAt)
	tracker.RecordConfig(ControlPlaneConfigStatus{
		Platform: "shein",
		LeaderLock: ControlPlaneConfigFieldStatus{
			Value:     "listing:control-plane:leader:shein",
			Effective: true,
			Status:    "active",
		},
		QuotaKeyTTLGrace: ControlPlaneConfigFieldStatus{
			Value:     "1m0s",
			Effective: false,
			Status:    "reserved",
		},
	})
	tracker.RecordLeader(LeaderSnapshot{
		Key:      "listing:control-plane:leader:shein",
		Owner:    "node-a",
		IsLeader: true,
		TTL:      "30s",
	})
	tracker.RecordSuccess(controllib.RecoverySummary{}, controllib.DispatchSummary{Dispatched: 1}, successAt)
	tracker.RecordStandby(LeaderSnapshot{
		Key:      "listing:control-plane:leader:shein",
		Owner:    "node-b",
		IsLeader: false,
		TTL:      "30s",
	}, standbyAt)

	statusResponse := httptest.NewRecorder()
	newStatusHandler(tracker).ServeHTTP(statusResponse, httptest.NewRequest(http.MethodGet, "/status", nil))
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("status response code = %d", statusResponse.Code)
	}
	body := statusResponse.Body.String()
	for _, want := range []string{
		`"status":"standby"`,
		`"lastSuccessfulCycleAt":"2026-06-24T01:01:00Z"`,
		`"owner":"node-b"`,
		`"platform":"shein"`,
		`"leaderLock":{"value":"listing:control-plane:leader:shein","effective":true,"status":"active"}`,
		`"quotaKeyTTLGrace":{"value":"1m0s","effective":false,"status":"reserved"}`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("status body missing %s: %s", want, body)
		}
	}
}

type fakeOnceRunner struct {
	name string
}

func (f *fakeOnceRunner) run(order *[]string) func(context.Context) (controllib.RecoverySummary, error) {
	return func(ctx context.Context) (controllib.RecoverySummary, error) {
		*order = append(*order, f.name)
		return controllib.RecoverySummary{}, nil
	}
}

type fakeLeaderLock struct {
	acquired     bool
	snapshot     LeaderSnapshot
	acquireErr   error
	acquireCalls int
	renewed      chan<- struct{}
}

func (f *fakeLeaderLock) Acquire(ctx context.Context) (LeaderSnapshot, bool, error) {
	f.acquireCalls++
	if f.acquireCalls > 1 && f.renewed != nil {
		select {
		case f.renewed <- struct{}{}:
		default:
		}
	}
	return f.snapshot, f.acquired, f.acquireErr
}

type sharedLeaderLockState struct {
	mu    sync.Mutex
	owner string
}

type sharedLeaderLock struct {
	state *sharedLeaderLockState
	key   string
	owner string
}

func (l sharedLeaderLock) Acquire(ctx context.Context) (LeaderSnapshot, bool, error) {
	l.state.mu.Lock()
	defer l.state.mu.Unlock()
	acquired := false
	if l.state.owner == "" || l.state.owner == l.owner {
		l.state.owner = l.owner
		acquired = true
	}
	return LeaderSnapshot{
		Key:      l.key,
		Owner:    l.state.owner,
		IsLeader: acquired,
		TTL:      "30s",
	}, acquired, nil
}

func sameStatusCount(first, second ControlPlaneStatus, status string) int {
	count := 0
	if first.Status == status {
		count++
	}
	if second.Status == status {
		count++
	}
	return count
}

type fakeStoreQueueDeclarer struct {
	declared   []string
	bound      []string
	declareErr error
	bindErr    error
}

func (f *fakeStoreQueueDeclarer) DeclareQueue(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) error {
	if f.declareErr != nil {
		return f.declareErr
	}
	f.declared = append(f.declared, name)
	return nil
}

func (f *fakeStoreQueueDeclarer) BindQueue(queueName, routingKey, exchangeName string, noWait bool, args amqp.Table) error {
	if f.bindErr != nil {
		return f.bindErr
	}
	f.bound = append(f.bound, queueName+"|"+routingKey+"|"+exchangeName)
	return nil
}

type fakeRuntimeDeps struct {
	runtimeDependencies
	dbOpened        bool
	redisOpened     bool
	rabbitConnected bool
}

func newFakeRuntimeDeps() *fakeRuntimeDeps {
	deps := &fakeRuntimeDeps{}
	deps.runtimeDependencies = defaultRuntimeDependencies()
	deps.OpenDB = func(ctx context.Context, cfg *config.DatabaseConfig) (*gorm.DB, error) {
		deps.dbOpened = true
		return nil, nil
	}
	deps.OpenRedis = func(ctx context.Context, cfg *config.RedisConfig) (redisRuntime, error) {
		deps.redisOpened = true
		return nil, nil
	}
	deps.OpenRabbitMQ = func(ctx context.Context, cfg *config.RabbitMQConfig, logger *logrus.Logger) (rabbitRuntime, error) {
		deps.rabbitConnected = true
		return nil, nil
	}
	return deps
}

func writeRuntimeConfig(t *testing.T, body string) string {
	t.Helper()

	path := t.TempDir() + string(os.PathSeparator) + "config.yaml"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
