package scheduler

import (
	"context"
	"errors"
	"strings"
	"testing"

	models "task-processor/internal/temu/api/product"
	temusync "task-processor/internal/temu/sync"
)

// MockTemuProductSyncService 模拟Temu产品同步服务
type MockTemuProductSyncService struct {
	FetchProductListFunc func(ctx context.Context) ([]models.GoodsSearchItem, error)
	ConvertProductsFunc  func(ctx context.Context, products []models.GoodsSearchItem, tenantID, storeID int64) ([]*temusync.TemuProductSnapshot, error)
	SaveProductsFunc     func(ctx context.Context, products []*temusync.TemuProductSnapshot) (int, error)
}

func (m *MockTemuProductSyncService) FetchProductList(ctx context.Context) ([]models.GoodsSearchItem, error) {
	if m.FetchProductListFunc != nil {
		return m.FetchProductListFunc(ctx)
	}
	return []models.GoodsSearchItem{}, nil
}

func (m *MockTemuProductSyncService) ConvertProducts(ctx context.Context, products []models.GoodsSearchItem, tenantID, storeID int64) ([]*temusync.TemuProductSnapshot, error) {
	if m.ConvertProductsFunc != nil {
		return m.ConvertProductsFunc(ctx, products, tenantID, storeID)
	}
	return []*temusync.TemuProductSnapshot{}, nil
}

func (m *MockTemuProductSyncService) SaveProducts(ctx context.Context, products []*temusync.TemuProductSnapshot) (int, error) {
	if m.SaveProductsFunc != nil {
		return m.SaveProductsFunc(ctx, products)
	}
	return len(products), nil
}

func TestNewTemuProductSyncServiceAdapter(t *testing.T) {
	mockService := &MockTemuProductSyncService{}
	adapter := newProductSyncServiceAdapter(mockService)

	if adapter == nil {
		t.Fatal("newProductSyncServiceAdapter returned nil")
	}
}

func TestTemuProductSyncServiceAdapter_FetchProductList_Success(t *testing.T) {
	mockProducts := []models.GoodsSearchItem{
		{GoodsID: "1", GoodsName: "Product 1"},
		{GoodsID: "2", GoodsName: "Product 2"},
	}

	mockService := &MockTemuProductSyncService{
		FetchProductListFunc: func(ctx context.Context) ([]models.GoodsSearchItem, error) {
			return mockProducts, nil
		},
	}

	adapter := newProductSyncServiceAdapter(mockService)
	ctx := context.Background()

	results, err := adapter.FetchProductList(ctx)

	if err != nil {
		t.Errorf("FetchProductList should succeed, got error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 products, got %d", len(results))
	}
}

func TestTemuProductSyncServiceAdapter_FetchProductList_Error(t *testing.T) {
	expectedError := errors.New("fetch failed")

	mockService := &MockTemuProductSyncService{
		FetchProductListFunc: func(ctx context.Context) ([]models.GoodsSearchItem, error) {
			return nil, expectedError
		},
	}

	adapter := newProductSyncServiceAdapter(mockService)
	ctx := context.Background()

	_, err := adapter.FetchProductList(ctx)

	if err == nil {
		t.Error("FetchProductList should fail")
	}

	if !errors.Is(err, expectedError) {
		t.Errorf("Expected error to be fetch error, got: %v", err)
	}
}

func TestTemuProductSyncServiceAdapter_ConvertProducts_Success(t *testing.T) {
	mockInput := []any{
		models.GoodsSearchItem{GoodsID: "1", GoodsName: "Product 1"},
		models.GoodsSearchItem{GoodsID: "2", GoodsName: "Product 2"},
	}

	mockOutput := []*temusync.TemuProductSnapshot{
		{ProductID: "1"},
		{ProductID: "2"},
	}

	mockService := &MockTemuProductSyncService{
		ConvertProductsFunc: func(ctx context.Context, products []models.GoodsSearchItem, tenantID, storeID int64) ([]*temusync.TemuProductSnapshot, error) {
			return mockOutput, nil
		},
	}

	adapter := newProductSyncServiceAdapter(mockService)
	ctx := context.Background()

	results, err := adapter.ConvertProducts(ctx, mockInput, 1, 100)

	if err != nil {
		t.Errorf("ConvertProducts should succeed, got error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 products, got %d", len(results))
	}
	if _, ok := results[0].(*temusync.TemuProductSnapshot); !ok {
		t.Fatalf("Expected snapshot result, got %T", results[0])
	}
}

func TestTemuProductSyncServiceAdapter_ConvertProducts_Error(t *testing.T) {
	expectedError := errors.New("convert failed")

	mockInput := []any{
		models.GoodsSearchItem{GoodsID: "1"},
	}

	mockService := &MockTemuProductSyncService{
		ConvertProductsFunc: func(ctx context.Context, products []models.GoodsSearchItem, tenantID, storeID int64) ([]*temusync.TemuProductSnapshot, error) {
			return nil, expectedError
		},
	}

	adapter := newProductSyncServiceAdapter(mockService)
	ctx := context.Background()

	_, err := adapter.ConvertProducts(ctx, mockInput, 1, 100)

	if err == nil {
		t.Error("ConvertProducts should fail")
	}

	if !errors.Is(err, expectedError) {
		t.Errorf("Expected error to be convert error, got: %v", err)
	}
}

func TestTemuProductSyncServiceAdapter_SaveProducts_Success(t *testing.T) {
	mockInput := []any{
		&temusync.TemuProductSnapshot{ProductID: "1"},
		&temusync.TemuProductSnapshot{ProductID: "2"},
	}

	mockService := &MockTemuProductSyncService{
		SaveProductsFunc: func(ctx context.Context, products []*temusync.TemuProductSnapshot) (int, error) {
			return len(products), nil
		},
	}

	adapter := newProductSyncServiceAdapter(mockService)
	ctx := context.Background()

	count, err := adapter.SaveProducts(ctx, mockInput)

	if err != nil {
		t.Errorf("SaveProducts should succeed, got error: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected count=2, got %d", count)
	}
}

func TestTemuProductSyncServiceAdapter_SaveProducts_RejectsUnexpectedType(t *testing.T) {
	mockService := &MockTemuProductSyncService{}
	adapter := newProductSyncServiceAdapter(mockService)

	_, err := adapter.SaveProducts(context.Background(), []any{"bad"})
	if err == nil {
		t.Fatal("SaveProducts should fail")
	}
	if !strings.Contains(err.Error(), "*TemuProductSnapshot") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTemuProductSyncServiceAdapter_SaveProducts_Error(t *testing.T) {
	expectedError := errors.New("save failed")

	mockInput := []any{
		&temusync.TemuProductSnapshot{ProductID: "1"},
	}

	mockService := &MockTemuProductSyncService{
		SaveProductsFunc: func(ctx context.Context, products []*temusync.TemuProductSnapshot) (int, error) {
			return 0, expectedError
		},
	}

	adapter := newProductSyncServiceAdapter(mockService)
	ctx := context.Background()

	_, err := adapter.SaveProducts(ctx, mockInput)

	if err == nil {
		t.Error("SaveProducts should fail")
	}

	if !errors.Is(err, expectedError) {
		t.Errorf("Expected error to be save error, got: %v", err)
	}
}
