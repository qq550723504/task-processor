// Package impl 产品数据API实现
package impl

import (
	"fmt"
	"net/http"
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/pkg/strutil"
	"task-processor/internal/pkg/types"
)

// ProductDataAPIClientImpl 产品数据API客户端实现
type ProductDataAPIClientImpl struct {
	*ManagementAPIClientImpl
	StoreID int64
}

// BatchCreateOrUpdate 批量创建或更新产品数据
func (c *ProductDataAPIClientImpl) BatchCreateOrUpdate(req *api.ProductDataBatchSaveReqDTO) (int, error) {
	if req == nil || len(req.Products) == 0 {
		return 0, nil
	}

	// 使用新的 RPC API 路径
	url := fmt.Sprintf("%s/rpc-api/listing/product-data/batch-save", c.baseURL)

	// 构建请求体
	reqBody := c.buildBatchSaveRequestFromDTO(req)

	var result api.CommonResult[int]
	if err := c.apiRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return 0, fmt.Errorf("批量保存产品失败: %w", err)
	}

	if result.Code != 0 {
		return 0, fmt.Errorf("批量保存产品失败: %s", result.Msg)
	}

	return result.Data, nil
}

// buildBatchSaveRequestFromDTO 从DTO构建批量保存请求体
func (c *ProductDataAPIClientImpl) buildBatchSaveRequestFromDTO(req *api.ProductDataBatchSaveReqDTO) map[string]any {
	productItems := make([]map[string]any, 0, len(req.Products))

	for _, product := range req.Products {
		item := map[string]any{
			"platformProductId":  product.PlatformProductID,
			"productName":        product.ProductName,
			"productSku":         product.ProductSku,
			"productPrice":       parseFlexiblePrice(product.ProductPrice),
			"productStock":       product.ProductStock,
			"productCategory":    product.ProductCategory,
			"productImage":       product.ProductImage,
			"productDescription": product.ProductDescription,
		}

		// 可选字段
		if product.ShelfStatus != nil {
			item["shelfStatus"] = *product.ShelfStatus
		}
		if product.Brand != "" {
			item["brand"] = product.Brand
		}
		if product.CategoryID != nil {
			item["categoryId"] = *product.CategoryID
		}
		if product.SpecialPrice.String() != "" {
			item["specialPrice"] = parseFlexiblePrice(product.SpecialPrice)
		}
		if product.PriceCurrency != "" {
			item["priceCurrency"] = product.PriceCurrency
		}
		if product.ImageUrls != "" {
			item["imageUrls"] = product.ImageUrls
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
		if product.CreateTime != nil && !product.CreateTime.IsZero() {
			item["createTime"] = product.CreateTime.Format("2006-01-02T15:04:05")
		}
		if product.UpdateTime != nil && !product.UpdateTime.IsZero() {
			item["updateTime"] = product.UpdateTime.Format("2006-01-02T15:04:05")
		}

		productItems = append(productItems, item)
	}

	reqBody := map[string]any{
		"platform": req.Platform,
		"tenantId": req.TenantID,
		"storeId":  req.StoreID,
		"region":   req.Region,
		"products": productItems,
	}

	return reqBody
}

// parseFlexiblePrice 解析FlexibleString价格为浮点数
// 已废弃: 请使用 strutil.ParseFloat(price.String())
func parseFlexiblePrice(price types.FlexibleString) float64 {
	return strutil.ParseFloat(price.String())
}

// parseStock 解析库存字符串为整数
// 已废弃: 请使用 strutil.ParseInt
func parseStock(stock string) int {
	return strutil.ParseInt(stock)
}

// parsePrice 解析价格字符串为浮点数
// 已废弃: 请使用 strutil.ParseFloat
func parsePrice(price string) float64 {
	return strutil.ParseFloat(price)
}

// ProductListItem 产品列表项（API 响应结构）
type ProductListItem struct {
	ID                int64                `json:"id"`
	Platform          string               `json:"platform"`
	StoreID           int64                `json:"storeId"`
	PlatformProductID string               `json:"platformProductId"`
	ProductID         string               `json:"productId"`
	ParentProductID   string               `json:"parentProductId"`
	Title             string               `json:"title"`
	Description       string               `json:"description"`
	OriginalPrice     types.FlexibleString `json:"originalPrice"`
	SpecialPrice      types.FlexibleString `json:"specialPrice"`
	PriceCurrency     string               `json:"priceCurrency"`
	Stock             types.FlexibleString `json:"stock"`
	ShelfStatus       int                  `json:"shelfStatus"`
	Brand             string               `json:"brand"`
	Category          string               `json:"category"`
	CategoryID        int64                `json:"categoryId"`
	Region            string               `json:"region"`
	MainImageURL      string               `json:"mainImageUrl"`
	ImageURLs         string               `json:"imageUrls"`
	Attributes        string               `json:"attributes"`
	SourceURL         string               `json:"sourceUrl"`
	PlatformStatus    string               `json:"platformStatus"`
	PlatformData      string               `json:"platformData"`
	PublishTime       *types.FlexibleTime  `json:"publishTime"`
	ShelfTime         *types.FlexibleTime  `json:"shelfTime"`
	LastSyncTime      *types.FlexibleTime  `json:"lastSyncTime"`
	CreateTime        *types.FlexibleTime  `json:"createTime"`
	UpdateTime        *types.FlexibleTime  `json:"updateTime"`
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

	reqBody := map[string]any{
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
			PublishTime:       item.PublishTime,
			ShelfTime:         item.ShelfTime,
			LastSyncTime:      item.LastSyncTime,
			CreateTime:        item.CreateTime,
			UpdateTime:        item.UpdateTime,
			TenantID:          tenantID,
		}
		products = append(products, product)
	}

	return products, nil
}

