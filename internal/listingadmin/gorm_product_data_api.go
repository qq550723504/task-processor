package listingadmin

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"task-processor/internal/pkg/types"
)

// NewGormProductDataAPI adapts resource-owned product operations for callers
// using the legacy API DTO contract.
func NewGormProductDataAPI(repository ProductDataRepository, storeID int64) ProductDataAPI {
	if repository == nil {
		return nil
	}
	return gormProductDataAPI{repository: repository, storeID: storeID}
}

type gormProductDataAPI struct {
	repository ProductDataRepository
	storeID    int64
}

func (a gormProductDataAPI) BatchCreateOrUpdate(req *ProductDataBatchSaveReqDTO) (int, error) {
	if a.repository == nil || req == nil {
		return 0, nil
	}
	return a.repository.UpsertProductDataBatch(context.Background(), ProductDataFromBatchSaveReq(req))
}

func (a gormProductDataAPI) ListByStore(platform string, tenantID, storeID int64, shelfStatus *int) ([]*ProductDataDTO, error) {
	if a.repository == nil {
		return nil, nil
	}
	if storeID == 0 {
		storeID = a.storeID
	}
	page, err := a.repository.ListProductData(context.Background(), ProductDataQuery{
		TenantID:    tenantID,
		StoreID:     int64Ptr(storeID),
		Platform:    platform,
		Page:        1,
		PageSize:    2000,
		ShelfStatus: shelfStatus,
	})
	if err != nil || page == nil {
		return nil, err
	}
	items := make([]*ProductDataDTO, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, ProductDataToDTO(&page.Items[i]))
	}
	return items, nil
}

func (a gormProductDataAPI) BatchUpdateAttributes(req *ProductDataBatchUpdateAttributesReqDTO) (int, error) {
	if a.repository == nil || req == nil {
		return 0, nil
	}
	return a.repository.BatchUpdateAttributesByPlatformProductID(context.Background(), ProductDataFromAttributesBatchReq(req))
}

// ProductDataFromBatchSaveReq converts the legacy batch-save request into the
// resource repository model while preserving optional DTO field semantics.
func ProductDataFromBatchSaveReq(req *ProductDataBatchSaveReqDTO) []ProductData {
	if req == nil {
		return nil
	}
	items := make([]ProductData, 0, len(req.Products))
	for _, product := range req.Products {
		item := ProductData{
			TenantID:          req.TenantID,
			StoreID:           int64Ptr(req.StoreID),
			Platform:          req.Platform,
			Region:            req.Region,
			ParentProductID:   product.ParentProductID,
			ProductID:         product.ProductSku,
			Title:             product.ProductName,
			Description:       product.ProductDescription,
			OriginalPrice:     flexibleStringToFloat64(product.ProductPrice),
			SpecialPrice:      flexibleStringToFloat64(product.SpecialPrice),
			PriceCurrency:     product.PriceCurrency,
			Stock:             product.ProductStock.String(),
			Brand:             product.Brand,
			Category:          product.ProductCategory,
			MainImageURL:      product.ProductImage,
			ImageURLs:         rawJSONBytes(product.ImageUrls),
			Attributes:        rawJSONBytes(product.Attributes),
			PlatformStatus:    product.PlatformStatus,
			PlatformData:      rawJSONBytes(product.PlatformData),
			PlatformProductID: product.PlatformProductID,
		}
		if product.ShelfStatus != nil {
			item.ShelfStatus = product.ShelfStatus
		}
		if product.CategoryID != nil {
			item.CategoryID = product.CategoryID
		}
		if product.PublishTime != nil {
			item.PublishTime = &product.PublishTime.Time
		}
		if product.ShelfTime != nil {
			item.ShelfTime = &product.ShelfTime.Time
		}
		if product.CreateTime != nil {
			item.CreateTime = &product.CreateTime.Time
		}
		if product.UpdateTime != nil {
			item.UpdateTime = &product.UpdateTime.Time
		}
		items = append(items, item)
	}
	return items
}

