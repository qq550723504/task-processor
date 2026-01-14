package repo

import (
	"task-processor/internal/platforms/shein/api/product"
)

// ProductAPIInterface 产品API接口
type ProductAPIInterface interface {
	// ListProducts 获取产品列表
	ListProducts(pageNum, pageSize int, request *product.ProductListRequest) (*product.ProductListResponse, error)

	// GetProduct 获取产品信息
	GetProduct(productID string) (*product.Product, error)

	// UpdateProduct 更新产品信息
	UpdateProduct(product *product.Product) error

	// DeleteProduct 删除产品
	DeleteProduct(productID string) error

	// QueryStock 查询产品库存
	QueryStock(request *product.StockQueryRequest) (*product.StockQueryResponse, error)

	// QueryInventory 查询产品库存详情
	QueryInventory(spuName string) (*product.InventoryQueryResponse, error)

	// UpdateInventory 更新产品库存
	UpdateInventory(request *product.InventoryUpdateRequest) error

	// QueryPrice 查询产品价格
	QueryPrice(spuName string) (*product.PriceQueryResponse, error)

	// QueryCostPrice 查询成本价格
	QueryCostPrice(spuName string, skcNameList []string) (*product.CostPriceQueryResponse, error)

	// OffShelf 产品下架
	OffShelf(request *product.ShelfOperateRequest) error

	// OnShelf 产品上架
	OnShelf(request *product.ShelfOperateRequest) error
}
