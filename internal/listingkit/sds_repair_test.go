package listingkit

import (
	"context"
	"errors"
	"testing"

	sdsdesign "task-processor/internal/sds/design"
)

func TestGetTaskSDSRepairReturnsCurrentLayersForFailedVariant(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	if err := repo.CreateTask(context.Background(), &Task{
		ID:       "task-sds-repair-1",
		TenantID: "tenant-1",
		Status:   TaskStatusNeedsReview,
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
			Status: string(TaskStatusFailed),
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

func TestRepairAndRetryTaskSDSRejectsLayerMissingFromCurrentVariantPage(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	if err := repo.CreateTask(context.Background(), &Task{
		ID:       "task-sds-repair-invalid-layer",
		TenantID: "tenant-1",
		Status:   TaskStatusNeedsReview,
		Request: &GenerateRequest{Options: &GenerateOptions{SDS: &SDSSyncOptions{
			VariantID: 101, ParentProductID: 200, PrototypeGroupID: 300, LayerID: "10033204",
			Variants: []SDSSyncVariantOption{{
				VariantID: 101, VariantSKU: "white-s", Color: "white", PrototypeGroupID: 300, LayerID: "10033204",
			}},
		}}},
		Result: &ListingKitResult{ChildTasks: []ChildTaskState{{Kind: "sds_design_sync", Status: string(TaskStatusFailed)}}},
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
		Status:   TaskStatusNeedsReview,
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/source.png"},
			Options: &GenerateOptions{SDS: &SDSSyncOptions{
				VariantID: 101, ParentProductID: 200, PrototypeGroupID: 300, LayerID: "10033204",
				Variants: []SDSSyncVariantOption{{
					VariantID: 101, VariantSKU: "white-s", Color: "white", PrototypeGroupID: 300, LayerID: "10033204",
				}},
			}},
		},
		Result: &ListingKitResult{ChildTasks: []ChildTaskState{{Kind: "sds_design_sync", Status: string(TaskStatusFailed)}}},
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
