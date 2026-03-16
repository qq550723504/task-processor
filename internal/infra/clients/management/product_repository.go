package management

import (
	"fmt"
	"net/http"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pkg/strutil"
	"task-processor/internal/pkg/types"
)

// ProductDataAPIClient 产品数据API客户端实现
type ProductDataAPIClient struct {
	*ManagementAPIClient
	StoreID int64
}

// BatchCreateOrUpdate 批量创建或更新产品数据
func (c *ProductDataAPIClient) BatchCreateOrUpdate(req *api.ProductDataBatchSaveReqDTO) (int, error) {
	if req == nil || len(req.Products) == 0 {
		return 0, nil
	}

	url := fmt.Sprintf("%s/rpc-api/listing/product-data/batch-save", c.baseURL)
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
func (c *ProductDataAPIClient) buildBatchSaveRequestFromDTO(req *api.ProductDataBatchSaveReqDTO) map[string]any {
	productItems := make([]map[string]any, 0, len(req.Products))

	for _, product := range req.Products {
		item := map[string]any{
			"platformProductId":  product.PlatformProductID,
			"productName":        product.ProductName,
			"productSku":         product.ProductSku,
			"productPrice":       strutil.ParseFloat(product.ProductPrice.String()),
			"productStock":       product.ProductStock,
			"productCategory":    product.ProductCategory,
			"productImage":       product.ProductImage,
			"productDescription": product.ProductDescription,
		}

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
			item["specialPrice"] = strutil.ParseFloat(product.SpecialPrice.String())
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

	return map[string]any{
		"platform": req.Platform,
		"tenantId": req.TenantID,
		"storeId":  req.StoreID,
		"region":   req.Region,
		"products": productItems,
	}
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
func (c *ProductDataAPIClient) ListByStore(platform string, tenantID, storeID int64, shelfStatus *int) ([]*api.ProductDataDTO, error) {
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

	if err := c.ProcessAPIResponse(&APIResponse{Code: result.Code, Message: result.Msg}, 0); err != nil {
		return nil, fmt.Errorf("处理API响应失败: %w", err)
	}

	products := make([]*api.ProductDataDTO, 0, len(result.Data))
	for _, item := range result.Data {
		products = append(products, &api.ProductDataDTO{
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
		})
	}

	return products, nil
}

// BatchUpdateAttributes 批量更新产品属性
func (c *ProductDataAPIClient) BatchUpdateAttributes(req *api.ProductDataBatchUpdateAttributesReqDTO) (int, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/product-data/batch-update-attributes", c.baseURL)

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

func convertToProductAttributesItems(items []api.ProductAttributesItemDTO) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		m := map[string]any{
			"platformProductId": item.PlatformProductID,
			"attributes":        item.Attributes,
		}
		if item.UpdateTime != nil {
			m["updateTime"] = *item.UpdateTime
		}
		result = append(result, m)
	}
	return result
}

// PageProductDataByStore 分页查询店铺产品数据
func (c *ProductDataAPIClient) PageProductDataByStore(req *api.ProductDataListByStorePageReqDTO) (*api.PageResult[*api.ProductDataRespDTO], error) {
	url := fmt.Sprintf("%s/rpc-api/listing/product-data/page-by-store", c.baseURL)

	reqBody := map[string]any{
		"platform": req.Platform,
		"tenantId": req.TenantID,
		"storeId":  req.StoreID,
		"pageNo":   req.PageNo,
		"pageSize": req.PageSize,
	}
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

	respList := make([]*api.ProductDataRespDTO, 0, len(result.Data.List))
	for _, item := range result.Data.List {
		respList = append(respList, &api.ProductDataRespDTO{
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
		})
	}

	return &api.PageResult[*api.ProductDataRespDTO]{
		List:     respList,
		Total:    result.Data.Total,
		PageNo:   result.Data.PageNo,
		PageSize: result.Data.PageSize,
	}, nil
}
