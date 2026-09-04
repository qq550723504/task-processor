package amazonlisting

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

type WorkflowArtifacts struct {
	Snapshot       catalog.ProductSnapshot
	ApprovedAssets productasset.ApprovedAssetInventory
	Draft          *AmazonListingDraft
}

type ListingWorkflow interface {
	Run(ctx context.Context, task *Task) (*WorkflowArtifacts, error)
}

type listingWorkflow struct {
	productSnapshots ProductSnapshotReader
	approvedAssets   ApprovedAssetInventoryReader
	assembler        Assembler
	autoFixer        AutoFixer
	exportBuilder    ExportBuilder
}

func NewListingWorkflow(productSnapshots ProductSnapshotReader, approvedAssets ApprovedAssetInventoryReader, assembler Assembler, autoFixer AutoFixer, exportBuilder ExportBuilder) ListingWorkflow {
	return &listingWorkflow{
		productSnapshots: productSnapshots,
		approvedAssets:   approvedAssets,
		assembler:        assembler,
		autoFixer:        autoFixer,
		exportBuilder:    exportBuilder,
	}
}

func (w *listingWorkflow) Run(ctx context.Context, task *Task) (*WorkflowArtifacts, error) {
	if task == nil || task.Request == nil {
		return nil, fmt.Errorf("task and request are required")
	}
	if w.assembler == nil {
		return nil, fmt.Errorf("assembler is not configured")
	}

	query := ProductSnapshotQuery{
		TenantID:   strings.TrimSpace(task.ExecutionTenantID),
		ProductKey: strings.TrimSpace(task.Request.ProductKey),
		Version:    task.SourceSnapshotVersion,
	}
	if query.TenantID == "" || query.ProductKey == "" || w.productSnapshots == nil {
		return nil, ErrProductSnapshotNotReady
	}
	snapshot, err := w.productSnapshots.GetProductSnapshot(ctx, query)
	if err != nil {
		return nil, err
	}
	snapshot, err = cloneAmazonProductSnapshot(snapshot)
	if err != nil {
		return nil, err
	}

	scope := productasset.InventoryScope{TenantID: query.TenantID, ProductKey: query.ProductKey, TargetPlatform: strings.ToLower(strings.TrimSpace(task.Request.Marketplace)), SourceSnapshotVersion: query.Version}
	if w.approvedAssets == nil {
		return nil, productasset.ErrApprovedAssetsNotReady
	}
	inventory, err := w.approvedAssets.GetApprovedInventory(ctx, scope)
	if err != nil {
		return nil, err
	}
	if inventory.Scope != scope {
		return nil, productasset.ErrRepositoryStateInvalid
	}
	inventory = productasset.CloneApprovedAssetInventory(inventory)

	draft, err := w.assembler.Build(DraftInput{
		TaskID:         task.ID,
		Request:        task.Request,
		Snapshot:       snapshot,
		ApprovedAssets: inventory,
	})
	if err != nil {
		return nil, err
	}
	if draft == nil {
		return nil, fmt.Errorf("assembler returned nil draft")
	}
	draft.ListingIPRisk = assessContentIPRisk(task.Request, draft)
	draft.ReviewItems = append(draft.ReviewItems, buildReviewItemsFromSnapshot(&snapshot)...)
	if w.autoFixer != nil {
		w.autoFixer.Fix(task.Request, draft)
	}
	if w.exportBuilder != nil {
		draft.Export = w.exportBuilder.Build(task.Request, draft)
	}

	return &WorkflowArtifacts{Snapshot: snapshot, ApprovedAssets: inventory, Draft: draft}, nil
}

func cloneAmazonProductSnapshot(snapshot catalog.ProductSnapshot) (catalog.ProductSnapshot, error) {
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return catalog.ProductSnapshot{}, fmt.Errorf("clone product snapshot: %w", err)
	}
	if string(raw) == "{}" {
		return catalog.ProductSnapshot{}, ErrProductSnapshotNotReady
	}
	var cloned catalog.ProductSnapshot
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return catalog.ProductSnapshot{}, fmt.Errorf("clone product snapshot: %w", err)
	}
	return cloned, nil
}
