package assettest

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"task-processor/internal/product/asset"
)

func TestCommitApprovalCanceledBeforeLockAcquisitionDoesNotWrite(t *testing.T) {
	t.Parallel()

	repo := NewMemoryRepository()
	repo.mu.Lock()
	ctx := newLockContentionContext()
	result := make(chan error, 1)
	go func() {
		_, err := repo.CommitApproval(ctx, contractCommit("tenant-a", "product-1", "approve-1", "asset-1"))
		result <- err
	}()

	ctx.waitForInitialCheck(t)
	ctx.cancel()
	repo.mu.Unlock()

	if err := waitRepositoryResult(t, result); !errors.Is(err, context.Canceled) {
		t.Fatalf("CommitApproval() error = %v, want context.Canceled", err)
	}
	_, err := repo.GetApprovedInventory(context.Background(), asset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"})
	if !errors.Is(err, asset.ErrApprovedAssetsNotReady) {
		t.Fatalf("inventory after canceled commit error = %v, want ErrApprovedAssetsNotReady", err)
	}
}

func TestGetApprovedInventoryCanceledBeforeLockAcquisitionReturnsNoInventory(t *testing.T) {
	t.Parallel()

	repo := NewMemoryRepository()
	_, err := repo.CommitApproval(context.Background(), contractCommit("tenant-a", "product-1", "approve-1", "asset-1"))
	if err != nil {
		t.Fatal(err)
	}

	repo.mu.Lock()
	ctx := newLockContentionContext()
	type readResult struct {
		inventory asset.ApprovedAssetInventory
		err       error
	}
	result := make(chan readResult, 1)
	go func() {
		inventory, err := repo.GetApprovedInventory(ctx, asset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"})
		result <- readResult{inventory: inventory, err: err}
	}()

	ctx.waitForInitialCheck(t)
	ctx.cancel()
	repo.mu.Unlock()

	got := waitRepositoryResult(t, result)
	if !errors.Is(got.err, context.Canceled) {
		t.Fatalf("GetApprovedInventory() error = %v, want context.Canceled", got.err)
	}
	if len(got.inventory.Assets) != 0 {
		t.Fatalf("GetApprovedInventory() = %+v, want no result after cancellation", got.inventory)
	}
}

type lockContentionContext struct {
	firstErrChecked chan struct{}
	done            chan struct{}
	firstOnce       sync.Once
	cancelOnce      sync.Once
	canceled        atomic.Bool
}

func newLockContentionContext() *lockContentionContext {
	return &lockContentionContext{
		firstErrChecked: make(chan struct{}),
		done:            make(chan struct{}),
	}
}

func (c *lockContentionContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *lockContentionContext) Done() <-chan struct{}       { return c.done }
func (c *lockContentionContext) Value(any) any               { return nil }

func (c *lockContentionContext) Err() error {
	if c.canceled.Load() {
		return context.Canceled
	}
	c.firstOnce.Do(func() { close(c.firstErrChecked) })
	return nil
}

func (c *lockContentionContext) cancel() {
	c.cancelOnce.Do(func() {
		c.canceled.Store(true)
		close(c.done)
	})
}

func (c *lockContentionContext) waitForInitialCheck(t *testing.T) {
	t.Helper()
	select {
	case <-c.firstErrChecked:
	case <-time.After(5 * time.Second):
		t.Fatal("repository did not complete its initial context check")
	}
}

func waitRepositoryResult[T any](t *testing.T, result <-chan T) T {
	t.Helper()
	select {
	case got := <-result:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("repository operation did not return after lock release")
		var zero T
		return zero
	}
}
