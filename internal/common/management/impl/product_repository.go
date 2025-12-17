// Package impl 产品数据API实现
package impl

import (
	"fmt"
	"net/http"
	"task-processor/internal/common/management/api"
)

// ProductDataAPIClientImpl 产品数据API客户端实现
type ProductDataAPIClientImpl struct {
	*ManagementAPIClientImpl
	StoreID int64
}

// CreateOrUpdate 创建或更新单个产品数据
func (c *ProductDataAPIClientImpl) CreateOrUpdate(product *api.ProductDataDTO) error {
	// 使用批量接口保存单条记录
	return c.BatchCreateOrUpdate([]*api.ProductDataDTO{product})
}

// BatchCreateOrUpdate 批量创建或更新产品数据
func (c *ProductDataAPIClientImpl) BatchCreateOrUpdate(products []*api.ProductDataDTO) error {
	if len(products) == 0 {
		return nil
	}

	// 使用新的 RPC API 路径
	url := fmt.Sprintf("%s/rpc-api/listing/product-data/batch-save", c.baseURL)

	// 按平台分组
	platformGroups := groupProductsByPlatform(products)

	// 按平台分别调用
	for platform, groupProducts := range platformGroups {
		reqBody := c.buildBatchSaveRequest(platform, groupProducts)

		var result APIResponse
		if err := c.apiRequest(http.MethodPost, url, reqBody, &result); err != nil {
			return fmt.Errorf("批量保存平台 %s 的产品失败: %w", platform, err)
		}

		if err := c.ProcessAPIResponse(&result, 0); err != nil {
			return fmt.Errorf("处理平台 %s 的API响应失败: %w", platform, err)
		}
	}

	return nil
}

// groupProductsByPlatform 按平台分组产品
func groupProductsByPlatform(products []*api.ProductDataDTO) map[string][]*api.ProductDataDTO {
	platformGroups := make(map[string][]*api.ProductDataDTO)
	for _, product := range products {
		platform := product.Platform
		if platform == "" {
			platform = "UNKNOWN"
		}
		platformGroups[platform] = append(platformGroups[platform], product)
	}
	return platformGroups
}

// buildBatchSaveRequest 构建批量保存请求体
func (c *ProductDataAPIClientImpl) buildBatchSaveRequest(platform string, products []*api.ProductDataDTO) map[string]interface{} {
	productItems := make([]map[string]interface{}, 0, len(products))

	for _, product := range products {
		item := map[string]interface{}{
			"platformProductId":  product.PlatformProductID,
			"productName":        product.Title,
			"productSku":         product.ProductID,
			"productPrice":       parsePrice(product.OriginalPrice.String()),
			"productStock":       parseStock(product.Stock.String()),
			"productCategory":    product.Category,
			"productImage":       product.MainImageURL,
			"productDescription": product.Description,
			"shelfStatus":        product.ShelfStatus,
		}

		if product.SpecialPrice.String() != "" {
			item["specialPrice"] = parsePrice(product.SpecialPrice.String())
		}
		if product.PriceCurrency != "" {
			item["priceCurrency"] = product.PriceCurrency
		}
		if product.ImageURLs != "" {
			item["imageUrls"] = product.ImageURLs
		}
		if product.Attributes != "" {
			item["attributes"] = product.Attributes
		}
		if product.PlatformStatus != "" {
			item["platformStatus"] = product.PlatformStatus
		}
		if product.PlatformData != "" {
			item["platformData"] = product.PlatformData
		}
		if product.ParentProductID != "" {
			item["parentProductId"] = product.ParentProductID
		}

		// 时间字段使用毫秒时间戳
		if product.PublishTime != nil && !product.PublishTime.IsZero() {
			item["publishTime"] = product.PublishTime.UnixMilli()
		}
		if product.ShelfTime != nil && !product.ShelfTime.IsZero() {
			item["shelfTime"] = product.ShelfTime.UnixMilli()
		}

		productItems = append(productItems, item)
	}

	// 从第一个产品中获取租户ID和区域（所有产品应该属于同一租户和区域）
	var tenantID int64
	var region string
	if len(products) > 0 {
		tenantID = products[0].TenantID
		region = products[0].Region
	}

	reqBody := map[string]interface{}{
		"platform": platform,
		"tenantId": tenantID,
		"storeId":  c.StoreID,
		"region":   region,
		"products": productItems,
	}

	return reqBody
}

