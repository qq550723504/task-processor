package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/listingkit/core"
	sdsdesign "task-processor/internal/sds/design"
)

func TestGetTaskSDSRepairReturnsCurrentLayersForFailedVariant(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	if err := repo.CreateTask(context.Background(), &Task{
		ID:       "task-sds-repair-1",
		TenantID: "tenant-1",
		Status:   core.TaskStatusNeedsReview,
		Request: &GenerateRequest{Options: &GenerateOptions{SDS: &SDSSyncOptions{
			VariantID:        101,
			ParentProductID:  200,
			PrototypeGroupID: 300,
			LayerID:          "10033204",
			Variants: []SDSSyncVariantOption{{
				VariantID:        101,
				VariantSKU:       "white-s",
				Color:            "white",
				Size:             "S",
				PrototypeGroupID: 300,
				LayerID:          "10033204",
			}},
		}}},
		Result: &ListingKitResult{ChildTasks: []ChildTaskState{{
			Kind:   "sds_design_sync",
			Status: string(core.TaskStatusFailed),
		}}},
	}); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	svc := seedSupportDeps(&service{repo: repo}, supportDependencySeed{
		sdsBaselineRemoteProvider: stubSDSBaselineRemoteProvider{
			designProduct: &sdsdesign.DesignProductPage{Layers: []sdsdesign.DesignLayer{
				{ID: "10040001", Name: "Front"},
			}},
		},
	})

	session, err := svc.GetTaskSDSRepair(context.Background(), "task-sds-repair-1")
	if err != nil {
		t.Fatalf("GetTaskSDSRepair() error = %v", err)
	}
	if got, want := session.TaskID, "task-sds-repair-1"; got != want {
		t.Fatalf("TaskID = %q, want %q", got, want)
	}
	if len(session.Variants) != 1 {
		t.Fatalf("variants = %+v, want one variant", session.Variants)
	}
	variant := session.Variants[0]
	if got, want := variant.OldLayerID, "10033204"; got != want {
		t.Fatalf("OldLayerID = %q, want %q", got, want)
	}
	if got, want := variant.Color, "white"; got != want {
		t.Fatalf("Color = %q, want %q", got, want)
	}
	if len(variant.Layers) != 1 || variant.Layers[0].ID != "10040001" {
		t.Fatalf("Layers = %+v, want current remote layer", variant.Layers)
	}
}

func TestSDSRepairCleanupContextSurvivesRequestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cleanupCtx, cleanupCancel := sdsRepairCleanupContext(ctx)
	defer cleanupCancel()
	if err := cleanupCtx.Err(); err != nil {
		t.Fatalf("cleanup context error = %v, want nil after request cancellation", err)
	}
	if _, ok := cleanupCtx.Deadline(); !ok {
		t.Fatal("cleanup context has no bounded deadline")
	}
}

func TestRepairAndRetryTaskSDSRejectsLayerMissingFromCurrentVariantPage(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	if err := repo.CreateTask(context.Background(), &Task{
		ID:       "task-sds-repair-invalid-layer",
		TenantID: "tenant-1",
		Status:   core.TaskStatusNeedsReview,
		Request: &GenerateRequest{Options: &GenerateOptions{SDS: &SDSSyncOptions{
			VariantID: 101, ParentProductID: 200, PrototypeGroupID: 300, LayerID: "10033204",
			Variants: []SDSSyncVariantOption{{
				VariantID: 101, VariantSKU: "white-s", Color: "white", PrototypeGroupID: 300, LayerID: "10033204",
			}},
		}}},
		Result: &ListingKitResult{ChildTasks: []ChildTaskState{{Kind: "sds_design_sync", Status: string(core.TaskStatusFailed)}}},
	}); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	svc := seedSupportDeps(&service{repo: repo}, supportDependencySeed{
		sdsBaselineRemoteProvider: stubSDSBaselineRemoteProvider{designProduct: &sdsdesign.DesignProductPage{
			Layers: []sdsdesign.DesignLayer{{ID: "10040001", Name: "Front"}},
		}},
	})

	_, err := svc.RepairAndRetryTaskSDS(context.Background(), "task-sds-repair-invalid-layer", &ApplyTaskSDSRepairRequest{
		Variants: []SDSRepairVariantSelection{{VariantID: 101, LayerID: "not-on-page"}},
	})
	if !errors.Is(err, ErrSDSRepairLayerUnavailable) {
		t.Fatalf("RepairAndRetryTaskSDS() error = %v, want ErrSDSRepairLayerUnavailable", err)
	}
	after, err := repo.GetTask(context.Background(), "task-sds-repair-invalid-layer")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got, want := after.Request.Options.SDS.Variants[0].LayerID, "10033204"; got != want {
		t.Fatalf("persisted layer = %q, want unchanged %q", got, want)
	}
}

