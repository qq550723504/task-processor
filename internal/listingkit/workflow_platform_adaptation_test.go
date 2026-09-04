package listingkit

import (
	"testing"

	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

func TestAssembleTargetSpecificPlatformsPassesEachInventoryToItsPlatform(t *testing.T) {
	task := &Task{
		TenantID: "tenant-a",
		Request:  &GenerateRequest{ProductKey: "product-1", Platforms: []string{"amazon", "shein"}},
	}
	inventories := map[string]productasset.ApprovedAssetInventory{
		"amazon": {Scope: productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1", TargetPlatform: "amazon"}, Assets: []productasset.ApprovedAsset{{ID: "amazon-main", Role: productasset.RoleMain}}},
		"shein":  {Scope: productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1", TargetPlatform: "shein"}, Assets: []productasset.ApprovedAsset{{ID: "shein-main", Role: productasset.RoleMain}}},
	}
	assembler := &recordingTargetInventoryAssembler{}

	result, err := assembleTargetSpecificPlatforms(assembler, task, &catalog.ProductSnapshot{Title: "Snapshot"}, inventories)

	if err != nil {
		t.Fatalf("assembleTargetSpecificPlatforms() error = %v", err)
	}
	if len(assembler.seen) != 2 || assembler.seen["amazon"] != "amazon-main" || assembler.seen["shein"] != "shein-main" {
		t.Fatalf("assembler saw inventories = %+v", assembler.seen)
	}
	if len(result.ApprovedAssetInventories) != 2 || result.ApprovedAssetInventories["amazon"].Assets[0].ID != "amazon-main" || result.ApprovedAssetInventories["shein"].Assets[0].ID != "shein-main" {
		t.Fatalf("result inventories = %+v", result.ApprovedAssetInventories)
	}
}

type recordingTargetInventoryAssembler struct {
	seen map[string]string
}

func (a *recordingTargetInventoryAssembler) Assemble(task *Task, product *catalog.ProductSnapshot, approved *productasset.ApprovedAssetInventory) (*ListingKitResult, error) {
	if a.seen == nil {
		a.seen = make(map[string]string)
	}
	a.seen[task.Request.Platforms[0]] = approved.Assets[0].ID
	return &ListingKitResult{CatalogProduct: product, ApprovedAssetInventory: approved}, nil
}