// parseStock 解析库存字符串为整数
func parseStock(stock string) int {
	if stock == "" {
		return 0
	}
	var result int
	fmt.Sscanf(stock, "%d", &result)
	return result
}

// parsePrice 解析价格字符串为浮点数
func parsePrice(price string) float64 {
	if price == "" {
		return 0.0
	}
	var result float64
	fmt.Sscanf(price, "%f", &result)
	return result
}

// ProductListItem 产品列表项（API 响应结构）
type ProductListItem struct {
	ID                int64              `json:"id"`
	Platform          string             `json:"platform"`
	StoreID           int64              `json:"storeId"`
	PlatformProductID string             `json:"platformProductId"`
	ProductID         string             `json:"productId"`
	ParentProductID   string             `json:"parentProductId"`
	Title             string             `json:"title"`
	Description       string             `json:"description"`
	OriginalPrice     api.FlexibleString `json:"originalPrice"`
	SpecialPrice      api.FlexibleString `json:"specialPrice"`
	PriceCurrency     string             `json:"priceCurrency"`
	Stock             api.FlexibleString `json:"stock"`
	ShelfStatus       int                `json:"shelfStatus"`
	Brand             string             `json:"brand"`
	Category          string             `json:"category"`
	CategoryID        int64              `json:"categoryId"`
	Region            string             `json:"region"`
	MainImageURL      string             `json:"mainImageUrl"`
	ImageURLs         string             `json:"imageUrls"`
	Attributes        string             `json:"attributes"`
	SourceURL         string             `json:"sourceUrl"`
	PlatformStatus    string             `json:"platformStatus"`
	PlatformData      string             `json:"platformData"`
}

// ProductListResponse 产品列表响应
type ProductListResponse struct {
	Code int               `json:"code"`
	Msg  string            `json:"msg"`
	Data []ProductListItem `json:"data"`
}

// ListByStore 查询店铺的所有产品数据
func (c *ProductDataAPIClientImpl) ListByStore(platform string, tenantID, storeID int64, shelfStatus *int) ([]*api.ProductDataDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/product-data/list-by-store", c.baseURL)

	reqBody := map[string]interface{}{
		"platform": platform,
		"tenantId": tenantID,
		"storeId":  storeID,
	}

	if shelfStatus != nil {
		reqBody["shelfStatus"] = *shelfStatus
	}

	var result ProductListResponse

	if err := c.apiRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, fmt.Errorf("查询店铺产品列表失败: %w", err)
	}

	if err := c.ProcessAPIResponse(&APIResponse{
		Code:    result.Code,
		Message: result.Msg,
	}, 0); err != nil {
		return nil, fmt.Errorf("处理API响应失败: %w", err)
	}

	// 转换为 ProductDataDTO
	products := make([]*api.ProductDataDTO, 0, len(result.Data))
	for _, item := range result.Data {
		product := &api.ProductDataDTO{
			ID:                item.ID,
			Platform:          item.Platform,
			StoreID:           item.StoreID,
			PlatformProductID: item.PlatformProductID,
			ProductID:         item.ProductID,
			ParentProductID:   item.ParentProductID,
			Title:             item.Title,
			Description:       item.Description,
			OriginalPrice:     item.OriginalPrice,
			SpecialPrice:      item.SpecialPrice,
			PriceCurrency:     item.PriceCurrency,
			Stock:             item.Stock,
			ShelfStatus:       item.ShelfStatus,
			Brand:             item.Brand,
			Category:          item.Category,
			CategoryID:        item.CategoryID,
			Region:            item.Region,
			MainImageURL:      item.MainImageURL,
			ImageURLs:         item.ImageURLs,
			Attributes:        item.Attributes,
			SourceURL:         item.SourceURL,
			PlatformStatus:    item.PlatformStatus,
			PlatformData:      item.PlatformData,
			TenantID:          tenantID,
		}
		products = append(products, product)
	}

	return products, nil
}
