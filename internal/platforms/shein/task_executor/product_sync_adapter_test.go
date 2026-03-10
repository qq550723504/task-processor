package scheduler

import (
	"context"
	"errors"
	"testing"

	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/api/product"
)

// MockSheinProductSyncService 模拟Shein产品同步服务
type MockSheinProductSyncService struct {
	FetchProductListFunc func(ctx context.Context) ([]product.ProductListItem, error)
	ConvertProductsFunc  func(ctx context.Context, products []product.ProductListItem, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error)
	SaveProductsFunc     func(ctx context.Context, products []*managementapi.ProductDataDTO) (int, error)
}

func (m *MockSheinProductSyncService) FetchProductList(ctx context.Context) ([]product.ProductListItem, error) {
	if m.FetchProductListFunc != nil {
		return m.FetchProductListFunc(ctx)
	}
	return []product.ProductListItem{}, nil
}

func (m *MockSheinProductSyncService) ConvertProducts(ctx context.Context, products []product.ProductListItem, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error) {
	if m.ConvertProductsFunc != nil {
		return m.ConvertProductsFunc(ctx, products, tenantID, storeID)
	}
	return []*managementapi.ProductDataDTO{}, nil
}

func (m *MockSheinProductSyncService) SaveProducts(ctx context.Context, products []*managementapi.ProductDataDTO) (int, error) {
	if m.SaveProductsFunc != nil {
		return m.SaveProductsFunc(ctx, products)
	}
	return len(products), nil
}

func TestNewProductSyncServiceAdapter(t *testing.T) {
	mockService := &MockSheinProductSyncService{}
	adapter := newProductSyncServiceAdapter(mockService)

	if adapter == nil {
		t.Fatal("newProductSyncServiceAdapter returned nil")
	}
}

func TestProductSyncServiceAdapter_FetchProductList_Success(t *testing.T) {
	mockProducts := []product.ProductListItem{
		{SpuCode: "1", SpuName: "Product 1"},
		{SpuCode: "2", SpuName: "Product 2"},
	}

	mockService := &MockSheinProductSyncService{
		FetchProductListFunc: func(ctx context.Context) ([]product.ProductListItem, error) {
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

func TestProductSyncServiceAdapter_FetchProductList_Error(t *testing.T) {
	expectedError := errors.New("fetch failed")

	mockService := &MockSheinProductSyncService{
		FetchProductListFunc: func(ctx context.Context) ([]product.ProductListItem, error) {
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

func TestProductSyncServiceAdapter_ConvertProducts_Success(t *testing.T) {
	mockInput := []interface{}{
		product.ProductListItem{SpuCode: "1", SpuName: "Product 1"},
		product.ProductListItem{SpuCode: "2", SpuName: "Product 2"},
	}

	mockOutput := []*managementapi.ProductDataDTO{
		{ProductID: "1"},
		{ProductID: "2"},
	}

	mockService := &MockSheinProductSyncService{
		ConvertProductsFunc: func(ctx context.Context, products []product.ProductListItem, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error) {
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

func TestProductSyncServiceAdapter_ConvertProducts_Error(t *testing.T) {
	expectedError := errors.New("convert failed")

	mockInput := []interface{}{
		product.ProductListItem{SpuCode: "1"},
	}

	mockService := &MockSheinProductSyncService{
		ConvertProductsFunc: func(ctx context.Context, products []product.ProductListItem, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error) {
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

func TestProductSyncServiceAdapter_SaveProducts_Success(t *testing.T) {
	mockInput := []interface{}{
		&managementapi.ProductDataDTO{ProductID: "1"},
		&managementapi.ProductDataDTO{ProductID: "2"},
	}

	mockService := &MockSheinProductSyncService{
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

func TestProductSyncServiceAdapter_SaveProducts_Error(t *testing.T) {
	expectedError := errors.New("save failed")

	mockInput := []interface{}{
		&managementapi.ProductDataDTO{ProductID: "1"},
	}

	mockService := &MockSheinProductSyncService{
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
