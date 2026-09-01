package assetpersistence

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/asset/assettest"
)

func TestRepositoryContract(t *testing.T) {
	assettest.ExerciseRepositoryContract(t, func(t *testing.T) productasset.Repository {
		t.Helper()
		db := openRepositoryTestDB(t)
		if err := db.AutoMigrate(&ApprovedAssetRecord{}, &ApprovalReceiptRecord{}); err != nil {
			t.Fatalf("AutoMigrate() error = %v", err)
		}
		repo, err := NewRepository(db)
		if err != nil {
			t.Fatalf("NewRepository() error = %v", err)
		}
		return repo
	})
}

func TestNewRepositoryRejectsNilDatabase(t *testing.T) {
	repo, err := NewRepository(nil)
	if err == nil {
		t.Fatal("NewRepository(nil) error = nil, want error")
	}
	if repo != nil {
		t.Fatalf("NewRepository(nil) repository = %T, want nil", repo)
	}
}

func TestCommitApprovalRollsBackWholeBatchAndReceiptOnInsertFailure(t *testing.T) {
	db := openRepositoryTestDB(t)
	if err := db.AutoMigrate(&ApprovedAssetRecord{}, &ApprovalReceiptRecord{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
		CREATE TRIGGER fail_second_approved_asset
		BEFORE INSERT ON product_approved_assets
		WHEN NEW.asset_id = 'asset-2'
		BEGIN
			SELECT RAISE(FAIL, 'injected asset insert failure');
		END
	`).Error; err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	repo, err := NewRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	commit := repositoryTestCommit("tenant-a", "product-1", "approve-1", "asset-1")
	second := commit.Assets[0]
	second.ID = "asset-2"
	second.SlotID = "gallery-1"
	second.Role = productasset.RoleGallery
	second.URL = "https://cdn.example/asset-2.png"
	commit.Assets = append(commit.Assets, second)

	if _, err := repo.CommitApproval(context.Background(), commit); err == nil {
		t.Fatal("CommitApproval() error = nil, want injected transaction failure")
	} else if errors.Is(err, productasset.ErrApprovalConflict) {
		t.Fatalf("CommitApproval() error = %v, want storage failure rather than approval conflict", err)
	}
	for _, model := range []any{&ApprovedAssetRecord{}, &ApprovalReceiptRecord{}} {
		var count int64
		if err := db.Model(model).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("rows for %T after failed transaction = %d, want 0", model, count)
		}
	}
	if err := db.Exec("DROP TRIGGER fail_second_approved_asset").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CommitApproval(context.Background(), commit); err != nil {
		t.Fatalf("retry after rollback error = %v", err)
	}
	inventory, err := repo.GetApprovedInventory(context.Background(), productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Assets) != 2 {
		t.Fatalf("inventory assets = %d, want 2", len(inventory.Assets))
	}
}

func TestCommitApprovalPersistsFullTenantQualifiedIdentity(t *testing.T) {
	db := openRepositoryTestDB(t)
	if err := db.AutoMigrate(&ApprovedAssetRecord{}, &ApprovalReceiptRecord{}); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	commit := repositoryTestCommit("tenant-a", "product-1", "approve-7", "asset-1")
	commit.Assets[0].RunID = "run-9"
	commit.Assets[0].PlanRevision = 4
	commit.Assets[0].SlotID = "main-hero"
	commit.Assets[0].Attempt = 3
	if _, err := repo.CommitApproval(context.Background(), commit); err != nil {
		t.Fatal(err)
	}

	var record ApprovedAssetRecord
	if err := db.First(&record).Error; err != nil {
		t.Fatal(err)
	}
	if record.TenantID != "tenant-a" || record.RunID != "run-9" || record.PlanRevision != 4 || record.SlotID != "main-hero" || record.Attempt != 3 || record.ActionID != "approve-7" {
		t.Fatalf("persisted identity = tenant=%q run=%q revision=%d slot=%q attempt=%d action=%q", record.TenantID, record.RunID, record.PlanRevision, record.SlotID, record.Attempt, record.ActionID)
	}
}

func TestGetApprovedInventoryHasStableReconstructionOrder(t *testing.T) {
	db := openRepositoryTestDB(t)
	if err := db.AutoMigrate(&ApprovedAssetRecord{}, &ApprovalReceiptRecord{}); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	for index, assetID := range []string{"asset-b", "asset-a"} {
		commit := repositoryTestCommit("tenant-a", "product-1", fmt.Sprintf("approve-%d", index+1), assetID)
		commit.Assets[0].RunID = fmt.Sprintf("run-%d", 2-index)
		if _, err := repo.CommitApproval(context.Background(), commit); err != nil {
			t.Fatal(err)
		}
	}
	first, err := repo.GetApprovedInventory(context.Background(), productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.GetApprovedInventory(context.Background(), productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Assets) != 2 || first.Assets[0].ID != "asset-a" || first.Assets[1].ID != "asset-b" {
		t.Fatalf("stable inventory order = %+v, want asset-a then asset-b", first.Assets)
	}
	if first.Assets[0].ID != second.Assets[0].ID || first.Assets[1].ID != second.Assets[1].ID {
		t.Fatalf("repeated inventory order differs: first=%+v second=%+v", first.Assets, second.Assets)
	}
}

func TestGetApprovedInventoryFiltersProductInsideTenant(t *testing.T) {
	db := openRepositoryTestDB(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CommitApproval(context.Background(), repositoryTestCommit("tenant-a", "product-1", "approve-1", "asset-1")); err != nil {
		t.Fatal(err)
	}
	_, err = repo.GetApprovedInventory(context.Background(), productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-2"})
	if !errors.Is(err, productasset.ErrApprovedAssetsNotReady) {
		t.Fatalf("cross-product read error = %v, want ErrApprovedAssetsNotReady", err)
	}
}

func TestCommitApprovalCanonicalHashDistinguishesNilAndEmptyOperations(t *testing.T) {
	db := openRepositoryTestDB(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	commit := repositoryTestCommit("tenant-a", "product-1", "approve-1", "asset-1")
	commit.Assets[0].Operations = nil
	if _, err := repo.CommitApproval(context.Background(), commit); err != nil {
		t.Fatal(err)
	}
	commit.Assets[0].Operations = []string{}
	if _, err := repo.CommitApproval(context.Background(), commit); !errors.Is(err, productasset.ErrApprovalConflict) {
		t.Fatalf("CommitApproval() error = %v, want ErrApprovalConflict for a distinct canonical payload", err)
	}
}

func TestGetApprovedInventoryPreservesExplicitlyEmptyOperations(t *testing.T) {
	db := openRepositoryTestDB(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	commit := repositoryTestCommit("tenant-a", "product-1", "approve-1", "asset-1")
	commit.Assets[0].Operations = []string{}
	if _, err := repo.CommitApproval(context.Background(), commit); err != nil {
		t.Fatal(err)
	}
	inventory, err := repo.GetApprovedInventory(context.Background(), productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"})
	if err != nil {
		t.Fatal(err)
	}
	if inventory.Assets[0].Operations == nil {
		t.Fatal("inventory operations = nil, want explicitly empty slice preserved")
	}
}

func TestCommitApprovalConcurrentReplayIsIdempotent(t *testing.T) {
	db := openConcurrentRepositoryTestDB(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	commit := repositoryTestCommit("tenant-a", "product-1", "approve-1", "asset-1")
	const callers = 8
	start := make(chan struct{})
	errorsByCaller := make(chan error, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			receipt, commitErr := repo.CommitApproval(context.Background(), commit)
			if commitErr == nil && (receipt.ActionID != "approve-1" || len(receipt.AssetIDs) != 1 || receipt.AssetIDs[0] != "asset-1") {
				commitErr = fmt.Errorf("receipt = %+v", receipt)
			}
			errorsByCaller <- commitErr
		}()
	}
	close(start)
	wg.Wait()
	close(errorsByCaller)
	for commitErr := range errorsByCaller {
		if commitErr != nil {
			t.Fatalf("concurrent CommitApproval() error = %v", commitErr)
		}
	}
	var assetCount, receiptCount int64
	if err := db.Model(&ApprovedAssetRecord{}).Count(&assetCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&ApprovalReceiptRecord{}).Count(&receiptCount).Error; err != nil {
		t.Fatal(err)
	}
	if assetCount != 1 || receiptCount != 1 {
		t.Fatalf("concurrent replay rows = assets:%d receipts:%d, want 1 and 1", assetCount, receiptCount)
	}
}

func openRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "approved-assets.sqlite")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func openConcurrentRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "approved-assets-concurrent.sqlite")) + "?_busy_timeout=5000&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open concurrent sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func repositoryTestCommit(tenantID, productKey, actionID, assetID string) productasset.ApprovalCommit {
	return productasset.ApprovalCommit{
		TenantID: tenantID, ProductKey: productKey, ActionID: actionID,
		Assets: []productasset.ApprovedAsset{{
			ID: assetID, RunID: "run-1", PlanRevision: 2, SlotID: "main", Attempt: 1,
			Role: productasset.RoleMain, URL: "https://cdn.example/" + assetID + ".png",
			Width: 1200, Height: 1200, Operations: []string{"remove_background", "approve"},
		}},
	}
}
