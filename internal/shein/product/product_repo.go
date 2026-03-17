// Package product 提供SHEIN通用客户端API实现
package product

import (
	"fmt"
	"net/http"
	"task-processor/internal/shein/api"
	"task-processor/internal/shein/api/product"
	"task-processor/internal/shein/client"
	"task-processor/internal/shein/pricing"
)

// ProductAPI 产品相关API实现
type ProductAPI struct {
	*client.BaseAPIClient

	// 注入的专职处理器
	productManager   *ProductManager
	inventoryManager *InventoryManager
	priceManager     *pricing.PriceManager
	errorHandler     *client.APIErrorHandler
}

// NewProductAPI 创建新的产品API实现
func NewProductAPI(baseClient *client.BaseAPIClient) *ProductAPI {
	// 创建专职处理器
	errorHandler := client.NewAPIErrorHandler(baseClient)
	productManager := NewProductManager(baseClient, errorHandler)
	inventoryManager := NewInventoryManager(baseClient, errorHandler)
	priceManager := pricing.NewPriceManager(baseClient, errorHandler)

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

// operateShelf 通用的上下架操作
func (p *ProductAPI) operateShelf(request *product.ShelfOperateRequest, operateType, errorMsg string) error {
	url := fmt.Sprintf("%s%s", p.BaseAPIClient.GetBaseURL(), client.GetOperateShelfStatusEndpoint())

	// 构建请求体
	reqBody := struct {
		*product.ShelfOperateRequest
		OperateType string `json:"operate_type"`
	}{
		ShelfOperateRequest: request,
		OperateType:         operateType,
	}

	var result api.APIResponse
	if err := p.BaseAPIClient.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return err
	}

	// 统一错误处理
	return p.errorHandler.ProcessAPIResponse(&result, "0", url, errorMsg)
}

// OffShelf 产品下架
func (p *ProductAPI) OffShelf(request *product.ShelfOperateRequest) error {
	return p.operateShelf(request, "off_shelf", "产品下架失败")
}

// OnShelf 产品上架
func (p *ProductAPI) OnShelf(request *product.ShelfOperateRequest) error {
	return p.operateShelf(request, "on_shelf", "产品上架失败")
}
