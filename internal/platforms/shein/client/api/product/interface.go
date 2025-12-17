// Package product 产品相关API接口定义
package product

// ProductAPI 产品相关API接口
type ProductAPI interface {
	// 基础产品操作
	GetProduct(productID string) (*Product, error)
	UpdateProduct(product *Product) error
	DeleteProduct(productID string) error
	GetPartInfo(categoryID int) (*PartInfoResponse, error)

	// 产品发布
	SaveDraftProduct(product *Product) (*SheinResponse, string, error)
	PublishProduct(product *Product) (*SheinResponse, string, error)
	ConfirmPublish(product *Product) (bool, string, error)

	// 产品查询
	Record(request *ProductRecordRequest) (*RecordResponse, error)
	ListProducts(pageNum, pageSize int, request *ProductListRequest) (*ProductListResponse, error)

	// 库存管理
	QueryStock(request *StockQueryRequest) (*StockQueryResponse, error)
	QueryInventory(spuName string) (*InventoryQueryResponse, error)
	UpdateInventory(request *InventoryUpdateRequest) error

	// 价格管理
	QueryPrice(spuName string) (*PriceQueryResponse, error)
	QueryCostPrice(spuName string, skcNameList []string) (*CostPriceQueryResponse, error)

	// 上下架管理
	OffShelf(request *ShelfOperateRequest) error
	OnShelf(request *ShelfOperateRequest) error
}