// BatchUpdateAttributes 批量更新产品属性
func (c *ProductDataAPIClientImpl) BatchUpdateAttributes(req *api.ProductDataBatchUpdateAttributesReqDTO) (int, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/product-data/batch-update-attributes", c.baseURL)

	// 构建请求体
	reqBody := map[string]any{
		"platform": req.Platform,
		"tenantId": req.TenantID,
		"storeId":  req.StoreID,
		"region":   req.Region,
		"products": convertToProductAttributesItems(req.Products),
	}

	var result api.CommonResult[int]
	if err := c.apiRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return 0, fmt.Errorf("批量更新产品属性失败: %w", err)
	}

	if result.Code != 0 {
		return 0, fmt.Errorf("批量更新产品属性失败: %s", result.Msg)
	}

	return result.Data, nil
}

// convertToProductAttributesItems 转换产品属性项
func convertToProductAttributesItems(items []api.ProductAttributesItemDTO) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		itemMap := map[string]any{
			"platformProductId": item.PlatformProductID,
			"attributes":        item.Attributes,
		}
		if item.UpdateTime != nil {
			itemMap["updateTime"] = *item.UpdateTime
		}
		result = append(result, itemMap)
	}
	return result
}

// PageProductDataByStore 分页查询店铺产品数据
func (c *ProductDataAPIClientImpl) PageProductDataByStore(req *api.ProductDataListByStorePageReqDTO) (*api.PageResult[*api.ProductDataRespDTO], error) {
	url := fmt.Sprintf("%s/rpc-api/listing/product-data/page-by-store", c.baseURL)

	// 构建请求体
	reqBody := map[string]any{
		"platform": req.Platform,
		"tenantId": req.TenantID,
		"storeId":  req.StoreID,
		"pageNo":   req.PageNo,
		"pageSize": req.PageSize,
	}

	// 添加可选参数
	if req.Region != "" {
		reqBody["region"] = req.Region
	}
	if req.ShelfStatus != nil {
		reqBody["shelfStatus"] = *req.ShelfStatus
	}
	if req.Title != "" {
		reqBody["title"] = req.Title
	}
	if req.Brand != "" {
		reqBody["brand"] = req.Brand
	}
	if req.Category != "" {
		reqBody["category"] = req.Category
	}
	if req.PlatformProductID != "" {
		reqBody["platformProductId"] = req.PlatformProductID
	}

	var result api.CommonResult[api.PageResult[ProductListItem]]
	if err := c.apiRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, fmt.Errorf("分页查询店铺产品数据失败: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("分页查询店铺产品数据失败: %s", result.Msg)
	}

	// 转换为响应DTO
	respList := make([]*api.ProductDataRespDTO, 0, len(result.Data.List))
	for _, item := range result.Data.List {
		respDTO := &api.ProductDataRespDTO{
			ProductDataDTO: &api.ProductDataDTO{
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
				PublishTime:       item.PublishTime,
				ShelfTime:         item.ShelfTime,
				LastSyncTime:      item.LastSyncTime,
				CreateTime:        item.CreateTime,
				UpdateTime:        item.UpdateTime,
				TenantID:          req.TenantID,
			},
		}
		respList = append(respList, respDTO)
	}

	pageResult := &api.PageResult[*api.ProductDataRespDTO]{
		List:     respList,
		Total:    result.Data.Total,
		PageNo:   result.Data.PageNo,
		PageSize: result.Data.PageSize,
	}

	return pageResult, nil
}
