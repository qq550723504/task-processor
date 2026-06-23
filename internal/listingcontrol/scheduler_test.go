package listingcontrol

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"task-processor/internal/listingadmin"
	"task-processor/internal/model"
)

func TestSchedulerDispatchesReadyStoreToStoreQueue(t *testing.T) {
	ctx := context.Background()
	repo := newFakeDispatchRepo([]listingadmin.ImportTask{
		testImportTask(101, 10, 976, model.TaskStatusCrawled.Int16()),
	})
	publisher := &fakeTaskPublisher{}
	scheduler := NewScheduler(repo, fakeReadinessProvider{readiness: []StoreReadiness{
		readyStore(10, 976, "node-a", 3, 0),
	}}, publisher, SchedulerConfig{
		ClaimTokenPrefix: "test",
	})
	scheduler.TokenGenerator = fixedTokenGenerator{tokens: []string{"test-claim-1"}}

	summary, err := scheduler.DispatchOnce(ctx)
	if err != nil {
		t.Fatalf("DispatchOnce returned error: %v", err)
	}

	if summary.Candidates != 1 || summary.Dispatched != 1 || summary.Skipped != 0 || summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(repo.claims) != 1 {
		t.Fatalf("claims = %d, want 1", len(repo.claims))
	}
	if repo.claims[0].TaskID != 101 || repo.claims[0].PreviousStatus != model.TaskStatusCrawled.Int16() {
		t.Fatalf("unexpected claim: %+v", repo.claims[0])
	}
	if strings.TrimSpace(repo.claims[0].ProcessingNode) == "" {
		t.Fatal("claim processing node is blank")
	}
	if repo.claims[0].ProcessingNode != "test-claim-1" {
		t.Fatalf("processing node = %q, want deterministic token", repo.claims[0].ProcessingNode)
	}
	if len(publisher.tasks) != 1 {
		t.Fatalf("published tasks = %d, want 1", len(publisher.tasks))
	}
	published := publisher.tasks[0]
	if published.ID != 101 || published.TenantID != 10 || published.StoreID != 976 {
		t.Fatalf("published wrong task identity: %+v", published)
	}
	if published.Platform != "shein" || published.SourcePlatform != "amazon" {
		t.Fatalf("published wrong platforms: %+v", published)
	}
	if len(summary.Decisions) != 1 {
		t.Fatalf("decisions = %d, want 1", len(summary.Decisions))
	}
	decision := summary.Decisions[0]
	if decision.Action != DispatchActionDispatched || decision.Queue != "shein.tasks.store.976" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
	if decision.OwnerNode != "node-a" || decision.Capacity != 3 || decision.Queued != 0 {
		t.Fatalf("decision lost readiness details: %+v", decision)
	}
}

func TestSchedulerSkipsDisabledStore(t *testing.T) {
	repo := newFakeDispatchRepo([]listingadmin.ImportTask{
		testImportTask(102, 10, 976, model.TaskStatusPending.Int16()),
	})
	scheduler := NewScheduler(repo, fakeReadinessProvider{readiness: []StoreReadiness{{
		Store:  StoreSnapshot{TenantID: 10, StoreID: 976, Platform: "shein", Status: 1},
		Reason: ReasonStoreDisabled,
	}}}, &fakeTaskPublisher{}, SchedulerConfig{})

	summary, err := scheduler.DispatchOnce(context.Background())
	if err != nil {
		t.Fatalf("DispatchOnce returned error: %v", err)
	}

	assertSingleSkippedDecision(t, summary, 102, ReasonStoreDisabled)
	if len(repo.claims) != 0 {
		t.Fatalf("claims = %d, want 0", len(repo.claims))
	}
}

func TestSchedulerSkipsStoreWithoutOwner(t *testing.T) {
	repo := newFakeDispatchRepo([]listingadmin.ImportTask{
		testImportTask(103, 10, 976, model.TaskStatusPending.Int16()),
	})
	scheduler := NewScheduler(repo, fakeReadinessProvider{readiness: []StoreReadiness{{
		Store:  StoreSnapshot{TenantID: 10, StoreID: 976, Platform: "shein", Status: StoreStatusEnabled},
		Reason: ReasonNoLiveOwner,
	}}}, &fakeTaskPublisher{}, SchedulerConfig{})

	summary, err := scheduler.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	assertSingleSkippedDecision(t, summary, 103, ReasonNoLiveOwner)
	if len(repo.claims) != 0 {
		t.Fatalf("claims = %d, want 0", len(repo.claims))
	}
}

