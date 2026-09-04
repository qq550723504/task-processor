package amazonlisting

import (
	"context"
	"errors"
	"testing"

	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/shared/aiidentity"
)

type stubWorkflowProductSnapshotReader struct {
	snapshot catalog.ProductSnapshot
	err      error
	queries  []ProductSnapshotQuery
}

func (s *stubWorkflowProductSnapshotReader) GetProductSnapshot(_ context.Context, query ProductSnapshotQuery) (catalog.ProductSnapshot, error) {
	s.queries = append(s.queries, query)
	return s.snapshot, s.err
}

type stubWorkflowApprovedAssetReader struct {
	inventory productasset.ApprovedAssetInventory
	err       error
	scopes    []productasset.InventoryScope
}

func (s *stubWorkflowApprovedAssetReader) GetApprovedInventory(_ context.Context, scope productasset.InventoryScope) (productasset.ApprovedAssetInventory, error) {
	s.scopes = append(s.scopes, scope)
	return s.inventory, s.err
}

func TestListingWorkflowReadsScopedSnapshotAndApprovedAssets(t *testing.T) {
	snapshotReader := &stubWorkflowProductSnapshotReader{snapshot: catalog.ProductSnapshot{
		Title:       "Insulated Bottle",
		Description: "Vacuum insulated stainless steel bottle for everyday use.",
	}}
	assetReader := &stubWorkflowApprovedAssetReader{inventory: productasset.ApprovedAssetInventory{
		Scope: productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1", TargetPlatform: "amazon", SourceSnapshotVersion: 9},
		Assets: []productasset.ApprovedAsset{
			{ID: "main-1", Role: productasset.RoleMain, URL: "https://cdn.example.com/main.jpg"},
		},
	}}
	workflow := NewListingWorkflow(snapshotReader, assetReader, NewAssembler(), nil, nil)
	task := &Task{ID: "listing-task-1", Request: &GenerateRequest{Marketplace: "amazon", ProductKey: "product-1"}}
	task.SourceSnapshotVersion = 9
	task.SetExecutionEnvelope(aiidentity.ExecutionEnvelope{
		Version:        aiidentity.CurrentEnvelopeVersion,
		TenantID:       "tenant-a",
		UserID:         "user-a",
		BusinessTaskID: task.ID,
		SourcePlatform: "amazon",
		SourceTaskType: "listing",
	})

	artifacts, err := workflow.Run(context.Background(), task)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if artifacts == nil || artifacts.Draft == nil || artifacts.Draft.Title != "Insulated Bottle" {
		t.Fatalf("workflow artifacts = %+v", artifacts)
	}
	if len(snapshotReader.queries) != 1 || snapshotReader.queries[0] != (ProductSnapshotQuery{TenantID: "tenant-a", ProductKey: "product-1", Version: 9}) {
		t.Fatalf("snapshot queries = %+v", snapshotReader.queries)
	}
	wantScope := productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1", TargetPlatform: "amazon", SourceSnapshotVersion: 9}
	if len(assetReader.scopes) != 1 || assetReader.scopes[0] != wantScope {
		t.Fatalf("approved asset scopes = %+v, want %+v", assetReader.scopes, wantScope)
	}
}

func TestListingWorkflowReturnsSnapshotNotReadyWithoutReader(t *testing.T) {
	assetReader := &stubWorkflowApprovedAssetReader{}
	workflow := NewListingWorkflow(nil, assetReader, NewAssembler(), nil, nil)
	task := &Task{ID: "listing-task-1", Request: &GenerateRequest{Marketplace: "amazon", ProductKey: "product-1"}}
	task.SetExecutionEnvelope(aiidentity.ExecutionEnvelope{
		Version:        aiidentity.CurrentEnvelopeVersion,
		TenantID:       "tenant-a",
		UserID:         "user-a",
		BusinessTaskID: task.ID,
		SourcePlatform: "amazon",
		SourceTaskType: "listing",
	})

	_, err := workflow.Run(context.Background(), task)
	if !errors.Is(err, ErrProductSnapshotNotReady) {
		t.Fatalf("Run() error = %v, want ErrProductSnapshotNotReady", err)
	}
}

func TestListingWorkflowReturnsSnapshotNotReadyForEmptySnapshot(t *testing.T) {
	workflow := NewListingWorkflow(
		&stubWorkflowProductSnapshotReader{},
		&stubWorkflowApprovedAssetReader{},
		NewAssembler(),
		nil,
		nil,
	)
	task := newWorkflowTestTask("tenant-a", "product-1")

	_, err := workflow.Run(context.Background(), task)
	if !errors.Is(err, ErrProductSnapshotNotReady) {
		t.Fatalf("Run() error = %v, want ErrProductSnapshotNotReady", err)
	}
}

func TestListingWorkflowRejectsInventoryFromAnotherScope(t *testing.T) {
	workflow := NewListingWorkflow(
		&stubWorkflowProductSnapshotReader{snapshot: catalog.ProductSnapshot{Title: "Insulated Bottle"}},
		&stubWorkflowApprovedAssetReader{inventory: productasset.ApprovedAssetInventory{
			Scope: productasset.InventoryScope{TenantID: "tenant-b", ProductKey: "product-1"},
		}},
		NewAssembler(),
		nil,
		nil,
	)
	task := newWorkflowTestTask("tenant-a", "product-1")

	_, err := workflow.Run(context.Background(), task)
	if !errors.Is(err, productasset.ErrRepositoryStateInvalid) {
		t.Fatalf("Run() error = %v, want ErrRepositoryStateInvalid", err)
	}
}

func newWorkflowTestTask(tenantID, productKey string) *Task {
	task := &Task{ID: "listing-task-1", Request: &GenerateRequest{Marketplace: "amazon", ProductKey: productKey}}
	task.SetExecutionEnvelope(aiidentity.ExecutionEnvelope{
		Version:        aiidentity.CurrentEnvelopeVersion,
		TenantID:       tenantID,
		UserID:         "user-a",
		BusinessTaskID: task.ID,
		SourcePlatform: "amazon",
		SourceTaskType: "listing",
	})
	return task
}
