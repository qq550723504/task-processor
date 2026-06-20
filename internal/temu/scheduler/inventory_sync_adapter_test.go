package scheduler

import (
	"context"
	"strings"
	"testing"

	platformtask "task-processor/internal/platformtask"
	temusync "task-processor/internal/temu/sync"
)

type mockTemuInventorySyncService struct {
	fetchFunc   func(ctx context.Context, tenantID, storeID int64) ([]*temusync.TemuInventoryProductSnapshot, error)
	monitorFunc func(ctx context.Context, products []*temusync.TemuInventoryProductSnapshot, tenantID, storeID int64) (*temusync.MonitorResult, error)
}

func (m *mockTemuInventorySyncService) FetchProductsForInventorySync(ctx context.Context, tenantID, storeID int64) ([]*temusync.TemuInventoryProductSnapshot, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(ctx, tenantID, storeID)
	}
	return nil, nil
}

func (m *mockTemuInventorySyncService) MonitorInventoryChanges(ctx context.Context, products []*temusync.TemuInventoryProductSnapshot, tenantID, storeID int64) (*temusync.MonitorResult, error) {
	if m.monitorFunc != nil {
		return m.monitorFunc(ctx, products, tenantID, storeID)
	}
	return &temusync.MonitorResult{}, nil
}

func TestNewInventorySyncServiceAdapter(t *testing.T) {
	var svc temusync.InventorySyncService = &mockTemuInventorySyncService{}
	adapter := newInventorySyncServiceAdapter(svc)
	if adapter == nil {
		t.Fatal("newInventorySyncServiceAdapter returned nil")
	}
}

func TestInventorySyncServiceAdapter_FetchProductsForInventorySync(t *testing.T) {
	expected := []*temusync.TemuInventoryProductSnapshot{
		{ProductID: "p1"},
		{ProductID: "p2"},
	}
	adapter := newInventorySyncServiceAdapter(&mockTemuInventorySyncService{
		fetchFunc: func(ctx context.Context, tenantID, storeID int64) ([]*temusync.TemuInventoryProductSnapshot, error) {
			return expected, nil
		},
	})

	got, err := adapter.FetchProductsForInventorySync(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("FetchProductsForInventorySync returned error: %v", err)
	}
	if len(got) != len(expected) {
		t.Fatalf("expected %d products, got %d", len(expected), len(got))
	}
	for i, product := range got {
		snapshot, ok := product.(*temusync.TemuInventoryProductSnapshot)
		if !ok {
			t.Fatalf("result[%d] type = %T, want *TemuInventoryProductSnapshot", i, product)
		}
		if snapshot != expected[i] {
			t.Fatalf("result[%d] pointer mismatch", i)
		}
	}
}

func TestInventorySyncServiceAdapter_MonitorInventoryChangesRejectsUnexpectedType(t *testing.T) {
	adapter := newInventorySyncServiceAdapter(&mockTemuInventorySyncService{})

	_, err := adapter.MonitorInventoryChanges(context.Background(), []any{"bad"}, 1, 2)
	if err == nil {
		t.Fatal("expected type assertion error")
	}
	if !strings.Contains(err.Error(), "*TemuInventoryProductSnapshot") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInventorySyncServiceAdapter_MonitorInventoryChangesConvertsResult(t *testing.T) {
	input := []*temusync.TemuInventoryProductSnapshot{{ProductID: "p1"}}
	expected := &temusync.MonitorResult{
		TotalProducts:     1,
		ProcessedProducts: 1,
		PriceChanges:      2,
		StockChanges:      3,
		AmazonFetched:     4,
		AmazonFailed:      5,
	}
	adapter := newInventorySyncServiceAdapter(&mockTemuInventorySyncService{
		monitorFunc: func(ctx context.Context, products []*temusync.TemuInventoryProductSnapshot, tenantID, storeID int64) (*temusync.MonitorResult, error) {
			if len(products) != 1 || products[0] != input[0] {
				t.Fatalf("unexpected products: %#v", products)
			}
			return expected, nil
		},
	})

	got, err := adapter.MonitorInventoryChanges(context.Background(), []any{input[0]}, 1, 2)
	if err != nil {
		t.Fatalf("MonitorInventoryChanges returned error: %v", err)
	}

	want := &platformtask.InventorySyncResult{
		TotalProducts:     1,
		ProcessedProducts: 1,
		SkippedProducts:   0,
		PriceChanges:      2,
		StockChanges:      3,
		AmazonFetched:     4,
		AmazonFailed:      5,
	}
	if *got != *want {
		t.Fatalf("result = %#v, want %#v", got, want)
	}
}
