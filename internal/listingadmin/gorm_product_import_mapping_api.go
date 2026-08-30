package listingadmin

import (
	"context"
	"strconv"
	"strings"

	"task-processor/internal/pkg/types"
)

// NewGormProductImportMappingAPI adapts resource-owned mapping operations for
// callers using the legacy API DTO contract.
func NewGormProductImportMappingAPI(repository *GormProductImportMappingRepository) ProductImportMappingAPI {
	if repository == nil {
		return nil
	}
	return gormProductImportMappingAPI{repository: repository}
}

type gormProductImportMappingAPI struct {
	repository *GormProductImportMappingRepository
}

func (a gormProductImportMappingAPI) CreateProductImportMapping(req *ProductImportMappingCreateReqDTO) (int64, error) {
	mapping := ProductImportMappingFromCreateReq(req)
	if a.repository == nil || mapping == nil {
		return 0, nil
	}
	created, err := a.repository.CreateProductImportMappingForStore(context.Background(), mapping)
	if err != nil || created == nil {
		return 0, err
	}
	return created.ID, nil
}

func (a gormProductImportMappingAPI) GetProductImportMappingByPlatformProductId(req *ProductImportMappingGetReqDTO) (*ProductImportMappingRespDTO, error) {
	if a.repository == nil || req == nil {
		return nil, nil
	}
	return a.findLatest(ProductImportMappingQuery{PlatformProductID: req.PlatformProductId})
}

func (a gormProductImportMappingAPI) CheckProductExists(req *ProductImportMappingCheckReqDTO) (bool, error) {
	if a.repository == nil || req == nil {
		return false, nil
	}
	return a.repository.ExistsPublishedProduct(context.Background(), req.StoreId, req.Platform, req.Region, req.ProductId)
}

func (a gormProductImportMappingAPI) GetProductImportMappingBySku(req *ProductImportMappingGetBySkuReqDTO) (*ProductImportMappingRespDTO, error) {
	if a.repository == nil || req == nil {
		return nil, nil
	}
	return a.findLatest(ProductImportMappingQuery{SKU: req.Sku, StoreID: &req.StoreId})
}

func (a gormProductImportMappingAPI) GetProductImportMappingByTaskAndSku(importTaskID int64, sku string) (*ProductImportMappingRespDTO, error) {
	if a.repository == nil {
		return nil, nil
	}
	return a.findLatest(ProductImportMappingQuery{ImportTaskID: &importTaskID, SKU: sku})
}

func (a gormProductImportMappingAPI) GetProductImportMappingByPlatformProductIdAndStore(req *ProductImportMappingGetByPlatformProductIdAndStoreReqDTO) (*ProductImportMappingRespDTO, error) {
	if a.repository == nil || req == nil {
		return nil, nil
	}
	return a.findLatest(ProductImportMappingQuery{PlatformProductID: req.PlatformProductId, StoreID: &req.StoreId})
}

func (a gormProductImportMappingAPI) UpdateProductImportMapping(req *ProductImportMappingCreateReqDTO) error {
	mapping := ProductImportMappingFromCreateReq(req)
	if a.repository == nil || mapping == nil || mapping.ID == 0 {
		return nil
	}
	_, err := a.repository.UpdateProductImportMappingForStore(context.Background(), mapping)
	return err
}

// ProductImportMappingFromCreateReq converts the legacy management request
// into the resource repository model while preserving its optional defaults.
func ProductImportMappingFromCreateReq(req *ProductImportMappingCreateReqDTO) *ProductImportMapping {
	if req == nil {
		return nil
	}
	status := int16(0)
	if req.Status != nil {
		status = *req.Status
	}
	mapping := &ProductImportMapping{
		TenantID:                req.TenantID,
		OwnerUserID:             req.OwnerUserID,
		ImportTaskID:            req.ImportTaskId,
		StoreID:                 req.StoreId,
		Platform:                req.Platform,
		Region:                  req.Region,
		ProductID:               req.ProductId,
		CostPrice:               req.CostPrice,
		FilterRuleID:            req.FilterRuleId,
		ProfitRuleID:            req.ProfitRuleId,
		Status:                  status,
		SalePriceMultiplier:     parseOptionalMultiplier(req.SalePriceMultiplier),
		DiscountPriceMultiplier: parseOptionalMultiplier(req.DiscountPriceMultiplier),
	}
	if req.ID != nil {
		mapping.ID = *req.ID
	}
	if req.Sku != nil {
		mapping.SKU = *req.Sku
	}
	if req.PlatformProductId != nil {
		mapping.PlatformProductID = *req.PlatformProductId
	}
	if req.ParentProductId != nil {
		mapping.ParentProductID = *req.ParentProductId
	}
	if req.PlatformParentProductId != nil {
		mapping.PlatformParentProductID = *req.PlatformParentProductId
	}
	if req.FilterRuleRange != nil {
		mapping.FilterRuleRange = *req.FilterRuleRange
	}
	if req.Remark != nil {
		mapping.Remark = *req.Remark
	}
	return mapping
}

func parseOptionalMultiplier(raw *string) float64 {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return 1
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(*raw), 64)
	if err != nil {
		return 1
	}
	return value
}

func (a gormProductImportMappingAPI) findLatest(query ProductImportMappingQuery) (*ProductImportMappingRespDTO, error) {
	mapping, err := a.repository.FindLatest(context.Background(), query)
	if err != nil || mapping == nil {
		return nil, err
	}
	return ProductImportMappingToRespDTO(mapping), nil
}

// ProductImportMappingToRespDTO exposes the legacy API projection for a
// repository mapping.
func ProductImportMappingToRespDTO(mapping *ProductImportMapping) *ProductImportMappingRespDTO {
	if mapping == nil {
		return nil
	}
	dto := &ProductImportMappingRespDTO{
		ID:                      mapping.ID,
		OwnerUserID:             mapping.OwnerUserID,
		ImportTaskId:            mapping.ImportTaskID,
		StoreId:                 mapping.StoreID,
		Platform:                mapping.Platform,
		Region:                  mapping.Region,
		ProductId:               mapping.ProductID,
		CostPrice:               mapping.CostPrice,
		FilterRuleId:            mapping.FilterRuleID,
		ProfitRuleId:            mapping.ProfitRuleID,
		SalePriceMultiplier:     ptrFloat64(mapping.SalePriceMultiplier),
		DiscountPriceMultiplier: ptrFloat64(mapping.DiscountPriceMultiplier),
		Status:                  mapping.Status,
		CreateTime:              types.ToFlexibleTime(mapping.CreateTime),
		TenantId:                mapping.TenantID,
	}
	if mapping.ParentProductID != "" {
		dto.ParentProductId = ptrString(mapping.ParentProductID)
	}
	if mapping.PlatformProductID != "" {
		dto.PlatformProductId = ptrString(mapping.PlatformProductID)
	}
	if mapping.PlatformParentProductID != "" {
		dto.PlatformParentProductId = ptrString(mapping.PlatformParentProductID)
	}
	if mapping.SKU != "" {
		dto.Sku = ptrString(mapping.SKU)
	}
	if mapping.FilterRuleRange != "" {
		dto.FilterRuleRange = ptrString(mapping.FilterRuleRange)
	}
	if mapping.Remark != "" {
		dto.Remark = ptrString(mapping.Remark)
	}
	return dto
}