func TestRepairAndRetryTaskSDSReplacesPersistedVariantLayerBeforeRetry(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	if err := repo.CreateTask(context.Background(), &Task{
		ID:       "task-sds-repair-success",
		TenantID: "tenant-1",
		Status:   core.TaskStatusNeedsReview,
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/source.png"},
			Options: &GenerateOptions{SDS: &SDSSyncOptions{
				VariantID: 101, ParentProductID: 200, PrototypeGroupID: 300, LayerID: "10033204",
				Variants: []SDSSyncVariantOption{{
					VariantID: 101, VariantSKU: "white-s", Color: "white", PrototypeGroupID: 300, LayerID: "10033204",
				}},
			}},
		},
		Result: &ListingKitResult{ChildTasks: []ChildTaskState{{Kind: "sds_design_sync", Status: string(core.TaskStatusFailed)}}},
	}); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	svc := seedSupportDeps(&service{repo: repo}, supportDependencySeed{
		sdsSyncService: &stubWorkflowSDSSyncService{},
		assembler:      &stubProcessStatusAssembler{},
		sdsBaselineRemoteProvider: stubSDSBaselineRemoteProvider{designProduct: &sdsdesign.DesignProductPage{
			Layers: []sdsdesign.DesignLayer{{ID: "10040001", Name: "Front"}},
		}},
	})

	result, err := svc.RepairAndRetryTaskSDS(context.Background(), "task-sds-repair-success", &ApplyTaskSDSRepairRequest{
		Variants: []SDSRepairVariantSelection{{VariantID: 101, LayerID: "10040001"}},
	})
	if err != nil {
		t.Fatalf("RepairAndRetryTaskSDS() error = %v", err)
	}
	if result == nil || result.TaskID != "task-sds-repair-success" {
		t.Fatalf("result = %+v, want original task result", result)
	}
	after, err := repo.GetTask(context.Background(), "task-sds-repair-success")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got, want := after.Request.Options.SDS.LayerID, "10040001"; got != want {
		t.Fatalf("primary layer = %q, want %q", got, want)
	}
	if got, want := after.Request.Options.SDS.Variants[0].LayerID, "10040001"; got != want {
		t.Fatalf("variant layer = %q, want %q", got, want)
	}
	if after.Result.PodExecution == nil || len(after.Result.PodExecution.History) == 0 {
		t.Fatalf("repair audit history = %+v, want appended event", after.Result.PodExecution)
	}
}

func TestRepairAndRetryTaskSDSCancelsPendingDurableRetry(t *testing.T) {
	t.Parallel()

	repo := &sdsChildRetryTestRepository{Repository: NewInMemoryRepositoryForTest(), jobs: map[string]SDSChildRetryJob{
		"job-sds-repair-cancel": {
			ID: "job-sds-repair-cancel", TaskID: "task-sds-repair-cancel",
			Kind: SDSChildRetryKindDesignSync, Status: SDSChildRetryJobStatusPending,
		},
	}}
	if err := repo.CreateTask(context.Background(), &Task{
		ID: "task-sds-repair-cancel", TenantID: "tenant-1", Status: core.TaskStatusNeedsReview,
		Request: &GenerateRequest{ImageURLs: []string{"https://example.com/source.png"}, Options: &GenerateOptions{SDS: &SDSSyncOptions{
			VariantID: 101, ParentProductID: 200, PrototypeGroupID: 300, LayerID: "10033204",
			Variants: []SDSSyncVariantOption{{VariantID: 101, VariantSKU: "white-s", PrototypeGroupID: 300, LayerID: "10033204"}},
		}}},
		Result: &ListingKitResult{ChildTasks: []ChildTaskState{{Kind: "sds_design_sync", Status: string(core.TaskStatusFailed)}}},
	}); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	remoteResult := successfulWorkflowSDSSyncResult().DesignSync
	remoteResult.DesignResult.Page.Product.ID = 101
	svc := seedSupportDeps(&service{repo: repo}, supportDependencySeed{
		sdsSyncService: &stubWorkflowSDSSyncService{remoteResult: remoteResult}, assembler: &stubProcessStatusAssembler{},
		sdsBaselineRemoteProvider: stubSDSBaselineRemoteProvider{designProduct: &sdsdesign.DesignProductPage{Layers: []sdsdesign.DesignLayer{{ID: "10040001"}}}},
	})

	if _, err := svc.RepairAndRetryTaskSDS(context.Background(), "task-sds-repair-cancel", &ApplyTaskSDSRepairRequest{
		Variants: []SDSRepairVariantSelection{{VariantID: 101, LayerID: "10040001"}},
	}); err != nil {
		t.Fatalf("RepairAndRetryTaskSDS() error = %v", err)
	}
	if got := repo.jobs["job-sds-repair-cancel"].Status; got != SDSChildRetryJobStatusCancelled {
		t.Fatalf("durable retry status = %q, want cancelled", got)
	}
}

