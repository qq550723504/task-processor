package scheduler

import (
	"context"
	"errors"
	"testing"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/platforms/temu/api/models"
)

// MockTemuProductSyncService 模拟Temu产品同步服务
type MockTemuProductSyncService struct {
	FetchProductListFunc func(ctx context.Context) ([]models.GoodsSearchItem, error)
	ConvertProductsFunc  func(ctx context.Context, products []models.GoodsSearchItem, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error)
	SaveProductsFunc     func(ctx context.Context, products []*managementapi.ProductDataDTO) (int, error)
}

func (m *MockTemuProductSyncService) FetchProductList(ctx context.Context) ([]models.GoodsSearchItem, error) {
	if m.FetchProductListFunc != nil {
		return m.FetchProductListFunc(ctx)
	}
	return []models.GoodsSearchItem{}, nil
}

func (m *MockTemuProductSyncService) ConvertProducts(ctx context.Context, products []models.GoodsSearchItem, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error) {
	if m.ConvertProductsFunc != nil {
		return m.ConvertProductsFunc(ctx, products, tenantID, storeID)
	}
	return []*managementapi.ProductDataDTO{}, nil
}

func (m *MockTemuProductSyncService) SaveProducts(ctx context.Context, products []*managementapi.ProductDataDTO) (int, error) {
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
	mockInput := []interface{}{
		models.GoodsSearchItem{GoodsID: "1", GoodsName: "Product 1"},
		models.GoodsSearchItem{GoodsID: "2", GoodsName: "Product 2"},
	}

	mockOutput := []*managementapi.ProductDataDTO{
		{ProductID: "1"},
		{ProductID: "2"},
	}

	mockService := &MockTemuProductSyncService{
		ConvertProductsFunc: func(ctx context.Context, products []models.GoodsSearchItem, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error) {
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
}

func TestTemuProductSyncServiceAdapter_ConvertProducts_Error(t *testing.T) {
	expectedError := errors.New("convert failed")

	mockInput := []interface{}{
		models.GoodsSearchItem{GoodsID: "1"},
	}

	mockService := &MockTemuProductSyncService{
		ConvertProductsFunc: func(ctx context.Context, products []models.GoodsSearchItem, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error) {
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
	mockInput := []interface{}{
		&managementapi.ProductDataDTO{ProductID: "1"},
		&managementapi.ProductDataDTO{ProductID: "2"},
	}

	mockService := &MockTemuProductSyncService{
		SaveProductsFunc: func(ctx context.Context, products []*managementapi.ProductDataDTO) (int, error) {
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

func TestTemuProductSyncServiceAdapter_SaveProducts_Error(t *testing.T) {
	expectedError := errors.New("save failed")

	mockInput := []interface{}{
		&managementapi.ProductDataDTO{ProductID: "1"},
	}

	mockService := &MockTemuProductSyncService{
		SaveProductsFunc: func(ctx context.Context, products []*managementapi.ProductDataDTO) (int, error) {
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
