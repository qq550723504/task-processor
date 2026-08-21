package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

func TestStudioAsyncJobStartsAndReturnsSucceededDesignJob(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubStudioMediaHandlerService{
		studioDesigns: &listingkit.StudioDesignResponse{
			Prompt: "retro cherries",
			Images: []listingkit.StudioGeneratedImage{{
				ID:       "design-1",
				ImageURL: "https://example.com/design.png",
			}},
		},
	}
	subscriptionService := activeStudioSubscriptionService(t)
	h, err := NewHandler(&stubHandlerCoreService{}, WithStudioMediaService(svc), WithStudioSessionAsyncJobService(svc), WithSubscriptionService(subscriptionService))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/async-jobs", h.StartStudioAsyncJob)
	router.GET("/api/v1/listing-kits/studio/async-jobs/:job_id", h.GetStudioAsyncJob)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/async-jobs", strings.NewReader(`{"path":"/studio/designs","body":{"prompt":"retro cherries","count":1}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("start status = %d, want 202 body=%s", resp.Code, resp.Body.String())
	}

	var started struct {
		JobID  string `json:"job_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &started); err != nil {
		t.Fatalf("unmarshal start body: %v", err)
	}
	if started.JobID == "" || started.Status != "running" {
		t.Fatalf("started = %+v, want running job id", started)
	}

	var polled struct {
		Status string                           `json:"status"`
		Result *listingkit.StudioDesignResponse `json:"result"`
	}
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/studio/async-jobs/"+started.JobID, nil)
		resp = httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("poll status = %d, want 200 body=%s", resp.Code, resp.Body.String())
		}
		if err := json.Unmarshal(resp.Body.Bytes(), &polled); err != nil {
			t.Fatalf("unmarshal poll body: %v", err)
		}
		if polled.Status == "succeeded" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if polled.Status != "succeeded" || polled.Result == nil || len(polled.Result.Images) != 1 {
		t.Fatalf("polled = %+v, want succeeded design result", polled)
	}
	if svc.studioDesignReq == nil || svc.studioDesignReq.Prompt != "retro cherries" {
		t.Fatalf("studio design req = %+v, want bound prompt", svc.studioDesignReq)
	}
	summary, err := subscriptionService.GetSummary(t.Context(), listingkit.DefaultTenantID)
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}
	var studioUsage int
	for _, item := range summary.Entitlements {
		if item.Module.Code == listingsubscription.ModuleStudio {
			studioUsage = item.Used["design_jobs"]
			break
		}
	}
	if studioUsage != 1 {
		t.Fatalf("studio design_jobs usage = %d, want 1", studioUsage)
	}
}

type blockingStudioAsyncMediaService struct {
	*stubStudioMediaHandlerService
	started chan<- struct{}
	release <-chan struct{}
}

func (s *blockingStudioAsyncMediaService) GenerateStudioProductImages(ctx context.Context, req *listingkit.StudioProductImageRequest) (*listingkit.StudioProductImageResponse, error) {
	select {
	case s.started <- struct{}{}:
	default:
	}
	select {
	case <-s.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestRunStudioAsyncJobHeartbeatsBeforeLongProductImageGeneration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := listingkit.WithRequestIdentity(listingkit.WithTenantID(context.Background(), "tenant-heartbeat"), listingkit.RequestIdentity{TenantID: "tenant-heartbeat", UserID: "user-heartbeat"})
	repo := listingkit.NewMemStudioAsyncJobRepository()
	old := time.Now().UTC().Add(-2 * time.Minute)
	if err := repo.CreateStudioAsyncJob(ctx, &listingkit.StudioAsyncJobRecord{
		ID: "heartbeat-job", TenantID: "tenant-heartbeat", UserID: "user-heartbeat", Path: "/studio/product-images",
		Status: listingkit.StudioAsyncJobStatusRunning, CreatedAt: old, UpdatedAt: old,
	}); err != nil {
		t.Fatalf("CreateStudioAsyncJob() error = %v", err)
	}
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	media := &blockingStudioAsyncMediaService{stubStudioMediaHandlerService: &stubStudioMediaHandlerService{}, started: started, release: release}
	h := &handler{studioAsyncJobs: &studioAsyncJobStore{repo: repo}, studioMediaService: media}
	done := make(chan struct{})
	go func() {
		h.runStudioAsyncJob(ctx, "heartbeat-job", "/studio/product-images", json.RawMessage(`{}`), "", "", "", "")
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("product-image generation did not start")
	}
	job, err := repo.GetStudioAsyncJob(ctx, "heartbeat-job")
	if err != nil {
		t.Fatalf("GetStudioAsyncJob() error = %v", err)
	}
	if !job.UpdatedAt.After(old) {
		t.Fatalf("job UpdatedAt = %s, want heartbeat after %s", job.UpdatedAt, old)
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runStudioAsyncJob did not finish")
	}
}

type failingHeartbeatStudioAsyncJobRepository struct {
	listingkit.StudioAsyncJobRepository
	failHeartbeat       bool
	heartbeatCalls      atomic.Int32
	periodicFailures    atomic.Int32
	alwaysFailHeartbeat atomic.Bool
}

func (r *failingHeartbeatStudioAsyncJobRepository) HeartbeatStudioAsyncJob(ctx context.Context, jobID string, updatedAt time.Time) error {
	r.heartbeatCalls.Add(1)
	if r.failHeartbeat || r.alwaysFailHeartbeat.Load() {
		return errors.New("heartbeat temporarily unavailable")
	}
	for {
		remaining := r.periodicFailures.Load()
		if remaining <= 0 || !r.periodicFailures.CompareAndSwap(remaining, remaining-1) {
			break
		}
		return errors.New("heartbeat temporarily unavailable")
	}
	return r.StudioAsyncJobRepository.HeartbeatStudioAsyncJob(ctx, jobID, updatedAt)
}

func TestRunStudioAsyncJobCancelsWhenLastSuccessfulHeartbeatExpires(t *testing.T) {
	initial := time.Now().UTC()
	var clock atomic.Int64
	clock.Store(initial.UnixNano())
	heartbeatNow := func() time.Time {
		return time.Unix(0, clock.Load()).UTC()
	}
	ctx := listingkit.WithRequestIdentity(listingkit.WithTenantID(context.Background(), "tenant-heartbeat-expiry"), listingkit.RequestIdentity{TenantID: "tenant-heartbeat-expiry", UserID: "user-heartbeat-expiry"})
	baseRepo := listingkit.NewMemStudioAsyncJobRepository()
	if err := baseRepo.CreateStudioAsyncJob(ctx, &listingkit.StudioAsyncJobRecord{
		ID: "heartbeat-expiry-job", TenantID: "tenant-heartbeat-expiry", UserID: "user-heartbeat-expiry", Path: "/studio/product-images",
		Status: listingkit.StudioAsyncJobStatusRunning, CreatedAt: initial, UpdatedAt: initial,
	}); err != nil {
		t.Fatalf("CreateStudioAsyncJob() error = %v", err)
	}
	repo := &failingHeartbeatStudioAsyncJobRepository{StudioAsyncJobRepository: baseRepo}
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	media := &blockingStudioAsyncMediaService{stubStudioMediaHandlerService: &stubStudioMediaHandlerService{}, started: started, release: release}
	h := &handler{
		studioAsyncJobs:                 &studioAsyncJobStore{repo: repo},
		studioMediaService:              media,
		studioAsyncJobHeartbeatInterval: time.Millisecond,
		studioAsyncJobHeartbeatNow:      heartbeatNow,
	}
	done := make(chan struct{})
	go func() {
		h.runStudioAsyncJob(ctx, "heartbeat-expiry-job", "/studio/product-images", json.RawMessage(`{}`), "", "", "", "")
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("product-image generation did not start")
	}
	repo.alwaysFailHeartbeat.Store(true)
	deadline := time.Now().Add(time.Second)
	for repo.heartbeatCalls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if repo.heartbeatCalls.Load() < 2 {
		t.Fatal("heartbeat loop did not perform its first periodic write")
	}
	clock.Store(initial.Add(studioAsyncJobHeartbeatFailureRecoveryAfter).UnixNano())
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runStudioAsyncJob continued after the last successful heartbeat expired")
	}
	job, err := baseRepo.GetStudioAsyncJob(ctx, "heartbeat-expiry-job")
	if err != nil {
		t.Fatalf("GetStudioAsyncJob() error = %v", err)
	}
	if job.Status != listingkit.StudioAsyncJobStatusFailed {
		t.Fatalf("job status = %q, want failed after heartbeat lease expiry", job.Status)
	}
}

func TestRunStudioAsyncJobRetriesTransientHeartbeatFailure(t *testing.T) {
	ctx := listingkit.WithRequestIdentity(listingkit.WithTenantID(context.Background(), "tenant-heartbeat-retry"), listingkit.RequestIdentity{TenantID: "tenant-heartbeat-retry", UserID: "user-heartbeat-retry"})
	baseRepo := listingkit.NewMemStudioAsyncJobRepository()
	if err := baseRepo.CreateStudioAsyncJob(ctx, &listingkit.StudioAsyncJobRecord{
		ID: "heartbeat-retry-job", TenantID: "tenant-heartbeat-retry", UserID: "user-heartbeat-retry", Path: "/studio/product-images",
		Status: listingkit.StudioAsyncJobStatusRunning, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateStudioAsyncJob() error = %v", err)
	}
	repo := &failingHeartbeatStudioAsyncJobRepository{StudioAsyncJobRepository: baseRepo}
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	media := &blockingStudioAsyncMediaService{stubStudioMediaHandlerService: &stubStudioMediaHandlerService{}, started: started, release: release}
	h := &handler{
		studioAsyncJobs:                 &studioAsyncJobStore{repo: repo},
		studioMediaService:              media,
		studioAsyncJobHeartbeatInterval: 5 * time.Millisecond,
	}
	done := make(chan struct{})
	go func() {
		h.runStudioAsyncJob(ctx, "heartbeat-retry-job", "/studio/product-images", json.RawMessage(`{}`), "", "", "", "")
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("product-image generation did not start")
	}
	repo.periodicFailures.Store(1)
	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("runStudioAsyncJob stopped after a transient heartbeat failure")
	default:
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runStudioAsyncJob did not finish after provider completion")
	}
	job, err := baseRepo.GetStudioAsyncJob(ctx, "heartbeat-retry-job")
	if err != nil {
		t.Fatalf("GetStudioAsyncJob() error = %v", err)
	}
	if job.Status != listingkit.StudioAsyncJobStatusSucceeded {
		t.Fatalf("job status = %q, want succeeded after transient heartbeat failure", job.Status)
	}
}

func TestRunStudioAsyncJobStopsWhenInitialHeartbeatFails(t *testing.T) {
	ctx := listingkit.WithRequestIdentity(listingkit.WithTenantID(context.Background(), "tenant-heartbeat-failure"), listingkit.RequestIdentity{TenantID: "tenant-heartbeat-failure", UserID: "user-heartbeat-failure"})
	baseRepo := listingkit.NewMemStudioAsyncJobRepository()
	if err := baseRepo.CreateStudioAsyncJob(ctx, &listingkit.StudioAsyncJobRecord{
		ID: "heartbeat-failure-job", TenantID: "tenant-heartbeat-failure", UserID: "user-heartbeat-failure", Path: "/studio/product-images",
		Status: listingkit.StudioAsyncJobStatusRunning, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateStudioAsyncJob() error = %v", err)
	}
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	media := &blockingStudioAsyncMediaService{stubStudioMediaHandlerService: &stubStudioMediaHandlerService{}, started: started, release: release}
	h := &handler{
		studioAsyncJobs:    &studioAsyncJobStore{repo: &failingHeartbeatStudioAsyncJobRepository{StudioAsyncJobRepository: baseRepo, failHeartbeat: true}},
		studioMediaService: media,
	}
	done := make(chan struct{})
	go func() {
		h.runStudioAsyncJob(ctx, "heartbeat-failure-job", "/studio/product-images", json.RawMessage(`{}`), "", "", "", "")
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("product-image generation did not start")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runStudioAsyncJob continued after heartbeat failure")
	}
	job, err := baseRepo.GetStudioAsyncJob(ctx, "heartbeat-failure-job")
	if err != nil {
		t.Fatalf("GetStudioAsyncJob() error = %v", err)
	}
	if job.Status != listingkit.StudioAsyncJobStatusFailed {
		t.Fatalf("job status = %q, want failed after heartbeat loss", job.Status)
	}
}

type orderedStudioAsyncJobRepository struct {
	listingkit.StudioAsyncJobRepository
	order *[]string
}

func (r *orderedStudioAsyncJobRepository) UpdateStudioAsyncJob(ctx context.Context, record *listingkit.StudioAsyncJobRecord) error {
	if record != nil {
		switch record.Status {
		case listingkit.StudioAsyncJobStatusSucceeded:
			*r.order = append(*r.order, "job_succeeded")
		case listingkit.StudioAsyncJobStatusFailed:
			*r.order = append(*r.order, "job_failed")
		}
	}
	return r.StudioAsyncJobRepository.UpdateStudioAsyncJob(ctx, record)
}

type rejectingStudioAsyncSuccessRepository struct {
	listingkit.StudioAsyncJobRepository
	order      *[]string
	rejectFail bool
}

func (r *rejectingStudioAsyncSuccessRepository) UpdateStudioAsyncJob(ctx context.Context, record *listingkit.StudioAsyncJobRecord) error {
	if record != nil && record.Status == listingkit.StudioAsyncJobStatusSucceeded {
		return errors.New("persist succeeded job")
	}
	if record != nil && record.Status == listingkit.StudioAsyncJobStatusFailed {
		if r.rejectFail {
			return errors.New("persist failed job")
		}
		*r.order = append(*r.order, "job_failed")
	}
	return r.StudioAsyncJobRepository.UpdateStudioAsyncJob(ctx, record)
}

type orderedStudioUsageLedger struct {
	listingsubscription.UsageLedger
	order *[]string
}

func (l *orderedStudioUsageLedger) GetByID(ctx context.Context, eventID string) (listingsubscription.UsageEvent, error) {
	return l.UsageLedger.(listingsubscription.UsageLedgerEventLookup).GetByID(ctx, eventID)
}

func (l *orderedStudioUsageLedger) UpdateMetadata(ctx context.Context, eventID string, metadata map[string]string) (listingsubscription.UsageEvent, error) {
	return l.UsageLedger.(listingsubscription.UsageLedgerMetadataUpdater).UpdateMetadata(ctx, eventID, metadata)
}

func (l *orderedStudioUsageLedger) Commit(ctx context.Context, eventID string) (listingsubscription.UsageEvent, error) {
	*l.order = append(*l.order, "usage_committed")
	return l.UsageLedger.Commit(ctx, eventID)
}

func (l *orderedStudioUsageLedger) Release(ctx context.Context, eventID, reason string) (listingsubscription.UsageEvent, error) {
	*l.order = append(*l.order, "usage_released")
	return l.UsageLedger.Release(ctx, eventID, reason)
}

func TestRunStudioAsyncJobPersistsSuccessBeforeCommittingUsage(t *testing.T) {
	ctx := listingkit.WithRequestIdentity(listingkit.WithTenantID(context.Background(), "tenant-success-order"), listingkit.RequestIdentity{TenantID: "tenant-success-order", UserID: "user-success-order"})
	repo := listingsubscription.NewMemRepository()
	baseLedger := listingsubscription.NewMemUsageLedger(repo)
	order := make([]string, 0, 2)
	ledger := &orderedStudioUsageLedger{UsageLedger: baseLedger, order: &order}
	svc, err := listingsubscription.NewServiceWithLedger(repo, ledger)
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	if _, err := svc.UpsertEntitlement(ctx, "tenant-success-order", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive, Limits: map[string]int{"product_image_jobs": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
	reserved, err := svc.ReserveUsage(ctx, listingsubscription.ReserveUsageInput{
		TenantID: "tenant-success-order", ModuleCode: listingsubscription.ModuleStudio,
		Metric: "product_image_jobs_succeeded", Quantity: 1, PeriodKey: time.Now().UTC().Format("2006-01"),
		SourceType: "listingkit_async_product_image", SourceID: "success-order-job",
		IdempotencyKey: "success-order-job", OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}
	baseJobRepo := listingkit.NewMemStudioAsyncJobRepository()
	jobRepo := &orderedStudioAsyncJobRepository{StudioAsyncJobRepository: baseJobRepo, order: &order}
	if err := jobRepo.CreateStudioAsyncJob(ctx, &listingkit.StudioAsyncJobRecord{
		ID: "success-order-job", TenantID: "tenant-success-order", UserID: "user-success-order",
		Path: "/studio/product-images", Status: listingkit.StudioAsyncJobStatusRunning,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateStudioAsyncJob() error = %v", err)
	}
	h := &handler{
		subscriptionDependencies: subscriptionDependencies{subscriptionService: svc},
		studioAsyncJobs:          &studioAsyncJobStore{repo: jobRepo},
		studioMediaService:       &stubStudioMediaHandlerService{studioProductImages: &listingkit.StudioProductImageResponse{}},
	}
	h.runStudioAsyncJob(ctx, "success-order-job", "/studio/product-images", json.RawMessage(`{}`), "", "", "", reserved.Event.EventID)
	if len(order) != 2 || order[0] != "job_succeeded" || order[1] != "usage_committed" {
		t.Fatalf("durable success order = %v, want [job_succeeded usage_committed]", order)
	}
}

func TestRunStudioAsyncJobPersistsFailureBeforeReleasingUsage(t *testing.T) {
	ctx := listingkit.WithRequestIdentity(listingkit.WithTenantID(context.Background(), "tenant-failure-order"), listingkit.RequestIdentity{TenantID: "tenant-failure-order", UserID: "user-failure-order"})
	repo := listingsubscription.NewMemRepository()
	baseLedger := listingsubscription.NewMemUsageLedger(repo)
	order := make([]string, 0, 2)
	ledger := &orderedStudioUsageLedger{UsageLedger: baseLedger, order: &order}
	svc, err := listingsubscription.NewServiceWithLedger(repo, ledger)
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	if _, err := svc.UpsertEntitlement(ctx, "tenant-failure-order", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive, Limits: map[string]int{"product_image_jobs": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
	reserved, err := svc.ReserveUsage(ctx, listingsubscription.ReserveUsageInput{
		TenantID: "tenant-failure-order", ModuleCode: listingsubscription.ModuleStudio,
		Metric: studioProductImageLedgerMetric, Quantity: 1, PeriodKey: time.Now().UTC().Format("2006-01"),
		SourceType: studioProductImageAsyncSourceType, SourceID: "failure-order-job",
		IdempotencyKey: "failure-order-job", OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}
	baseJobRepo := listingkit.NewMemStudioAsyncJobRepository()
	jobRepo := &orderedStudioAsyncJobRepository{StudioAsyncJobRepository: baseJobRepo, order: &order}
	if err := jobRepo.CreateStudioAsyncJob(ctx, &listingkit.StudioAsyncJobRecord{
		ID: "failure-order-job", TenantID: "tenant-failure-order", UserID: "user-failure-order",
		Path: "/studio/product-images", Status: listingkit.StudioAsyncJobStatusRunning,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateStudioAsyncJob() error = %v", err)
	}
	h := &handler{
		subscriptionDependencies: subscriptionDependencies{subscriptionService: svc},
		studioAsyncJobs:          &studioAsyncJobStore{repo: jobRepo},
		studioMediaService:       &stubStudioMediaHandlerService{err: errors.New("generation failed")},
	}
	h.runStudioAsyncJob(ctx, "failure-order-job", "/studio/product-images", json.RawMessage(`{}`), "", "", "", reserved.Event.EventID)
	if len(order) != 2 || order[0] != "job_failed" || order[1] != "usage_released" {
		t.Fatalf("durable failure order = %v, want [job_failed usage_released]", order)
	}
}

func TestRunStudioAsyncJobPersistsFailureAfterSuccessPersistenceFailsBeforeReleasingUsage(t *testing.T) {
	ctx := listingkit.WithRequestIdentity(listingkit.WithTenantID(context.Background(), "tenant-success-persist-failure"), listingkit.RequestIdentity{TenantID: "tenant-success-persist-failure", UserID: "user-success-persist-failure"})
	repo := listingsubscription.NewMemRepository()
	baseLedger := listingsubscription.NewMemUsageLedger(repo)
	order := make([]string, 0, 2)
	ledger := &orderedStudioUsageLedger{UsageLedger: baseLedger, order: &order}
	svc, err := listingsubscription.NewServiceWithLedger(repo, ledger)
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	if _, err := svc.UpsertEntitlement(ctx, "tenant-success-persist-failure", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive, Limits: map[string]int{"product_image_jobs": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
	reserved, err := svc.ReserveUsage(ctx, listingsubscription.ReserveUsageInput{
		TenantID: "tenant-success-persist-failure", ModuleCode: listingsubscription.ModuleStudio,
		Metric: studioProductImageLedgerMetric, Quantity: 1, PeriodKey: time.Now().UTC().Format("2006-01"),
		SourceType: studioProductImageAsyncSourceType, SourceID: "success-persist-failure-job",
		IdempotencyKey: "success-persist-failure-job", OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}
	baseJobRepo := listingkit.NewMemStudioAsyncJobRepository()
	jobRepo := &rejectingStudioAsyncSuccessRepository{StudioAsyncJobRepository: baseJobRepo, order: &order}
	if err := jobRepo.CreateStudioAsyncJob(ctx, &listingkit.StudioAsyncJobRecord{
		ID: "success-persist-failure-job", TenantID: "tenant-success-persist-failure", UserID: "user-success-persist-failure",
		Path: "/studio/product-images", Status: listingkit.StudioAsyncJobStatusRunning,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateStudioAsyncJob() error = %v", err)
	}
	h := &handler{
		subscriptionDependencies: subscriptionDependencies{subscriptionService: svc},
		studioAsyncJobs:          &studioAsyncJobStore{repo: jobRepo},
		studioMediaService:       &stubStudioMediaHandlerService{studioProductImages: &listingkit.StudioProductImageResponse{}},
	}
	h.runStudioAsyncJob(ctx, "success-persist-failure-job", "/studio/product-images", json.RawMessage(`{}`), "", "", "", reserved.Event.EventID)
	if len(order) != 2 || order[0] != "job_failed" || order[1] != "usage_released" {
		t.Fatalf("success persistence failure order = %v, want [job_failed usage_released]", order)
	}
}

func TestRunStudioAsyncJobRetainsUsageWhenSuccessAndFailurePersistenceFail(t *testing.T) {
	ctx := listingkit.WithRequestIdentity(listingkit.WithTenantID(context.Background(), "tenant-terminal-persist-failure"), listingkit.RequestIdentity{TenantID: "tenant-terminal-persist-failure", UserID: "user-terminal-persist-failure"})
	repo := listingsubscription.NewMemRepository()
	baseLedger := listingsubscription.NewMemUsageLedger(repo)
	order := make([]string, 0, 1)
	ledger := &orderedStudioUsageLedger{UsageLedger: baseLedger, order: &order}
	svc, err := listingsubscription.NewServiceWithLedger(repo, ledger)
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	if _, err := svc.UpsertEntitlement(ctx, "tenant-terminal-persist-failure", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive, Limits: map[string]int{"product_image_jobs": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
	reserved, err := svc.ReserveUsage(ctx, listingsubscription.ReserveUsageInput{
		TenantID: "tenant-terminal-persist-failure", ModuleCode: listingsubscription.ModuleStudio,
		Metric: studioProductImageLedgerMetric, Quantity: 1, PeriodKey: time.Now().UTC().Format("2006-01"),
		SourceType: studioProductImageAsyncSourceType, SourceID: "terminal-persist-failure-job",
		IdempotencyKey: "terminal-persist-failure-job", OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}
	baseJobRepo := listingkit.NewMemStudioAsyncJobRepository()
	jobRepo := &rejectingStudioAsyncSuccessRepository{StudioAsyncJobRepository: baseJobRepo, order: &order, rejectFail: true}
	if err := jobRepo.CreateStudioAsyncJob(ctx, &listingkit.StudioAsyncJobRecord{
		ID: "terminal-persist-failure-job", TenantID: "tenant-terminal-persist-failure", UserID: "user-terminal-persist-failure",
		Path: "/studio/product-images", Status: listingkit.StudioAsyncJobStatusRunning,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateStudioAsyncJob() error = %v", err)
	}
	h := &handler{
		subscriptionDependencies: subscriptionDependencies{subscriptionService: svc},
		studioAsyncJobs:          &studioAsyncJobStore{repo: jobRepo},
		studioMediaService:       &stubStudioMediaHandlerService{studioProductImages: &listingkit.StudioProductImageResponse{}},
	}
	h.runStudioAsyncJob(ctx, "terminal-persist-failure-job", "/studio/product-images", json.RawMessage(`{}`), "", "", "", reserved.Event.EventID)
	if len(order) != 0 {
		t.Fatalf("usage should remain reserved when terminal persistence fails, order = %v", order)
	}
	event, err := baseLedger.(listingsubscription.UsageLedgerEventLookup).GetByID(ctx, reserved.Event.EventID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if event.Status != listingsubscription.UsageEventReserved {
		t.Fatalf("usage status = %q, want reserved for reconciliation", event.Status)
	}
}

func TestStartStudioAsyncJobUsesSharedStudioBatchExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &stubStudioMediaHandlerService{
		studioDesigns: &listingkit.StudioDesignResponse{
			Prompt: "inline service should not run",
			Images: []listingkit.StudioGeneratedImage{{
				ID:       "design-inline",
				ImageURL: "https://example.com/inline.png",
			}},
		},
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithStudioMediaService(svc), WithStudioSessionAsyncJobService(svc), WithSubscriptionService(activeStudioSubscriptionService(t)))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	originalExecute := executeStudioDesignBatch
	executeStudioDesignBatch = func(ctx context.Context, service listingkit.StudioMediaService, input listingkit.StudioBatchGenerateExecutionInput) (*listingkit.StudioBatchGenerateExecutionOutput, error) {
		if service != svc {
			t.Fatalf("service = %T, want stubStudioMediaHandlerService", service)
		}
		if input.Request == nil || input.Request.Prompt != "retro cherries" {
			t.Fatalf("input request = %+v, want bound prompt", input.Request)
		}
		return &listingkit.StudioBatchGenerateExecutionOutput{
			Response: &listingkit.StudioDesignResponse{
				Prompt: "shared seam response",
				Images: []listingkit.StudioGeneratedImage{{
					ID:       "design-shared",
					ImageURL: "/uploads/design-shared.png",
				}},
			},
			SessionID: input.SessionID,
		}, nil
	}
	t.Cleanup(func() {
		executeStudioDesignBatch = originalExecute
	})

	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/async-jobs", h.StartStudioAsyncJob)
	router.GET("/api/v1/listing-kits/studio/async-jobs/:job_id", h.GetStudioAsyncJob)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/async-jobs", strings.NewReader(`{"path":"/studio/designs","body":{"prompt":"retro cherries","count":1}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("start status = %d, want 202 body=%s", resp.Code, resp.Body.String())
	}

	var started struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &started); err != nil {
		t.Fatalf("unmarshal start body: %v", err)
	}

	var polled struct {
		Status string                           `json:"status"`
		Result *listingkit.StudioDesignResponse `json:"result"`
	}
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/studio/async-jobs/"+started.JobID, nil)
		resp = httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("poll status = %d, want 200 body=%s", resp.Code, resp.Body.String())
		}
		if err := json.Unmarshal(resp.Body.Bytes(), &polled); err != nil {
			t.Fatalf("unmarshal poll body: %v", err)
		}
		if polled.Status == "succeeded" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if polled.Status != "succeeded" || polled.Result == nil || polled.Result.Prompt != "shared seam response" {
		t.Fatalf("polled = %+v, want shared seam result", polled)
	}
	if svc.studioDesignReq != nil {
		t.Fatalf("studio design req = %+v, want handler to avoid inline service execution", svc.studioDesignReq)
	}
}

