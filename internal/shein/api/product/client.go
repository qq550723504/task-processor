package product

import (
	"fmt"
	"net/http"
	"task-processor/internal/shein/api"
	"task-processor/internal/shein/client"
)

// Client 产品相关API实现，组合了产品、库存、价格管理
type Client struct {
	*client.BaseAPIClient
	productMgr   *productManager
	inventoryMgr *inventoryManager
	priceMgr     *priceManager
	errorHandler *client.APIErrorHandler
}

// NewClient 创建新的产品API客户端
func NewClient(baseClient *client.BaseAPIClient) *Client {
	errorHandler := client.NewAPIErrorHandler(baseClient)
	return &Client{
		BaseAPIClient: baseClient,
		productMgr:    newProductManager(baseClient, errorHandler),
		inventoryMgr:  newInventoryManager(baseClient, errorHandler),
		priceMgr:      newPriceManager(baseClient, errorHandler),
		errorHandler:  errorHandler,
	}
}

// GetProduct 获取产品信息
func (c *Client) GetProduct(productID string) (*Product, error) {
	return c.productMgr.getProduct(productID)
}

// UpdateProduct 更新产品信息
func (c *Client) UpdateProduct(product *Product) error {
	return c.productMgr.updateProduct(product)
}

// DeleteProduct 删除产品
func (c *Client) DeleteProduct(productID string) error {
	return c.productMgr.deleteProduct(productID)
}

// GetPartInfo 获取部件信息
func (c *Client) GetPartInfo(categoryID int) (*PartInfoResponse, error) {
	return c.productMgr.getPartInfo(categoryID)
}

// SaveDraftProduct 保存产品草稿
func (c *Client) SaveDraftProduct(prod *Product) (*SheinResponse, string, error) {
	return c.productMgr.saveDraftProduct(prod)
}

// PublishProduct 发布产品
func (c *Client) PublishProduct(prod *Product) (*SheinResponse, string, error) {
	return c.productMgr.publishProduct(prod)
}

// ConfirmPublish 确认发布产品
func (c *Client) ConfirmPublish(product *Product) (bool, string, error) {
	return c.productMgr.confirmPublish(product)
}

// Record 获取产品发布记录
func (c *Client) Record(request *ProductRecordRequest) (*RecordResponse, error) {
	return c.productMgr.record(request)
}

// ListProducts 获取产品列表
func (c *Client) ListProducts(pageNum, pageSize int, request *ProductListRequest) (*ProductListResponse, error) {
	return c.productMgr.listProducts(pageNum, pageSize, request)
}

// QueryStock 查询产品库存
func (c *Client) QueryStock(request *StockQueryRequest) (*StockQueryResponse, error) {
	return c.inventoryMgr.queryStock(request)
}

// QueryInventory 查询产品库存详情
func (c *Client) QueryInventory(spuName string) (*InventoryQueryResponse, error) {
	return c.inventoryMgr.queryInventory(spuName)
}

// UpdateInventory 更新产品库存
func (c *Client) UpdateInventory(request *InventoryUpdateRequest) error {
	return c.inventoryMgr.updateInventory(request)
}

// QueryPrice 查询产品价格
func (c *Client) QueryPrice(spuName string) (*PriceQueryResponse, error) {
	return c.priceMgr.queryPrice(spuName)
}

// QueryCostPrice 查询成本价格
func (c *Client) QueryCostPrice(spuName string, skcNameList []string) (*CostPriceQueryResponse, error) {
	return c.priceMgr.queryCostPrice(spuName, skcNameList)
}

// operateShelf 通用上下架操作
func (c *Client) operateShelf(request *ShelfOperateRequest, operateType, errorMsg string) error {
	url := fmt.Sprintf("%s%s", c.BaseAPIClient.GetBaseURL(), client.GetOperateShelfStatusEndpoint())

	reqBody := struct {
		*ShelfOperateRequest
		OperateType string `json:"operate_type"`
	}{ShelfOperateRequest: request, OperateType: operateType}

	var result api.APIResponse
	if err := c.BaseAPIClient.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return err
	}

	return c.errorHandler.ProcessAPIResponse(&result, "0", url, errorMsg)
}

// OffShelf 产品下架
func (c *Client) OffShelf(request *ShelfOperateRequest) error {
	return c.operateShelf(request, "off_shelf", "产品下架失败")
}

// OnShelf 产品上架
func (c *Client) OnShelf(request *ShelfOperateRequest) error {
	return c.operateShelf(request, "on_shelf", "产品上架失败")
}
