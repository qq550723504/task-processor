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
	calls            []string
	reserveErr       error
	commitErr        error
	reserveCommitted bool
	repo             *stubProcessStatusRepo
	reservedTaskID   string
	committedTaskID  string
	releasedTaskID   string
	releasedReason   string
}

func (s *recordingGenerationUsageSettlement) ReserveGeneration(_ context.Context, _ string, taskID string, _ time.Time) (GenerationUsageReservation, error) {
	s.calls = append(s.calls, "reserve")
	s.reservedTaskID = taskID
	if s.reserveErr != nil {
		return GenerationUsageReservation{}, s.reserveErr
	}
	return GenerationUsageReservation{EventID: "usage-" + taskID, AlreadyCommitted: s.reserveCommitted}, nil
}

func (s *recordingGenerationUsageSettlement) CommitGeneration(_ context.Context, _ string, taskID string) error {
	s.calls = append(s.calls, "commit")
	s.committedTaskID = taskID
	if s.repo != nil && s.repo.task.Status != core.TaskStatusCompleted && s.repo.task.Status != core.TaskStatusNeedsReview {
		return errors.New("commit called before terminal task persistence")
	}
	return s.commitErr
}

func (s *recordingGenerationUsageSettlement) ReleaseGeneration(_ context.Context, _ string, taskID, reason string) error {
	s.calls = append(s.calls, "release")
	s.releasedTaskID = taskID
	s.releasedReason = reason
	return nil
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
	if len(settlement.calls) != 1 || settlement.calls[0] != "reserve" {
		t.Fatalf("settlement calls = %#v, want one idempotent reserve", settlement.calls)
	}
}