func activeStudioSubscriptionService(t *testing.T) *listingsubscription.Service {
	t.Helper()
	svc, err := listingsubscription.NewService(listingsubscription.NewMemRepository())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpsertEntitlement(t.Context(), listingkit.DefaultTenantID, listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{Status: listingsubscription.StatusActive}); err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestStudioAsyncJobRejectsUnknownPath(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubHandlerCoreService{})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/async-jobs", h.StartStudioAsyncJob)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/async-jobs", strings.NewReader(`{"path":"/unknown","body":{}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestStudioAsyncJobRequiresStudioSubscription(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	subscriptionService, err := listingsubscription.NewService(listingsubscription.NewMemRepository())
	if err != nil {
		t.Fatalf("create subscription service: %v", err)
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithSubscriptionService(subscriptionService))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/async-jobs", h.StartStudioAsyncJob)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/async-jobs", strings.NewReader(`{"path":"/studio/designs","body":{"prompt":"retro cherries","count":1}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402 body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"error":"subscription_required"`) {
		t.Fatalf("body = %s, want subscription_required", resp.Body.String())
	}
}

func TestStudioAsyncJobReturnsNotFoundForMissingJob(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubHandlerCoreService{})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	router := gin.New()
	router.GET("/api/v1/listing-kits/studio/async-jobs/:job_id", h.GetStudioAsyncJob)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/studio/async-jobs/missing", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.Code)
	}
}

func TestStudioAsyncJobSyncsSessionWhenDesignJobStarts(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubStudioMediaHandlerService{
		studioDesigns: &listingkit.StudioDesignResponse{
			Prompt: "retro cherries",
			Images: []listingkit.StudioGeneratedImage{{
				ID:       "design-1",
				ImageURL: "https://example.com/design.png",
			}},
		},
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithStudioMediaService(svc), WithStudioSessionAsyncJobService(svc), WithSubscriptionService(activeStudioSubscriptionService(t)))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/async-jobs", h.StartStudioAsyncJob)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/async-jobs", strings.NewReader(`{"path":"/studio/designs","session_id":"session-1","body":{"prompt":"retro cherries","count":1}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 body=%s", resp.Code, resp.Body.String())
	}

	if svc.updatedStudioSessionID != "session-1" {
		t.Fatalf("updated session id = %q, want session-1", svc.updatedStudioSessionID)
	}
	if svc.updatedStudioSessionReq == nil || svc.updatedStudioSessionReq.Status == nil {
		t.Fatalf("updated session req = %+v, want synced session status", svc.updatedStudioSessionReq)
	}
	if got := *svc.updatedStudioSessionReq.Status; got != listingkit.SheinStudioSessionStatusGenerating && got != listingkit.SheinStudioSessionStatusGenerated {
		t.Fatalf("session status = %q, want generating/generated", got)
	}
	if svc.updatedStudioSessionReq.GenerationJobID == nil || strings.TrimSpace(*svc.updatedStudioSessionReq.GenerationJobID) == "" {
		t.Fatalf("generation job id = %+v, want non-empty", svc.updatedStudioSessionReq.GenerationJobID)
	}
}

func TestStudioAsyncJobPropagatesTraceContextToDesignExecution(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubStudioMediaHandlerService{
		studioDesigns: &listingkit.StudioDesignResponse{
			Prompt: "retro cherries",
			Images: []listingkit.StudioGeneratedImage{{
				ID:       "design-1",
				ImageURL: "https://example.com/design.png",
			}},
		},
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithStudioMediaService(svc), WithStudioSessionAsyncJobService(svc), WithSubscriptionService(activeStudioSubscriptionService(t)))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/async-jobs", h.StartStudioAsyncJob)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/async-jobs", strings.NewReader(`{"path":"/studio/designs","session_id":"session-1","body":{"prompt":"retro cherries","count":1}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ListingKit-Batch-Run-Id", "run-1")
	req.Header.Set("X-ListingKit-Batch-Id", "batch-1")
	req.Header.Set("X-ListingKit-Studio-Session-Id", "session-1")
	req.Header.Set("X-ListingKit-Queue-Mode", "generate")
	req.Header.Set("X-ListingKit-Queue-Index", "2")
	req.Header.Set("X-ListingKit-Queue-Total", "5")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 body=%s", resp.Code, resp.Body.String())
	}

	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		if svc.studioDesignCtx != nil && svc.updatedStudioSessionCtx != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if svc.studioDesignCtx == nil {
		t.Fatal("studio design ctx was not captured")
	}
	if svc.updatedStudioSessionCtx == nil {
		t.Fatal("updated studio session ctx was not captured")
	}

	trace := listingkit.RequestTraceFromContext(svc.studioDesignCtx)
	if trace.BatchRunID != "run-1" || trace.BatchID != "batch-1" || trace.SessionID != "session-1" {
		t.Fatalf("studio design trace = %+v, want run/batch/session ids", trace)
	}
	if trace.QueueMode != "generate" || trace.QueueIndex != 2 || trace.QueueTotal != 5 {
		t.Fatalf("studio design trace = %+v, want queue fields", trace)
	}

	sessionTrace := listingkit.RequestTraceFromContext(svc.updatedStudioSessionCtx)
	if sessionTrace.BatchRunID != "run-1" || sessionTrace.SessionID != "session-1" {
		t.Fatalf("updated session trace = %+v, want propagated trace fields", sessionTrace)
	}
}
