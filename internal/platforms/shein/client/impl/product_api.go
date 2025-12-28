// Package impl 提供SHEIN客户端API实现
package impl

import (
	"task-processor/internal/platforms/shein/client/api/product"
)

// ProductAPI 产品相关API实现
type ProductAPI struct {
	*BaseAPIClient

	// 注入的专职处理器
	productManager   *ProductManager
	inventoryManager *InventoryManager
	priceManager     *PriceManager
	errorHandler     *APIErrorHandler
}

// NewProductAPI 创建新的产品API实现
func NewProductAPI(baseClient *BaseAPIClient) *ProductAPI {
	// 创建专职处理器
	errorHandler := NewAPIErrorHandler(baseClient)
	productManager := NewProductManager(baseClient, errorHandler)
	inventoryManager := NewInventoryManager(baseClient, errorHandler)
	priceManager := NewPriceManager(baseClient, errorHandler)

	return &ProductAPI{
		BaseAPIClient:    baseClient,
		productManager:   productManager,
		inventoryManager: inventoryManager,
		priceManager:     priceManager,
		errorHandler:     errorHandler,
	}
}

// GetProduct 获取产品信息
func (p *ProductAPI) GetProduct(productID string) (*product.Product, error) {
	return p.productManager.GetProduct(productID)
}

// UpdateProduct 更新产品信息
func (p *ProductAPI) UpdateProduct(product *product.Product) error {
	return p.productManager.UpdateProduct(product)
}

// DeleteProduct 删除产品
func (p *ProductAPI) DeleteProduct(productID string) error {
	return p.productManager.DeleteProduct(productID)
}

// GetPartInfo 获取部件信息
func (p *ProductAPI) GetPartInfo(categoryID int) (*product.PartInfoResponse, error) {
	return p.productManager.GetPartInfo(categoryID)
}

// SaveDraftProduct 保存产品草稿
func (p *ProductAPI) SaveDraftProduct(prod *product.Product) (*product.SheinResponse, string, error) {
	return p.productManager.SaveDraftProduct(prod)
}

// PublishProduct 发布产品
func (p *ProductAPI) PublishProduct(prod *product.Product) (*product.SheinResponse, string, error) {
	return p.productManager.PublishProduct(prod)
}

// ConfirmPublish 确认发布产品
func (p *ProductAPI) ConfirmPublish(product *product.Product) (bool, string, error) {
	return p.productManager.ConfirmPublish(product)
}

// Record 获取产品发布记录
func (p *ProductAPI) Record(request *product.ProductRecordRequest) (*product.RecordResponse, error) {
	return p.productManager.Record(request)
}

// ListProducts 获取产品列表
func (p *ProductAPI) ListProducts(pageNum, pageSize int, request *product.ProductListRequest) (*product.ProductListResponse, error) {
	return p.productManager.ListProducts(pageNum, pageSize, request)
}

// QueryStock 查询产品库存
func (p *ProductAPI) QueryStock(request *product.StockQueryRequest) (*product.StockQueryResponse, error) {
	return p.inventoryManager.QueryStock(request)
}

// QueryInventory 查询产品库存详情
func (p *ProductAPI) QueryInventory(spuName string) (*product.InventoryQueryResponse, error) {
	return p.inventoryManager.QueryInventory(spuName)
}

// UpdateInventory 更新产品库存
func (p *ProductAPI) UpdateInventory(request *product.InventoryUpdateRequest) error {
	return p.inventoryManager.UpdateInventory(request)
}

// QueryPrice 查询产品价格
func (p *ProductAPI) QueryPrice(spuName string) (*product.PriceQueryResponse, error) {
	return p.priceManager.QueryPrice(spuName)
}

// QueryCostPrice 查询成本价格（半托店铺）
func (p *ProductAPI) QueryCostPrice(spuName string, skcNameList []string) (*product.CostPriceQueryResponse, error) {
	return p.priceManager.QueryCostPrice(spuName, skcNameList)
}

// OffShelf 产品下架
func (p *ProductAPI) OffShelf(request *product.ShelfOperateRequest) error {
	return p.productManager.OffShelf(request)
}

// OnShelf 产品上架
func (p *ProductAPI) OnShelf(request *product.ShelfOperateRequest) error {
	return p.productManager.OnShelf(request)
}