func TestRepairAndRetryTaskSDSPreservesSiblingDurableRetry(t *testing.T) {
	t.Parallel()

	repo := &sdsChildRetryTestRepository{Repository: NewInMemoryRepositoryForTest(), jobs: map[string]SDSChildRetryJob{
		"job-sds-repair-design": {
			ID: "job-sds-repair-design", TaskID: "task-sds-repair-sibling",
			Kind: SDSChildRetryKindDesignSync, Status: SDSChildRetryJobStatusPending,
		},
		"job-sds-repair-catalog": {
			ID: "job-sds-repair-catalog", TaskID: "task-sds-repair-sibling",
			Kind: SDSChildRetryKindCatalogProduct, Status: SDSChildRetryJobStatusPending,
		},
	}}
	if err := repo.CreateTask(context.Background(), &Task{
		ID: "task-sds-repair-sibling", TenantID: "tenant-1", Status: core.TaskStatusNeedsReview,
		Request: &GenerateRequest{ImageURLs: []string{"https://example.com/source.png"}, Options: &GenerateOptions{SDS: &SDSSyncOptions{
			VariantID: 101, ParentProductID: 200, PrototypeGroupID: 300, LayerID: "10033204",
			Variants: []SDSSyncVariantOption{{VariantID: 101, VariantSKU: "white-s", PrototypeGroupID: 300, LayerID: "10033204"}},
		}}},
		Result: &ListingKitResult{ChildTasks: []ChildTaskState{{Kind: "sds_design_sync", Status: string(core.TaskStatusFailed)}}},
	}); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	remoteResult := successfulWorkflowSDSSyncResult().DesignSync
	remoteResult.DesignResult.Page.Product.ID = 101
	svc := seedSupportDeps(&service{repo: repo}, supportDependencySeed{
		sdsSyncService: &stubWorkflowSDSSyncService{remoteResult: remoteResult}, assembler: &stubProcessStatusAssembler{},
		sdsBaselineRemoteProvider: stubSDSBaselineRemoteProvider{designProduct: &sdsdesign.DesignProductPage{Layers: []sdsdesign.DesignLayer{{ID: "10040001"}}}},
	})

	if _, err := svc.RepairAndRetryTaskSDS(context.Background(), "task-sds-repair-sibling", &ApplyTaskSDSRepairRequest{
		Variants: []SDSRepairVariantSelection{{VariantID: 101, LayerID: "10040001"}},
	}); err != nil {
		t.Fatalf("RepairAndRetryTaskSDS() error = %v", err)
	}
	if got := repo.jobs["job-sds-repair-design"].Status; got != SDSChildRetryJobStatusCancelled {
		t.Fatalf("design retry status = %q, want cancelled", got)
	}
	if got := repo.jobs["job-sds-repair-catalog"].Status; got != SDSChildRetryJobStatusPending {
		t.Fatalf("catalog retry status = %q, want pending", got)
	}
}

