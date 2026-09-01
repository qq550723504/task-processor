// Package assettest provides test-only repository implementations and a
// reusable contract suite. Production composition must never import it.
package assettest

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"task-processor/internal/product/asset"
)

type MemoryRepository struct {
	mu          sync.RWMutex
	actions     map[actionKey]actionRecord
	inventories map[inventoryKey][]asset.ApprovedAsset
	assetIDs    map[tenantAssetKey]struct{}
}

type actionKey struct {
	tenantID string
	actionID string
}

type inventoryKey struct {
	tenantID   string
	productKey string
}

type tenantAssetKey struct {
	tenantID string
	assetID  string
}

type actionRecord struct {
	commit  asset.ApprovalCommit
	receipt asset.ApprovalReceipt
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		actions:     make(map[actionKey]actionRecord),
		inventories: make(map[inventoryKey][]asset.ApprovedAsset),
		assetIDs:    make(map[tenantAssetKey]struct{}),
	}
}

func (r *MemoryRepository) CommitApproval(ctx context.Context, commit asset.ApprovalCommit) (asset.ApprovalReceipt, error) {
	if err := ctx.Err(); err != nil {
		return asset.ApprovalReceipt{}, err
	}
	if err := asset.ValidateApprovalCommit(commit); err != nil {
		return asset.ApprovalReceipt{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return asset.ApprovalReceipt{}, err
	}

	key := actionKey{tenantID: commit.TenantID, actionID: commit.ActionID}
	if existing, ok := r.actions[key]; ok {
		if !reflect.DeepEqual(existing.commit, commit) {
			return asset.ApprovalReceipt{}, asset.ErrApprovalConflict
		}
		return asset.CloneApprovalReceipt(existing.receipt), nil
	}
	for _, approved := range commit.Assets {
		if _, exists := r.assetIDs[tenantAssetKey{tenantID: commit.TenantID, assetID: approved.ID}]; exists {
			return asset.ApprovalReceipt{}, asset.ErrApprovalConflict
		}
	}

	receipt := asset.ApprovalReceipt{ActionID: commit.ActionID, AssetIDs: make([]string, len(commit.Assets))}
	for index, approved := range commit.Assets {
		receipt.AssetIDs[index] = approved.ID
	}
	storedCommit := asset.CloneApprovalCommit(commit)
	storedReceipt := asset.CloneApprovalReceipt(receipt)
	r.actions[key] = actionRecord{commit: storedCommit, receipt: storedReceipt}
	invKey := inventoryKey{tenantID: commit.TenantID, productKey: commit.ProductKey}
	r.inventories[invKey] = append(r.inventories[invKey], asset.CloneApprovalCommit(commit).Assets...)
	for _, approved := range commit.Assets {
		r.assetIDs[tenantAssetKey{tenantID: commit.TenantID, assetID: approved.ID}] = struct{}{}
	}
	return asset.CloneApprovalReceipt(storedReceipt), nil
}

func (r *MemoryRepository) GetApprovedInventory(ctx context.Context, scope asset.InventoryScope) (asset.ApprovedAssetInventory, error) {
	if err := ctx.Err(); err != nil {
		return asset.ApprovedAssetInventory{}, err
	}
	if err := asset.ValidateInventoryScope(scope); err != nil {
		return asset.ApprovedAssetInventory{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return asset.ApprovedAssetInventory{}, err
	}
	approved, ok := r.inventories[inventoryKey{tenantID: scope.TenantID, productKey: scope.ProductKey}]
	if !ok || len(approved) == 0 {
		return asset.ApprovedAssetInventory{}, asset.ErrApprovedAssetsNotReady
	}
	return asset.CloneApprovedAssetInventory(asset.ApprovedAssetInventory{Scope: scope, Assets: approved}), nil
}

type RepositoryFactory func(t *testing.T) asset.Repository

// ExerciseRepositoryContract verifies observable Repository behavior against
// a real implementation. Adapter packages reuse it with their own factory.
func ExerciseRepositoryContract(t *testing.T, factory RepositoryFactory) {
	t.Helper()

	t.Run("idempotent approval exposes only committed assets", func(t *testing.T) {
		repo := factory(t)
		commit := contractCommit("tenant-a", "product-1", "approve-1", "asset-1")
		first, err := repo.CommitApproval(context.Background(), commit)
		assertNoError(t, err)
		second, err := repo.CommitApproval(context.Background(), commit)
		assertNoError(t, err)
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("idempotent receipts differ: first=%+v second=%+v", first, second)
		}
		inventory, err := repo.GetApprovedInventory(context.Background(), asset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"})
		assertNoError(t, err)
		if len(inventory.Assets) != 1 || inventory.Assets[0].ID != "asset-1" {
			t.Fatalf("inventory = %+v, want exactly approved asset-1", inventory)
		}
	})

	t.Run("valid empty inventory is explicitly not ready", func(t *testing.T) {
		repo := factory(t)
		_, err := repo.GetApprovedInventory(context.Background(), asset.InventoryScope{TenantID: "tenant-a", ProductKey: "missing"})
		if !errors.Is(err, asset.ErrApprovedAssetsNotReady) {
			t.Fatalf("GetApprovedInventory() error = %v, want ErrApprovedAssetsNotReady", err)
		}
	})

	t.Run("tenant isolation permits the same action and asset ids", func(t *testing.T) {
		repo := factory(t)
		for _, tenantID := range []string{"tenant-a", "tenant-b"} {
			_, err := repo.CommitApproval(context.Background(), contractCommit(tenantID, "product-1", "approve-1", "asset-1"))
			assertNoError(t, err)
		}
		for _, tenantID := range []string{"tenant-a", "tenant-b"} {
			inventory, err := repo.GetApprovedInventory(context.Background(), asset.InventoryScope{TenantID: tenantID, ProductKey: "product-1"})
			assertNoError(t, err)
			if len(inventory.Assets) != 1 || inventory.Scope.TenantID != tenantID {
				t.Fatalf("inventory for %s = %+v, want isolated asset", tenantID, inventory)
			}
		}
		_, err := repo.GetApprovedInventory(context.Background(), asset.InventoryScope{TenantID: "tenant-c", ProductKey: "product-1"})
		if !errors.Is(err, asset.ErrApprovedAssetsNotReady) {
			t.Fatalf("cross-tenant read error = %v, want ErrApprovedAssetsNotReady", err)
		}
	})

	t.Run("same action with a different payload conflicts", func(t *testing.T) {
		repo := factory(t)
		commit := contractCommit("tenant-a", "product-1", "approve-1", "asset-1")
		_, err := repo.CommitApproval(context.Background(), commit)
		assertNoError(t, err)
		commit.Assets[0].URL = "https://cdn.example/replaced.png"
		_, err = repo.CommitApproval(context.Background(), commit)
		if !errors.Is(err, asset.ErrApprovalConflict) {
			t.Fatalf("CommitApproval() error = %v, want ErrApprovalConflict", err)
		}
	})

	t.Run("idempotency preserves an explicitly empty operations list", func(t *testing.T) {
		repo := factory(t)
		commit := contractCommit("tenant-a", "product-1", "approve-1", "asset-1")
		commit.Assets[0].Operations = []string{}
		first, err := repo.CommitApproval(context.Background(), commit)
		assertNoError(t, err)
		second, err := repo.CommitApproval(context.Background(), commit)
		assertNoError(t, err)
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("idempotent receipts differ: first=%+v second=%+v", first, second)
		}
	})

	t.Run("asset id uniqueness is tenant qualified", func(t *testing.T) {
		repo := factory(t)
		_, err := repo.CommitApproval(context.Background(), contractCommit("tenant-a", "product-1", "approve-1", "asset-1"))
		assertNoError(t, err)
		_, err = repo.CommitApproval(context.Background(), contractCommit("tenant-a", "product-2", "approve-2", "asset-1"))
		if !errors.Is(err, asset.ErrApprovalConflict) {
			t.Fatalf("same-tenant duplicate asset error = %v, want ErrApprovalConflict", err)
		}
		_, err = repo.CommitApproval(context.Background(), contractCommit("tenant-b", "product-2", "approve-2", "asset-1"))
		assertNoError(t, err)
	})

	t.Run("stored approvals and returned values are immutable copies", func(t *testing.T) {
		repo := factory(t)
		commit := contractCommit("tenant-a", "product-1", "approve-1", "asset-1")
		receipt, err := repo.CommitApproval(context.Background(), commit)
		assertNoError(t, err)
		commit.Assets[0].Operations[0] = "caller-mutated"
		receipt.AssetIDs[0] = "caller-mutated"

		inventory, err := repo.GetApprovedInventory(context.Background(), asset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"})
		assertNoError(t, err)
		if inventory.Assets[0].Operations[0] != "remove_background" {
			t.Fatalf("stored operations = %v, want defensive input copy", inventory.Assets[0].Operations)
		}
		inventory.Assets[0].Operations[0] = "output-mutated"
		again, err := repo.GetApprovedInventory(context.Background(), asset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"})
		assertNoError(t, err)
		if again.Assets[0].Operations[0] != "remove_background" {
			t.Fatalf("stored operations = %v, want defensive output copy", again.Assets[0].Operations)
		}
	})

	t.Run("canceled operations do not read or write", func(t *testing.T) {
		repo := factory(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := repo.CommitApproval(ctx, contractCommit("tenant-a", "product-1", "approve-1", "asset-1"))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("CommitApproval(canceled) error = %v, want context.Canceled", err)
		}
		_, err = repo.GetApprovedInventory(ctx, asset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("GetApprovedInventory(canceled) error = %v, want context.Canceled", err)
		}
	})
}

func contractCommit(tenantID, productKey, actionID, assetID string) asset.ApprovalCommit {
	return asset.ApprovalCommit{
		TenantID: tenantID, ProductKey: productKey, ActionID: actionID,
		Assets: []asset.ApprovedAsset{{
			ID: assetID, RunID: "run-1", PlanRevision: 2, SlotID: "main", Attempt: 1,
			Role: asset.RoleMain, URL: "https://cdn.example/" + assetID + ".png",
			Width: 1200, Height: 1200, Operations: []string{"remove_background", "approve"},
		}},
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

var _ asset.Repository = (*MemoryRepository)(nil)