// ProductDataFromAttributesBatchReq converts the legacy attributes request
// into resource-owned update records.
func ProductDataFromAttributesBatchReq(req *ProductDataBatchUpdateAttributesReqDTO) []ProductData {
	if req == nil {
		return nil
	}
	items := make([]ProductData, 0, len(req.Products))
	for _, product := range req.Products {
		items = append(items, ProductData{
			TenantID:          req.TenantID,
			StoreID:           int64Ptr(req.StoreID),
			Platform:          req.Platform,
			PlatformProductID: product.PlatformProductID,
			Attributes:        rawJSONBytes(product.Attributes),
		})
	}
	return items
}

func int64Ptr(value int64) *int64 {
	v := value
	return &v
}

func flexibleStringToFloat64(value types.FlexibleString) float64 {
	raw := strings.TrimSpace(value.String())
	if raw == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func rawJSONBytes(value string) []byte {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if json.Valid([]byte(trimmed)) {
		return []byte(trimmed)
	}
	encoded, _ := json.Marshal(trimmed)
	return encoded
}

func (a gormProductDataAPI) PageProductDataByStore(req *ProductDataListByStorePageReqDTO) (*PageResult[*ProductDataRespDTO], error) {
	if a.repository == nil || req == nil {
		return nil, nil
	}
	query := ProductDataQuery{
		TenantID:          req.TenantID,
		StoreID:           int64Ptr(req.StoreID),
		Platform:          req.Platform,
		Region:            req.Region,
		Title:             req.Title,
		Brand:             req.Brand,
		PlatformProductID: req.PlatformProductID,
		Page:              req.PageNo,
		PageSize:          req.PageSize,
		ShelfStatus:       req.ShelfStatus,
	}
	if req.Category != "" {
		query.Category = req.Category
	}
	page, err := a.repository.ListProductData(context.Background(), query)
	if err != nil || page == nil {
		return nil, err
	}
	items := make([]*ProductDataRespDTO, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, &ProductDataRespDTO{ProductDataDTO: ProductDataToDTO(&page.Items[i])})
	}
	return &PageResult[*ProductDataRespDTO]{List: items, Total: page.Total, PageNo: page.Page, PageSize: page.PageSize}, nil
}

// ProductDataToDTO exposes the legacy API projection for a repository product.
func ProductDataToDTO(product *ProductData) *ProductDataDTO {
	if product == nil {
		return nil
	}
	return &ProductDataDTO{
		ID:                product.ID,
		Source:            product.Source,
		ImportTaskID:      int64Value(product.ImportTaskID),
		StoreID:           int64Value(product.StoreID),
		Platform:          product.Platform,
		CategoryID:        int64Value(product.CategoryID),
		Region:            product.Region,
		ParentProductID:   product.ParentProductID,
		ProductID:         product.ProductID,
		Title:             product.Title,
		Description:       product.Description,
		OriginalPrice:     types.FlexibleString(strconv.FormatFloat(product.OriginalPrice, 'f', -1, 64)),
		SpecialPrice:      types.FlexibleString(strconv.FormatFloat(product.SpecialPrice, 'f', -1, 64)),
		PriceCurrency:     product.PriceCurrency,
		Stock:             types.FlexibleString(product.Stock),
		Brand:             product.Brand,
		Category:          product.Category,
		MainImageURL:      product.MainImageURL,
		ImageURLs:         string(product.ImageURLs),
		Attributes:        string(product.Attributes),
		SourceURL:         product.SourceURL,
		Status:            product.Status,
		RawJSONDataID:     int64Value(product.RawJSONDataID),
		PlatformProductID: product.PlatformProductID,
		PlatformStatus:    product.PlatformStatus,
		ShelfStatus:       intValue(product.ShelfStatus),
		PublishTime:       types.ToFlexibleTime(product.PublishTime),
		ShelfTime:         types.ToFlexibleTime(product.ShelfTime),
		LastSyncTime:      types.ToFlexibleTime(product.LastSyncTime),
		PlatformData:      string(product.PlatformData),
		TenantID:          product.TenantID,
		CreateTime:        types.ToFlexibleTime(product.CreateTime),
		UpdateTime:        types.ToFlexibleTime(product.UpdateTime),
	}
}