func TestSchedulerSkipsQuotaExhaustedWithoutClaiming(t *testing.T) {
	repo := newFakeDispatchRepo([]listingadmin.ImportTask{
		testImportTask(104, 10, 976, model.TaskStatusPendingRetry.Int16()),
	})
	scheduler := NewScheduler(repo, fakeReadinessProvider{readiness: []StoreReadiness{{
		Store:        StoreSnapshot{TenantID: 10, StoreID: 976, Platform: "shein", Status: StoreStatusEnabled},
		Dispatchable: false,
		Reason:       ReasonQuotaExhausted,
		OwnerNode:    "node-a",
		Capacity:     1,
		Queued:       1,
	}}}, &fakeTaskPublisher{}, SchedulerConfig{})

	summary, err := scheduler.DispatchOnce(context.Background())
	if err != nil {
		t.Fatalf("DispatchOnce returned error: %v", err)
	}

	assertSingleSkippedDecision(t, summary, 104, ReasonQuotaExhausted)
	if len(repo.claims) != 0 {
		t.Fatalf("claims = %d, want 0", len(repo.claims))
	}
}

func TestSchedulerRollsBackClaimWhenPublishFails(t *testing.T) {
	publishErr := errors.New("rabbit unavailable")
	repo := newFakeDispatchRepo([]listingadmin.ImportTask{
		testImportTask(105, 10, 976, model.TaskStatusCrawled.Int16()),
	})
	publisher := &fakeTaskPublisher{err: publishErr}
	scheduler := NewScheduler(repo, fakeReadinessProvider{readiness: []StoreReadiness{
		readyStore(10, 976, "node-a", 2, 0),
	}}, publisher, SchedulerConfig{})
	scheduler.TokenGenerator = fixedTokenGenerator{tokens: []string{"rollback-token"}}

	summary, err := scheduler.DispatchOnce(context.Background())
	if err != nil {
		t.Fatalf("DispatchOnce returned error: %v", err)
	}

	if summary.Failed != 1 || summary.Dispatched != 0 || len(summary.Decisions) != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	decision := summary.Decisions[0]
	if decision.Action != DispatchActionFailed {
		t.Fatalf("decision action = %q, want failed", decision.Action)
	}
	if !strings.Contains(decision.Reason, publishErr.Error()) {
		t.Fatalf("decision reason = %q, want publish error", decision.Reason)
	}
	if len(repo.rollbacks) != 1 {
		t.Fatalf("rollbacks = %d, want 1", len(repo.rollbacks))
	}
	rollback := repo.rollbacks[0]
	if rollback.taskID != 105 || rollback.previousStatus != model.TaskStatusCrawled.Int16() {
		t.Fatalf("unexpected rollback identity: %+v", rollback)
	}
	if rollback.processingNode != "rollback-token" {
		t.Fatalf("rollback processing node = %q, want claim token", rollback.processingNode)
	}
}

func TestSchedulerDryRunReportsDecisionsWithoutClaimOrPublish(t *testing.T) {
	repo := newFakeDispatchRepo([]listingadmin.ImportTask{
		testImportTask(106, 10, 976, model.TaskStatusPending.Int16()),
	})
	publisher := &fakeTaskPublisher{}
	scheduler := NewScheduler(repo, fakeReadinessProvider{readiness: []StoreReadiness{
		readyStore(10, 976, "node-a", 1, 0),
	}}, publisher, SchedulerConfig{DryRun: true})

	summary, err := scheduler.DispatchOnce(context.Background())
	if err != nil {
		t.Fatalf("DispatchOnce returned error: %v", err)
	}

	if summary.Dispatched != 0 || summary.Skipped != 1 || summary.Failed != 0 {
		t.Fatalf("unexpected dry-run summary: %+v", summary)
	}
	if len(repo.claims) != 0 {
		t.Fatalf("claims = %d, want 0", len(repo.claims))
	}
	if len(publisher.tasks) != 0 {
		t.Fatalf("published tasks = %d, want 0", len(publisher.tasks))
	}
	if len(summary.Decisions) != 1 {
		t.Fatalf("decisions = %d, want 1", len(summary.Decisions))
	}
	decision := summary.Decisions[0]
	if decision.Action != DispatchActionDryRun || decision.Queue != "shein.tasks.store.976" {
		t.Fatalf("unexpected dry-run decision: %+v", decision)
	}
}