func TestRepairAndRetryTaskSDSRevalidatesTaskAfterRepairLease(t *testing.T) {
	t.Parallel()

	repo := &sdsChildRetryTestRepository{Repository: NewInMemoryRepositoryForTest(), jobs: make(map[string]SDSChildRetryJob)}
	if err := repo.CreateTask(context.Background(), &Task{
		ID: "task-sds-repair-revalidate", TenantID: "tenant-1", Status: core.TaskStatusNeedsReview,
		Request: &GenerateRequest{ImageURLs: []string{"https://example.com/source.png"}, Options: &GenerateOptions{SDS: &SDSSyncOptions{
			VariantID: 101, ParentProductID: 200, PrototypeGroupID: 300, LayerID: "10033204",
			Variants: []SDSSyncVariantOption{{VariantID: 101, VariantSKU: "white-s", PrototypeGroupID: 300, LayerID: "10033204"}},
		}}},
		Result: &ListingKitResult{ChildTasks: []ChildTaskState{{Kind: "sds_design_sync", Status: string(core.TaskStatusFailed)}}},
	}); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	repo.afterBegin = func() {
		result := &ListingKitResult{ChildTasks: []ChildTaskState{{Kind: "sds_design_sync", Status: string(core.TaskStatusCompleted)}}}
		if err := repo.Repository.MarkCompleted(context.Background(), "task-sds-repair-revalidate", result); err != nil {
			t.Fatalf("MarkCompleted() error = %v", err)
		}
	}
	svc := seedSupportDeps(&service{repo: repo}, supportDependencySeed{
		sdsBaselineRemoteProvider: stubSDSBaselineRemoteProvider{designProduct: &sdsdesign.DesignProductPage{Layers: []sdsdesign.DesignLayer{{ID: "10040001"}}}},
	})

	_, err := svc.RepairAndRetryTaskSDS(context.Background(), "task-sds-repair-revalidate", &ApplyTaskSDSRepairRequest{
		Variants: []SDSRepairVariantSelection{{VariantID: 101, LayerID: "10040001"}},
	})
	if !errors.Is(err, ErrSDSRepairNotEligible) {
		t.Fatalf("RepairAndRetryTaskSDS() error = %v, want ErrSDSRepairNotEligible", err)
	}
	if got, want := repo.getTaskCalls, 2; got != want {
		t.Fatalf("GetTask calls = %d, want %d after acquiring repair lease", got, want)
	}
}

func TestRepairAndRetryTaskSDSRejectsActiveDurableRetry(t *testing.T) {
	t.Parallel()

	leaseUntil := time.Now().UTC().Add(time.Hour)
	repo := &sdsChildRetryTestRepository{Repository: NewInMemoryRepositoryForTest(), jobs: map[string]SDSChildRetryJob{
		"job-sds-repair-active": {
			ID: "job-sds-repair-active", TaskID: "task-sds-repair-active",
			Kind: SDSChildRetryKindDesignSync, Status: SDSChildRetryJobStatusPending,
			LeaseOwner: "sweeper", LeaseUntil: &leaseUntil,
		},
	}}
	if err := repo.CreateTask(context.Background(), &Task{
		ID: "task-sds-repair-active", TenantID: "tenant-1", Status: core.TaskStatusNeedsReview,
		Request: &GenerateRequest{ImageURLs: []string{"https://example.com/source.png"}, Options: &GenerateOptions{SDS: &SDSSyncOptions{
			VariantID: 101, ParentProductID: 200, PrototypeGroupID: 300, LayerID: "10033204",
			Variants: []SDSSyncVariantOption{{VariantID: 101, VariantSKU: "white-s", PrototypeGroupID: 300, LayerID: "10033204"}},
		}}},
		Result: &ListingKitResult{ChildTasks: []ChildTaskState{{Kind: "sds_design_sync", Status: string(core.TaskStatusFailed)}}},
	}); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	remoteResult := successfulWorkflowSDSSyncResult().DesignSync
	remoteResult.DesignResult.Page.Product.ID = 101
	svc := seedSupportDeps(&service{repo: repo}, supportDependencySeed{
		sdsSyncService:            &stubWorkflowSDSSyncService{remoteResult: remoteResult},
		assembler:                 &stubProcessStatusAssembler{},
		sdsBaselineRemoteProvider: stubSDSBaselineRemoteProvider{designProduct: &sdsdesign.DesignProductPage{Layers: []sdsdesign.DesignLayer{{ID: "10040001"}}}},
	})

	if _, err := svc.RepairAndRetryTaskSDS(context.Background(), "task-sds-repair-active", &ApplyTaskSDSRepairRequest{
		Variants: []SDSRepairVariantSelection{{VariantID: 101, LayerID: "10040001"}},
	}); !errors.Is(err, ErrSDSRepairRetryInProgress) {
		t.Fatalf("RepairAndRetryTaskSDS() error = %v, want ErrSDSRepairRetryInProgress", err)
	}
	if got := repo.jobs["job-sds-repair-active"].Status; got != SDSChildRetryJobStatusPending {
		t.Fatalf("durable retry status = %q, want pending", got)
	}
}
