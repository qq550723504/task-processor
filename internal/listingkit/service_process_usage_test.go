package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/listingkit/core"
	"task-processor/internal/listingsubscription"
	"task-processor/internal/productenrich"
)

type recordingGenerationUsageSettlement struct {
	calls             []string
	reserveErr        error
	commitErr         error
	releaseErr        error
	reserveCommitted  bool
	repo              *stubProcessStatusRepo
	reservedTaskID    string
	reservedTenantID  string
	committedTaskID   string
	committedTenantID string
	releasedTaskID    string
	releasedTenantID  string
	releasedReason    string
	reservedAt        time.Time
}

type generationUsageTestAdmission struct {
	tenantIDs map[string]struct{}
}

func (a generationUsageTestAdmission) AllowsGenerationUsage(tenantID string) bool {
	_, ok := a.tenantIDs[tenantID]
	return ok
}

func (s *recordingGenerationUsageSettlement) ReserveGeneration(_ context.Context, tenantID string, taskID string, occurredAt time.Time) (GenerationUsageReservation, error) {
	s.calls = append(s.calls, "reserve")
	s.reservedTaskID = taskID
	s.reservedTenantID = tenantID
	s.reservedAt = occurredAt
	if s.reserveErr != nil {
		return GenerationUsageReservation{}, s.reserveErr
	}
	return GenerationUsageReservation{EventID: "usage-" + taskID, AlreadyCommitted: s.reserveCommitted}, nil
}

func (s *recordingGenerationUsageSettlement) CommitGeneration(_ context.Context, tenantID string, taskID string) error {
	s.calls = append(s.calls, "commit")
	s.committedTaskID = taskID
	s.committedTenantID = tenantID
	if s.repo != nil && s.repo.task.Status != core.TaskStatusCompleted && s.repo.task.Status != core.TaskStatusNeedsReview {
		return errors.New("commit called before terminal task persistence")
	}
	return s.commitErr
}

func (s *recordingGenerationUsageSettlement) ReleaseGeneration(_ context.Context, tenantID string, taskID, reason string) error {
	s.calls = append(s.calls, "release")
	s.releasedTaskID = taskID
	s.releasedTenantID = tenantID
	s.releasedReason = reason
	return s.releaseErr
}

type processUsageProductService struct {
	task         *productenrich.Task
	product      *productenrich.ProductJSON
	processErr   error
	processCalls int
}

func (s *processUsageProductService) CreateGenerateTask(context.Context, *productenrich.GenerateRequest) (*productenrich.Task, error) {
	return s.task, nil
}

func (s *processUsageProductService) GetTaskResult(context.Context, string) (*productenrich.TaskResult, error) {
	return nil, nil
}

func (s *processUsageProductService) ProcessProduct(context.Context, *productenrich.Task) (*productenrich.ProductJSON, error) {
	s.processCalls++
	if s.processErr != nil {
		return nil, s.processErr
	}
	return s.product, nil
}

func newProcessUsageFixture(t *testing.T, settlement GenerationUsageSettlement, productErr error) (Service, *stubProcessStatusRepo, *processUsageProductService, *Task) {
	t.Helper()
	repo := &stubProcessStatusRepo{stubGenerationRepo: &stubGenerationRepo{}}
	productService := &processUsageProductService{
		task:       &productenrich.Task{ID: "product-task-usage", Request: &productenrich.GenerateRequest{ProductURL: "https://example.com/product"}},
		product:    &productenrich.ProductJSON{Title: "Travel Bag", Category: []string{"bags"}, Attributes: map[string]string{"color": "black"}},
		processErr: productErr,
	}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestProductService(productService),
		withTestAssembler(&stubProcessStatusAssembler{result: &ListingKitResult{Shein: &SheinPackage{}, Summary: &GenerationSummary{}}}),
		withTestConfig(func(cfg *ServiceConfig) { cfg.Core.GenerationUsageLedger = settlement }),
	))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	task := &Task{ID: "listingkit-usage-1", TenantID: "tenant-17", Status: core.TaskStatusPending, Request: &GenerateRequest{ProductURL: "https://example.com/product", Platforms: []string{"shein"}}, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	return svc, repo, productService, task
}