func assertSingleSkippedDecision(t *testing.T, summary DispatchSummary, taskID int64, reason string) {
	t.Helper()
	if summary.Candidates != 1 || summary.Dispatched != 0 || summary.Skipped != 1 || summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(summary.Decisions) != 1 {
		t.Fatalf("decisions = %d, want 1", len(summary.Decisions))
	}
	decision := summary.Decisions[0]
	if decision.TaskID != taskID || decision.Action != DispatchActionSkipped || decision.Reason != reason {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func readyStore(tenantID, storeID int64, owner string, capacity int, queued int64) StoreReadiness {
	return StoreReadiness{
		Store:        StoreSnapshot{TenantID: tenantID, StoreID: storeID, Platform: "shein", Status: StoreStatusEnabled},
		Dispatchable: true,
		OwnerNode:    owner,
		Capacity:     capacity,
		Queued:       queued,
	}
}

func testImportTask(id, tenantID, storeID int64, status int16) listingadmin.ImportTask {
	now := time.Unix(1700000000, 0)
	categoryID := int64(7788)
	return listingadmin.ImportTask{
		ID:             id,
		TenantID:       tenantID,
		StoreID:        &storeID,
		Platform:       "legacy",
		TargetPlatform: "shein",
		SourcePlatform: "amazon",
		Region:         "US",
		CategoryID:     &categoryID,
		ProductID:      "B0TEST",
		Status:         status,
		RetryCount:     1,
		MaxRetryCount:  3,
		Remark:         "remark",
		Priority:       4,
		CreateTime:     &now,
		UpdateTime:     &now,
	}
}

type fakeDispatchRepo struct {
	candidates []listingadmin.ImportTask
	claims     []listingadmin.DispatchClaim
	rollbacks  []fakeRollback
	claimOK    bool
}

func newFakeDispatchRepo(candidates []listingadmin.ImportTask) *fakeDispatchRepo {
	return &fakeDispatchRepo{candidates: candidates, claimOK: true}
}

func (f *fakeDispatchRepo) ListDispatchCandidatesFair(ctx context.Context, req listingadmin.DispatchCandidateRequest) ([]listingadmin.ImportTask, error) {
	return f.candidates, nil
}

func (f *fakeDispatchRepo) ClaimForDispatch(ctx context.Context, claim listingadmin.DispatchClaim) (bool, error) {
	f.claims = append(f.claims, claim)
	return f.claimOK, nil
}

func (f *fakeDispatchRepo) RollbackDispatch(ctx context.Context, taskID int64, previousStatus int16, processingNode, reason string) error {
	f.rollbacks = append(f.rollbacks, fakeRollback{
		taskID:         taskID,
		previousStatus: previousStatus,
		processingNode: processingNode,
		reason:         reason,
	})
	return nil
}

type fakeRollback struct {
	taskID         int64
	previousStatus int16
	processingNode string
	reason         string
}

type fakeReadinessProvider struct {
	readiness []StoreReadiness
	err       error
}

func (f fakeReadinessProvider) ListReadiness(ctx context.Context, platform string) ([]StoreReadiness, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.readiness, nil
}

type fakeTaskPublisher struct {
	tasks []model.Task
	err   error
}

func (f *fakeTaskPublisher) PublishTask(ctx context.Context, task *model.Task) (PublishedDispatch, error) {
	if task != nil {
		f.tasks = append(f.tasks, *task)
	}
	if f.err != nil {
		return PublishedDispatch{}, f.err
	}
	return PublishedDispatch{Queue: "shein.tasks.store.976", MessageID: "published"}, nil
}

type fixedTokenGenerator struct {
	tokens []string
}

func (f fixedTokenGenerator) NewClaimToken(prefix string, taskID int64) string {
	if len(f.tokens) == 0 {
		return prefix + "-token"
	}
	return f.tokens[0]
}