func TestProcessListingKitReservesBeforeWorkflow(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{}
	svc, repo, productService, task := newProcessUsageFixture(t, settlement, nil)
	settlement.repo = repo

	if _, err := svc.ProcessListingKit(context.Background(), task); err != nil {
		t.Fatalf("ProcessListingKit() error = %v", err)
	}
	if productService.processCalls != 1 {
		t.Fatalf("ProcessProduct calls = %d, want 1", productService.processCalls)
	}
	if len(settlement.calls) < 2 || settlement.calls[0] != "reserve" || settlement.calls[len(settlement.calls)-1] != "commit" {
		t.Fatalf("settlement calls = %#v, want reserve before commit", settlement.calls)
	}
	if settlement.reservedTaskID != task.ID || settlement.committedTaskID != task.ID {
		t.Fatalf("settlement task IDs = (%q, %q), want %q", settlement.reservedTaskID, settlement.committedTaskID, task.ID)
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.GenerationUsageReservationState != "" || stored.GenerationUsageReservationLeaseUntil != nil {
		t.Fatalf("settled task reservation = (%q, %v), want cleared", stored.GenerationUsageReservationState, stored.GenerationUsageReservationLeaseUntil)
	}
}

func TestProcessListingKitQuotaRejectionSkipsWorkflow(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{reserveErr: listingsubscription.ErrUsageQuotaExceeded}
	svc, _, productService, task := newProcessUsageFixture(t, settlement, nil)

	if _, err := svc.ProcessListingKit(context.Background(), task); !errors.Is(err, listingsubscription.ErrUsageQuotaExceeded) {
		t.Fatalf("ProcessListingKit() error = %v, want quota error", err)
	}
	if productService.processCalls != 0 {
		t.Fatalf("ProcessProduct calls = %d, want 0 after quota rejection", productService.processCalls)
	}
	if len(settlement.calls) != 1 || settlement.calls[0] != "reserve" {
		t.Fatalf("settlement calls = %#v, want reserve only", settlement.calls)
	}
}

func TestProcessListingKitQuotaRejectionFallbackRemainsRecoverable(t *testing.T) {
	t.Parallel()

	repo := &stubProcessStatusRepo{stubGenerationRepo: &stubGenerationRepo{}, failedErrs: []error{errors.New("store unavailable"), errors.New("store unavailable"), errors.New("store unavailable")}}
	settlement := &recordingGenerationUsageSettlement{reserveErr: listingsubscription.ErrUsageQuotaExceeded}
	productService := &processUsageProductService{task: &productenrich.Task{ID: "product-task-quota-fallback", Request: &productenrich.GenerateRequest{ProductURL: "https://example.com/product"}}}
	svc, err := NewService(newTestServiceConfig(repo, withTestProductService(productService), withTestAssembler(&stubProcessStatusAssembler{result: &ListingKitResult{Shein: &SheinPackage{}, Summary: &GenerationSummary{}}}), withTestConfig(func(cfg *ServiceConfig) { cfg.Core.GenerationUsageLedger = settlement })))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	task := &Task{ID: "listingkit-quota-fallback", TenantID: "tenant-17", Status: core.TaskStatusPending, Request: &GenerateRequest{ProductURL: "https://example.com/product", Platforms: []string{"shein"}}, CreatedAt: time.Now().UTC()}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if _, err := svc.ProcessListingKit(context.Background(), task); !errors.Is(err, listingsubscription.ErrUsageQuotaExceeded) {
		t.Fatalf("ProcessListingKit() error = %v, want quota error", err)
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != core.TaskStatusBlockedRetryable || stored.RetryableBlock == nil || stored.RetryableBlock.ReasonCode != terminalPersistencePendingReason {
		t.Fatalf("stored task = %#v, want terminal persistence fallback block", stored)
	}
}

func TestProcessListingKitCommittedReplayFallbackRemainsRecoverable(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{reserveCommitted: true}
	svc, repo, _, task := newProcessUsageFixture(t, settlement, nil)
	repo.completedErr = errors.New("task store unavailable")
	task.Result = &ListingKitResult{Status: string(core.TaskStatusCompleted), Summary: &GenerationSummary{}}
	repo.task.Result = task.Result
	if _, err := svc.ProcessListingKit(context.Background(), task); err == nil {
		t.Fatal("ProcessListingKit() error = nil, want committed replay persistence error")
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != core.TaskStatusBlockedRetryable || stored.RetryableBlock == nil || stored.RetryableBlock.ReasonCode != committedReplayPersistencePendingReason {
		t.Fatalf("stored task = %#v, want committed replay persistence fallback block", stored)
	}
}

func TestProcessListingKitNormalizesBlankTenantForUsageSettlement(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{}
	svc, _, _, task := newProcessUsageFixture(t, settlement, nil)
	task.TenantID = ""
	if _, err := svc.ProcessListingKit(context.Background(), task); err != nil {
		t.Fatalf("ProcessListingKit() error = %v", err)
	}
	if settlement.reservedTenantID != DefaultTenantID || settlement.committedTenantID != DefaultTenantID {
		t.Fatalf("usage tenant IDs = (%q, %q), want default %q", settlement.reservedTenantID, settlement.committedTenantID, DefaultTenantID)
	}
}

func TestProcessListingKitUsesBillingTenantForUsageSettlement(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{}
	svc, _, _, task := newProcessUsageFixture(t, settlement, nil)
	task.TenantID = "zitadel-tenant"
	task.BillingTenantID = "227"
	if _, err := svc.ProcessListingKit(context.Background(), task); err != nil {
		t.Fatalf("ProcessListingKit() error = %v", err)
	}
	if settlement.reservedTenantID != "227" || settlement.committedTenantID != "227" {
		t.Fatalf("usage tenant IDs = (%q, %q), want billing tenant 227", settlement.reservedTenantID, settlement.committedTenantID)
	}
}

func TestProcessListingKitKeepsLegacyUsageOutsideGenerationLedgerCohort(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{}
	_, repo, productService, task := newProcessUsageFixture(t, settlement, nil)
	configured, err := NewService(newTestServiceConfig(
		repo,
		withTestProductService(productService),
		withTestAssembler(&stubProcessStatusAssembler{result: &ListingKitResult{Shein: &SheinPackage{}, Summary: &GenerationSummary{}}}),
		withTestConfig(func(cfg *ServiceConfig) {
			cfg.Core.GenerationUsageLedger = settlement
			cfg.Core.GenerationUsageAdmission = generationUsageTestAdmission{tenantIDs: map[string]struct{}{"tenant-selected": {}}}
		}),
	))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if _, err := configured.ProcessListingKit(context.Background(), task); err != nil {
		t.Fatalf("ProcessListingKit() error = %v", err)
	}
	if productService.processCalls != 1 {
		t.Fatalf("ProcessProduct calls = %d, want legacy workflow execution", productService.processCalls)
	}
	if len(settlement.calls) != 0 {
		t.Fatalf("settlement calls = %#v, want none outside the configured cohort", settlement.calls)
	}
}

func TestProcessListingKitReplaysExistingGenerationReservationAfterCohortNarrows(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{}
	_, repo, productService, task := newProcessUsageFixture(t, settlement, nil)
	task.BillingTenantID = "billing-selected"
	task.GenerationUsageReservationState = GenerationUsageReservationStatePending
	configured, err := NewService(newTestServiceConfig(
		repo,
		withTestProductService(productService),
		withTestAssembler(&stubProcessStatusAssembler{result: &ListingKitResult{Shein: &SheinPackage{}, Summary: &GenerationSummary{}}}),
		withTestConfig(func(cfg *ServiceConfig) {
			cfg.Core.GenerationUsageLedger = settlement
			cfg.Core.GenerationUsageAdmission = generationUsageTestAdmission{tenantIDs: map[string]struct{}{}}
		}),
	))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if _, err := configured.ProcessListingKit(context.Background(), task); err != nil {
		t.Fatalf("ProcessListingKit() error = %v", err)
	}
	if len(settlement.calls) == 0 || settlement.calls[0] != "reserve" {
		t.Fatalf("settlement calls = %#v, want existing reservation replayed", settlement.calls)
	}
}

func TestProcessListingKitKeepsBlankBillingTenantOnLegacyPath(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{}
	_, repo, productService, task := newProcessUsageFixture(t, settlement, nil)
	configured, err := NewService(newTestServiceConfig(
		repo,
		withTestProductService(productService),
		withTestAssembler(&stubProcessStatusAssembler{result: &ListingKitResult{Shein: &SheinPackage{}, Summary: &GenerationSummary{}}}),
		withTestConfig(func(cfg *ServiceConfig) {
			cfg.Core.GenerationUsageLedger = settlement
			cfg.Core.GenerationUsageAdmission = generationUsageTestAdmission{tenantIDs: map[string]struct{}{"tenant-17": {}}}
		}),
	))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if _, err := configured.ProcessListingKit(context.Background(), task); err != nil {
		t.Fatalf("ProcessListingKit() error = %v", err)
	}
	if len(settlement.calls) != 0 {
		t.Fatalf("settlement calls = %#v, want no new usage for blank billing tenant", settlement.calls)
	}
}

func TestProcessListingKitKeepsLegacyRetryPersistenceOutsideGenerationUsageCohort(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{}
	_, repo, productService, task := newProcessUsageFixture(t, settlement, errors.New("OpenAI API error: insufficient credits in account balance"))
	configured, err := NewService(newTestServiceConfig(
		repo,
		withTestProductService(productService),
		withTestAssembler(&stubProcessStatusAssembler{result: &ListingKitResult{Shein: &SheinPackage{}, Summary: &GenerationSummary{}}}),
		withTestConfig(func(cfg *ServiceConfig) {
			cfg.Core.GenerationUsageLedger = settlement
			cfg.Core.GenerationUsageAdmission = generationUsageTestAdmission{tenantIDs: map[string]struct{}{}}
		}),
	))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if _, err := configured.ProcessListingKit(context.Background(), task); err == nil {
		t.Fatal("ProcessListingKit() error = nil, want retryable provider failure")
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.RetryableBlock == nil || stored.RetryableBlock.NextRetryAt != nil {
		t.Fatalf("retryable block = %#v, want legacy unscheduled retry block", stored.RetryableBlock)
	}
}

func TestGenerationUsageReservationLeaseRenewerExtendsDurableLease(t *testing.T) {
	t.Parallel()

	repo := &stubProcessStatusRepo{stubGenerationRepo: &stubGenerationRepo{generationUsageRenewed: make(chan struct{}, 1)}}
	task := &Task{
		ID:                                   "generation-usage-lease-renewal",
		TenantID:                             "tenant-17",
		Status:                               core.TaskStatusProcessing,
		GenerationUsageReservationState:      GenerationUsageReservationStateReserved,
		GenerationUsageReservationLeaseUntil: timePointer(time.Now().UTC().Add(time.Minute)),
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	before := *repo.task.GenerationUsageReservationLeaseUntil
	_, stop := startGenerationUsageReservationLeaseRenewer(context.Background(), task, repo, time.Millisecond)
	t.Cleanup(func() {
		if err := stop(); err != nil {
			t.Fatalf("stop lease renewer error = %v", err)
		}
	})

	select {
	case <-repo.generationUsageRenewed:
		if err := stop(); err != nil {
			t.Fatalf("stop lease renewer error = %v", err)
		}
		if !repo.task.GenerationUsageReservationLeaseUntil.After(before) {
			t.Fatalf("lease = %v, want a renewal after %v", repo.task.GenerationUsageReservationLeaseUntil, before)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for durable generation usage lease renewal")
	}
}

func timePointer(value time.Time) *time.Time {
	return &value
}

func TestProcessListingKitReleasesReservationOnWorkflowFailure(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{}
	svc, _, _, task := newProcessUsageFixture(t, settlement, errors.New("provider failed"))

	if _, err := svc.ProcessListingKit(context.Background(), task); err == nil {
		t.Fatal("ProcessListingKit() error = nil, want workflow failure")
	}
	if settlement.releasedTaskID != task.ID || settlement.releasedReason == "" {
		t.Fatalf("release = (%q, %q), want task and reason", settlement.releasedTaskID, settlement.releasedReason)
	}
}

func TestProcessListingKitPreservesReservationOnRetryableWorkflowFailure(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{}
	svc, repo, _, task := newProcessUsageFixture(t, settlement, errors.New("OpenAI API error: insufficient credits in account balance"))

	if _, err := svc.ProcessListingKit(context.Background(), task); err == nil {
		t.Fatal("ProcessListingKit() error = nil, want retryable workflow failure")
	}
	if settlement.releasedTaskID != "" {
		t.Fatalf("released task = %q, want empty while retryable task remains reserved", settlement.releasedTaskID)
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != core.TaskStatusBlockedRetryable || stored.RetryableBlock == nil {
		t.Fatalf("stored task = %#v, want retryable block", stored)
	}
	if stored.RetryableBlock.NextRetryAt == nil {
		t.Fatal("retryable workflow block NextRetryAt = nil, want scheduled recovery")
	}
}

func TestProcessListingKitPreservesReleaseRecoveryWhenRetryableBlockPersistenceFails(t *testing.T) {
	t.Parallel()

	repo := &stubProcessStatusRepo{
		stubGenerationRepo: &stubGenerationRepo{},
		blockedErrs: []error{
			errors.New("task store unavailable"),
			errors.New("task store unavailable"),
			errors.New("task store unavailable"),
		},
	}
	settlement := &recordingGenerationUsageSettlement{}
	productService := &processUsageProductService{
		task:       &productenrich.Task{ID: "product-task-retryable-persist", Request: &productenrich.GenerateRequest{ProductURL: "https://example.com/product"}},
		processErr: errors.New("OpenAI API error: insufficient credits in account balance"),
	}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestProductService(productService),
		withTestAssembler(&stubProcessStatusAssembler{result: &ListingKitResult{Shein: &SheinPackage{}, Summary: &GenerationSummary{}}}),
		withTestConfig(func(cfg *ServiceConfig) { cfg.Core.GenerationUsageLedger = settlement }),
	))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	task := &Task{ID: "listingkit-retryable-persist", TenantID: "tenant-17", Status: core.TaskStatusPending, Request: &GenerateRequest{ProductURL: "https://example.com/product", Platforms: []string{"shein"}}, CreatedAt: time.Now().UTC()}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	if _, err := svc.ProcessListingKit(context.Background(), task); err == nil {
		t.Fatal("ProcessListingKit() error = nil, want retryable workflow failure")
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != core.TaskStatusBlockedRetryable || stored.RetryableBlock == nil || stored.RetryableBlock.ReasonCode != usageReleasePendingReason {
		t.Fatalf("stored task = %#v, want usage_release_pending so the held reservation can drain", stored)
	}
	if settlement.releasedTaskID != "" {
		t.Fatalf("released task = %q, want durable release recovery instead of an unrecorded release", settlement.releasedTaskID)
	}
}

func TestProcessListingKitSchedulesRetryableReservationFailure(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{reserveErr: errors.New("context deadline exceeded")}
	svc, repo, productService, task := newProcessUsageFixture(t, settlement, nil)
	if _, err := svc.ProcessListingKit(context.Background(), task); err == nil {
		t.Fatal("ProcessListingKit() error = nil, want reservation failure")
	}
	if productService.processCalls != 0 {
		t.Fatalf("ProcessProduct calls = %d, want 0 after reservation failure", productService.processCalls)
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != core.TaskStatusBlockedRetryable || stored.RetryableBlock == nil || stored.RetryableBlock.NextRetryAt == nil {
		t.Fatalf("stored task = %#v, want scheduled retryable reservation block", stored)
	}
}

func TestProcessListingKitPersistsReservationFailureWithCleanupContext(t *testing.T) {
	t.Parallel()

	repo := &stubProcessStatusRepo{stubGenerationRepo: &stubGenerationRepo{}, requireLiveBlockContext: true}
	settlement := &recordingGenerationUsageSettlement{reserveErr: context.DeadlineExceeded}
	productService := &processUsageProductService{task: &productenrich.Task{ID: "product-task-reservation-canceled", Request: &productenrich.GenerateRequest{ProductURL: "https://example.com/product"}}}
	svc, err := NewService(newTestServiceConfig(repo, withTestProductService(productService), withTestAssembler(&stubProcessStatusAssembler{result: &ListingKitResult{Shein: &SheinPackage{}, Summary: &GenerationSummary{}}}), withTestConfig(func(cfg *ServiceConfig) { cfg.Core.GenerationUsageLedger = settlement })))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	task := &Task{ID: "listingkit-reservation-canceled", TenantID: "tenant-17", Status: core.TaskStatusPending, Request: &GenerateRequest{ProductURL: "https://example.com/product", Platforms: []string{"shein"}}, CreatedAt: time.Now().UTC()}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.ProcessListingKit(ctx, task); err == nil {
		t.Fatal("ProcessListingKit() error = nil, want canceled reservation failure")
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != core.TaskStatusBlockedRetryable || stored.RetryableBlock == nil || stored.RetryableBlock.NextRetryAt == nil {
		t.Fatalf("stored task = %#v, want cleanup-persisted scheduled block", stored)
	}
}

func TestProcessListingKitUsesReservationTimeForNewUsageEvents(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{}
	svc, _, _, task := newProcessUsageFixture(t, settlement, nil)
	task.CreatedAt = time.Now().UTC().Add(-45 * 24 * time.Hour)
	before := time.Now().UTC()
	if _, err := svc.ProcessListingKit(context.Background(), task); err != nil {
		t.Fatalf("ProcessListingKit() error = %v", err)
	}
	if !settlement.reservedAt.After(task.CreatedAt) || settlement.reservedAt.Before(before.Add(-time.Second)) {
		t.Fatalf("reserved_at = %v, want current reservation time after task creation %v", settlement.reservedAt, task.CreatedAt)
	}
}

func TestProcessListingKitKeepsReleaseFailureRecoverableAfterTerminalWorkflowFailure(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{releaseErr: errors.New("ledger context deadline exceeded")}
	svc, repo, _, task := newProcessUsageFixture(t, settlement, errors.New("provider rejected request"))
	if _, err := svc.ProcessListingKit(context.Background(), task); err == nil {
		t.Fatal("ProcessListingKit() error = nil, want workflow/release failure")
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != core.TaskStatusBlockedRetryable || stored.RetryableBlock == nil || stored.RetryableBlock.ReasonCode != usageReleasePendingReason {
		t.Fatalf("stored task = %#v, want usage_release_pending block", stored)
	}
}

func TestProcessListingKitPersistsReleasePendingAfterWorkflowReleaseFailure(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{releaseErr: errors.New("ledger unavailable")}
	svc, repo, _, task := newProcessUsageFixture(t, settlement, errors.New("provider rejected request"))
	if _, err := svc.ProcessListingKit(context.Background(), task); err == nil {
		t.Fatal("ProcessListingKit() error = nil, want workflow/release failure")
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != core.TaskStatusBlockedRetryable || stored.RetryableBlock == nil || stored.RetryableBlock.ReasonCode != usageReleasePendingReason {
		t.Fatalf("stored task = %#v, want usage_release_pending block", stored)
	}
	if repo.failedCalls != 0 {
		t.Fatalf("MarkFailed calls = %d, want 0 while release remains recoverable", repo.failedCalls)
	}
}

func TestProcessListingKitPersistsCommitPendingAfterCanceledContext(t *testing.T) {
	t.Parallel()

	repo := &stubProcessStatusRepo{stubGenerationRepo: &stubGenerationRepo{}, requireLiveBlockContext: true}
	settlement := &recordingGenerationUsageSettlement{commitErr: context.DeadlineExceeded}
	productService := &processUsageProductService{
		task:    &productenrich.Task{ID: "product-task-canceled-commit", Request: &productenrich.GenerateRequest{ProductURL: "https://example.com/product"}},
		product: &productenrich.ProductJSON{Title: "Travel Bag"},
	}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestProductService(productService),
		withTestAssembler(&stubProcessStatusAssembler{result: &ListingKitResult{Shein: &SheinPackage{}, Summary: &GenerationSummary{}}}),
		withTestConfig(func(cfg *ServiceConfig) { cfg.Core.GenerationUsageLedger = settlement }),
	))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	task := &Task{ID: "listingkit-canceled-commit", TenantID: "tenant-17", Status: core.TaskStatusPending, Request: &GenerateRequest{ProductURL: "https://example.com/product", Platforms: []string{"shein"}}, CreatedAt: time.Now().UTC()}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.ProcessListingKit(ctx, task); err == nil {
		t.Fatal("ProcessListingKit() error = nil, want commit failure")
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.RetryableBlock == nil || stored.RetryableBlock.ReasonCode != usageCommitPendingReason {
		t.Fatalf("stored task = %#v, want usage_commit_pending block", stored)
	}
}

func TestProcessListingKitPersistsReleasePendingAfterCanceledContext(t *testing.T) {
	t.Parallel()

	repo := &stubProcessStatusRepo{stubGenerationRepo: &stubGenerationRepo{}, completedErr: errors.New("task store unavailable"), requireLiveBlockContext: true}
	settlement := &recordingGenerationUsageSettlement{releaseErr: context.DeadlineExceeded}
	productService := &processUsageProductService{
		task:    &productenrich.Task{ID: "product-task-canceled-release", Request: &productenrich.GenerateRequest{ProductURL: "https://example.com/product"}},
		product: &productenrich.ProductJSON{Title: "Travel Bag"},
	}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestProductService(productService),
		withTestAssembler(&stubProcessStatusAssembler{result: &ListingKitResult{Shein: &SheinPackage{}, Summary: &GenerationSummary{}}}),
		withTestConfig(func(cfg *ServiceConfig) { cfg.Core.GenerationUsageLedger = settlement }),
	))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	task := &Task{ID: "listingkit-canceled-release", TenantID: "tenant-17", Status: core.TaskStatusPending, Request: &GenerateRequest{ProductURL: "https://example.com/product", Platforms: []string{"shein"}}, CreatedAt: time.Now().UTC()}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.ProcessListingKit(ctx, task); err == nil {
		t.Fatal("ProcessListingKit() error = nil, want release failure")
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.RetryableBlock == nil || stored.RetryableBlock.ReasonCode != usageReleasePendingReason {
		t.Fatalf("stored task = %#v, want usage_release_pending block", stored)
	}
}

func TestProcessListingKitRetriesTerminalFailureStateAfterRelease(t *testing.T) {
	t.Parallel()

	repo := &stubProcessStatusRepo{
		stubGenerationRepo: &stubGenerationRepo{},
		completedErr:       errors.New("task store unavailable"),
		failedErrs:         []error{errors.New("task store unavailable"), errors.New("task store unavailable"), errors.New("task store unavailable")},
	}
	settlement := &recordingGenerationUsageSettlement{}
	productService := &processUsageProductService{
		task:    &productenrich.Task{ID: "product-task-terminal-failed-state", Request: &productenrich.GenerateRequest{ProductURL: "https://example.com/product"}},
		product: &productenrich.ProductJSON{Title: "Travel Bag"},
	}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestProductService(productService),
		withTestAssembler(&stubProcessStatusAssembler{result: &ListingKitResult{Shein: &SheinPackage{}, Summary: &GenerationSummary{}}}),
		withTestConfig(func(cfg *ServiceConfig) { cfg.Core.GenerationUsageLedger = settlement }),
	))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	task := &Task{ID: "listingkit-terminal-failed-state", TenantID: "tenant-17", Status: core.TaskStatusPending, Request: &GenerateRequest{ProductURL: "https://example.com/product", Platforms: []string{"shein"}}, CreatedAt: time.Now().UTC()}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if _, err := svc.ProcessListingKit(context.Background(), task); err == nil {
		t.Fatal("ProcessListingKit() error = nil, want terminal persistence failure")
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != core.TaskStatusBlockedRetryable || stored.RetryableBlock == nil || stored.RetryableBlock.ReasonCode != terminalPersistencePendingReason {
		t.Fatalf("stored task = %#v, want terminal persistence retry block", stored)
	}
}

func TestProcessListingKitPersistsFailureAndReleasesWhenTerminalPersistenceFails(t *testing.T) {
	t.Parallel()

	repo := &stubProcessStatusRepo{stubGenerationRepo: &stubGenerationRepo{}, completedErr: errors.New("task store unavailable")}
	settlement := &recordingGenerationUsageSettlement{}
	productService := &processUsageProductService{
		task:    &productenrich.Task{ID: "product-task-terminal-persist", Request: &productenrich.GenerateRequest{ProductURL: "https://example.com/product"}},
		product: &productenrich.ProductJSON{Title: "Travel Bag", Category: []string{"bags"}},
	}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestProductService(productService),
		withTestAssembler(&stubProcessStatusAssembler{result: &ListingKitResult{Shein: &SheinPackage{}, Summary: &GenerationSummary{}}}),
		withTestConfig(func(cfg *ServiceConfig) { cfg.Core.GenerationUsageLedger = settlement }),
	))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	task := &Task{ID: "listingkit-terminal-persist", TenantID: "tenant-17", Status: core.TaskStatusPending, Request: &GenerateRequest{ProductURL: "https://example.com/product", Platforms: []string{"shein"}}, CreatedAt: time.Now().UTC()}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	if _, err := svc.ProcessListingKit(context.Background(), task); err == nil {
		t.Fatal("ProcessListingKit() error = nil, want terminal persistence failure")
	}
	if settlement.releasedTaskID != task.ID || settlement.releasedReason != "terminal_persistence_failed" {
		t.Fatalf("release = (%q, %q), want terminal persistence release", settlement.releasedTaskID, settlement.releasedReason)
	}
	if repo.task.Status != core.TaskStatusFailed {
		t.Fatalf("task status = %s, want failed after terminal persistence failure", repo.task.Status)
	}
}

func TestProcessListingKitKeepsReleaseFailureRecoverableAfterTerminalPersistenceFailure(t *testing.T) {
	t.Parallel()

	repo := &stubProcessStatusRepo{stubGenerationRepo: &stubGenerationRepo{}, completedErr: errors.New("task store unavailable")}
	settlement := &recordingGenerationUsageSettlement{releaseErr: errors.New("ledger context deadline exceeded")}
	productService := &processUsageProductService{
		task:    &productenrich.Task{ID: "product-task-release-persist", Request: &productenrich.GenerateRequest{ProductURL: "https://example.com/product"}},
		product: &productenrich.ProductJSON{Title: "Travel Bag", Category: []string{"bags"}},
	}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestProductService(productService),
		withTestAssembler(&stubProcessStatusAssembler{result: &ListingKitResult{Shein: &SheinPackage{}, Summary: &GenerationSummary{}}}),
		withTestConfig(func(cfg *ServiceConfig) { cfg.Core.GenerationUsageLedger = settlement }),
	))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	task := &Task{ID: "listingkit-release-persist", TenantID: "tenant-17", Status: core.TaskStatusPending, Request: &GenerateRequest{ProductURL: "https://example.com/product", Platforms: []string{"shein"}}, CreatedAt: time.Now().UTC()}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	if _, err := svc.ProcessListingKit(context.Background(), task); err == nil {
		t.Fatal("ProcessListingKit() error = nil, want terminal persistence/release failure")
	}
	if repo.task.Status != core.TaskStatusBlockedRetryable || repo.task.RetryableBlock == nil || repo.task.RetryableBlock.ReasonCode != "usage_release_pending" {
		t.Fatalf("task after release failure = %#v, want usage_release_pending block", repo.task)
	}
	if repo.failedCalls != 0 {
		t.Fatalf("MarkFailed calls = %d, want 0 while release remains recoverable", repo.failedCalls)
	}
}

func TestProcessListingKitPersistsRetryableStateWhenReservationFails(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{reserveErr: errors.New("upstream request failed: context deadline exceeded")}
	svc, repo, _, task := newProcessUsageFixture(t, settlement, nil)

	if _, err := svc.ProcessListingKit(context.Background(), task); err == nil {
		t.Fatal("ProcessListingKit() error = nil, want reservation failure")
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != core.TaskStatusBlockedRetryable || stored.RetryableBlock == nil {
		t.Fatalf("stored task = %#v, want retryable reservation block", stored)
	}
}

func TestProcessListingKitRestoresRetryableBlockWhenFailurePersistenceFails(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{}
	svc, repo, _, task := newProcessUsageFixture(t, settlement, errors.New("OpenAI API error: insufficient credits in account balance"))
	// The fixture creates its own repository; inject the transient block failure
	// after construction so the fallback has to persist the retryable state.
	repo.blockedErrs = []error{errors.New("task store unavailable once")}

	if _, err := svc.ProcessListingKit(context.Background(), task); err == nil {
		t.Fatal("ProcessListingKit() error = nil, want retryable workflow failure")
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != core.TaskStatusBlockedRetryable || stored.RetryableBlock == nil || stored.RetryableBlock.ReasonCode != "openai_insufficient_credits" {
		t.Fatalf("stored task = %#v, want durable retryable block", stored)
	}
	if settlement.releasedTaskID != "" {
		t.Fatalf("released task = %q, want reservation preserved for retry", settlement.releasedTaskID)
	}
}

func TestProcessListingKitCommitsNeedsReviewResult(t *testing.T) {
	t.Parallel()

	repo := &stubProcessStatusRepo{stubGenerationRepo: &stubGenerationRepo{}}
	settlement := &recordingGenerationUsageSettlement{repo: repo}
	productService := &processUsageProductService{
		task:    &productenrich.Task{ID: "product-task-review", Request: &productenrich.GenerateRequest{ProductURL: "https://example.com/product"}},
		product: &productenrich.ProductJSON{Title: "Travel Bag", Category: []string{"bags"}},
	}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestProductService(productService),
		withTestAssembler(&stubProcessStatusAssembler{result: &ListingKitResult{Shein: &SheinPackage{}, Summary: &GenerationSummary{NeedsReview: true, Warnings: []string{"manual review"}}}}),
		withTestConfig(func(cfg *ServiceConfig) { cfg.Core.GenerationUsageLedger = settlement }),
	))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	task := &Task{ID: "listingkit-usage-review", TenantID: "tenant-17", Status: core.TaskStatusPending, Request: &GenerateRequest{ProductURL: "https://example.com/product", Platforms: []string{"shein"}}, CreatedAt: time.Now().UTC()}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	result, err := svc.ProcessListingKit(context.Background(), task)
	if err != nil {
		t.Fatalf("ProcessListingKit() error = %v", err)
	}
	if result.Status != string(core.TaskStatusNeedsReview) || repo.task.Status != core.TaskStatusNeedsReview {
		t.Fatalf("result/task status = (%q, %q), want needs_review", result.Status, repo.task.Status)
	}
	if len(settlement.calls) != 2 || settlement.calls[0] != "reserve" || settlement.calls[1] != "commit" {
		t.Fatalf("settlement calls = %#v, want reserve then commit", settlement.calls)
	}
}

func TestProcessListingKitPersistsUsageCommitPendingOnCommitFailure(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{commitErr: errors.New("ledger unavailable")}
	svc, repo, _, task := newProcessUsageFixture(t, settlement, nil)
	settlement.repo = repo

	if _, err := svc.ProcessListingKit(context.Background(), task); err == nil {
		t.Fatal("ProcessListingKit() error = nil, want commit failure")
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != core.TaskStatusBlockedRetryable || stored.RetryableBlock == nil || stored.RetryableBlock.ReasonCode != "usage_commit_pending" {
		t.Fatalf("stored task = %#v, want usage_commit_pending block", stored)
	}
	if stored.RetryableBlock.NextRetryAt == nil {
		t.Fatal("usage_commit_pending NextRetryAt = nil, want scheduled recovery")
	}
}

func TestProcessListingKitRetriesUsageCommitPendingPersistence(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{commitErr: errors.New("ledger unavailable")}
	svc, repo, _, task := newProcessUsageFixture(t, settlement, nil)
	settlement.repo = repo
	repo.blockedErrs = []error{errors.New("task store unavailable once")}

	if _, err := svc.ProcessListingKit(context.Background(), task); err == nil {
		t.Fatal("ProcessListingKit() error = nil, want commit failure")
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if repo.blockedCalls != 2 {
		t.Fatalf("MarkBlockedRetryable calls = %d, want retry after transient persistence error", repo.blockedCalls)
	}
	if stored.Status != core.TaskStatusBlockedRetryable || stored.RetryableBlock == nil || stored.RetryableBlock.ReasonCode != "usage_commit_pending" {
		t.Fatalf("stored task = %#v, want durable usage_commit_pending block", stored)
	}
}

func TestProcessListingKitDoesNotDoubleReserveOrRunOnCommittedReplay(t *testing.T) {
	t.Parallel()

	settlement := &recordingGenerationUsageSettlement{reserveCommitted: true}
	svc, repo, productService, task := newProcessUsageFixture(t, settlement, nil)
	if err := repo.SaveTaskResult(context.Background(), task.ID, &ListingKitResult{Status: string(core.TaskStatusCompleted)}); err != nil {
		t.Fatalf("SaveTaskResult() error = %v", err)
	}
	loaded, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}

	result, err := svc.ProcessListingKit(context.Background(), loaded)
	if err != nil {
		t.Fatalf("ProcessListingKit() error = %v, want committed replay result", err)
	}
	if productService.processCalls != 0 {
		t.Fatalf("ProcessProduct calls = %d, want 0 on committed replay", productService.processCalls)
	}
	if result == nil || result.Status != string(core.TaskStatusCompleted) {
		t.Fatalf("result = %#v, want persisted completed result", result)
	}
	stored, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if stored.Status != core.TaskStatusCompleted {
		t.Fatalf("stored status = %q, want completed after committed replay", stored.Status)
	}
	if stored.GenerationUsageReservationState != "" || stored.GenerationUsageReservationLeaseUntil != nil {
		t.Fatalf("stored reservation = (%q, %v), want cleared after committed replay", stored.GenerationUsageReservationState, stored.GenerationUsageReservationLeaseUntil)
	}
	if len(settlement.calls) != 1 || settlement.calls[0] != "reserve" {
		t.Fatalf("settlement calls = %#v, want one idempotent reserve", settlement.calls)
	}
}
