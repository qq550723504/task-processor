// Package scheduler 提供SHEIN平台SKU映射关系构建器
package scheduler

import (
	"fmt"
	"time"

	managementapi "task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// MappingBuilder SKU映射关系构建器
type MappingBuilder struct {
	mappingClient managementapi.ProductImportMappingAPI
	logger        *logrus.Entry
}

// NewMappingBuilder 创建映射关系构建器
func NewMappingBuilder(mappingClient managementapi.ProductImportMappingAPI) *MappingBuilder {
	return &MappingBuilder{
		mappingClient: mappingClient,
		logger:        logrus.WithField("component", "MappingBuilder"),
	}
}

// MappingCreateOptions 映射关系创建选项
type MappingCreateOptions struct {
	TenantID                int64                  `json:"tenantId"`                          // 租户ID
	StoreID                 int64                  `json:"storeId"`                           // 店铺ID
	SkuCode                 string                 `json:"skuCode"`                           // SKU编码（平台SKU）
	SupplierSku             string                 `json:"supplierSku,omitempty"`             // 供应商SKU
	ProductID               string                 `json:"productId"`                         // 产品ID（ASIN）
	ParentProductID         *string                `json:"parentProductId,omitempty"`         // 父产品ID（父ASIN）
	PlatformParentProductID *string                `json:"platformParentProductId,omitempty"` // 平台父产品ID（SPU名称）
	SpuCode                 string                 `json:"spuCode,omitempty"`                 // SPU编码
	SpuName                 string                 `json:"spuName,omitempty"`                 // SPU名称
	Region                  string                 `json:"region"`                            // 区域
	Reason                  string                 `json:"reason"`                            // 创建原因
	ImportTaskID            *int64                 `json:"importTaskId,omitempty"`            // 导入任务ID
	CostPrice               *float64               `json:"costPrice,omitempty"`               // 成本价
	Status                  *int16                 `json:"status,omitempty"`                  // 状态
	ProfitRuleID            *int64                 `json:"profitRuleId,omitempty"`            // 利润规则ID
	SalePriceMultiplier     *string                `json:"salePriceMultiplier,omitempty"`     // 售价倍数
	DiscountPriceMultiplier *string                `json:"discountPriceMultiplier,omitempty"` // 折扣价倍数
	FilterRuleID            *int64                 `json:"filterRuleId,omitempty"`            // 筛选规则ID
	FilterRuleRange         *string                `json:"filterRuleRange,omitempty"`         // 筛选规则范围
	AdditionalData          map[string]interface{} `json:"additionalData,omitempty"`          // 额外数据
}

// CreateMappingRelation 创建映射关系的统一函数
func (b *MappingBuilder) CreateMappingRelation(options *MappingCreateOptions) (*managementapi.ProductImportMappingRespDTO, error) {
	b.logger.WithFields(logrus.Fields{
		"sku_code": options.SkuCode,
		"store_id": options.StoreID,
		"reason":   options.Reason,
	}).Info("开始创建SKU映射关系")

	// 验证必要参数
	if err := b.validateOptions(options); err != nil {
		return nil, fmt.Errorf("参数验证失败: %w", err)
	}

	// 构建创建请求
	createReq := b.buildCreateRequest(options)

	// 创建映射关系
	mappingID, err := b.mappingClient.CreateProductImportMapping(createReq)
	if err != nil {
		b.logger.WithError(err).WithFields(logrus.Fields{
			"sku_code": options.SkuCode,
			"store_id": options.StoreID,
		}).Error("创建映射关系失败")
		return nil, fmt.Errorf("创建映射关系失败: %w", err)
	}

	b.logger.WithFields(logrus.Fields{
		"sku_code":   options.SkuCode,
		"store_id":   options.StoreID,
		"mapping_id": mappingID,
	}).Info("映射关系创建成功")

	// 查询创建的映射关系
	mappingInfo, err := b.queryCreatedMapping(options.SkuCode, options.StoreID)
	if err != nil {
		b.logger.WithError(err).WithFields(logrus.Fields{
			"sku_code":   options.SkuCode,
			"mapping_id": mappingID,
		}).Warn("查询新创建的映射关系失败")
		// 即使查询失败，也返回基本信息
		return &managementapi.ProductImportMappingRespDTO{
			ID:                mappingID,
			StoreId:           options.StoreID,
			PlatformProductId: &options.SkuCode,
		}, nil
	}

	return mappingInfo, nil
}

// CreateMappingFromContext 从修复上下文创建映射关系
func (b *MappingBuilder) CreateMappingFromContext(ctx *MappingRepairContext, reason string) (*managementapi.ProductImportMappingRespDTO, error) {
	options := &MappingCreateOptions{
		TenantID: ctx.Request.TenantID,
		StoreID:  ctx.Request.StoreID,
		SkuCode:  ctx.Request.SkuCode,
		SpuCode:  ctx.Request.SpuCode,
		SpuName:  ctx.Request.SpuName,
		Region:   b.determineRegion(ctx.StoreInfo),
		Reason:   reason,
	}

	// 如果有SKU详细信息，可以设置更多字段
	if ctx.SkuInfo != nil {
		b.enrichOptionsFromSkuInfo(options, ctx.SkuInfo)
	}

	// 如果有产品信息，设置产品相关字段
	if ctx.ProductInfo != nil {
		b.enrichOptionsFromProductInfo(options, ctx.ProductInfo)
	}

	return b.CreateMappingRelation(options)
}

// CreateBasicMapping 创建基础映射关系（最少参数）
func (b *MappingBuilder) CreateBasicMapping(tenantID, storeID int64, skuCode, region, reason string) (*managementapi.ProductImportMappingRespDTO, error) {
	options := &MappingCreateOptions{
		TenantID: tenantID,
		StoreID:  storeID,
		SkuCode:  skuCode,
		Region:   region,
		Reason:   reason,
	}

	return b.CreateMappingRelation(options)
}

// CreateMappingWithSPU 创建包含SPU信息的映射关系
func (b *MappingBuilder) CreateMappingWithSPU(tenantID, storeID int64, skuCode, spuCode, spuName, region, reason string) (*managementapi.ProductImportMappingRespDTO, error) {
	options := &MappingCreateOptions{
		TenantID: tenantID,
		StoreID:  storeID,
		SkuCode:  skuCode,
		SpuCode:  spuCode,
		SpuName:  spuName,
		Region:   region,
		Reason:   reason,
	}

	return b.CreateMappingRelation(options)
}

// CreateMappingWithPrice 创建包含价格信息的映射关系
func (b *MappingBuilder) CreateMappingWithPrice(tenantID, storeID int64, skuCode, region, reason string, costPrice float64) (*managementapi.ProductImportMappingRespDTO, error) {
	options := &MappingCreateOptions{
		TenantID:  tenantID,
		StoreID:   storeID,
		SkuCode:   skuCode,
		Region:    region,
		Reason:    reason,
		CostPrice: &costPrice,
	}

	return b.CreateMappingRelation(options)
}

// CreateMappingWithRules 创建包含规则信息的映射关系
func (b *MappingBuilder) CreateMappingWithRules(
	tenantID, storeID int64,
	skuCode, region, reason string,
	profitRuleID, filterRuleID *int64,
	salePriceMultiplier, discountPriceMultiplier, filterRuleRange *string,
) (*managementapi.ProductImportMappingRespDTO, error) {
	options := &MappingCreateOptions{
		TenantID:                tenantID,
		StoreID:                 storeID,
		SkuCode:                 skuCode,
		Region:                  region,
		Reason:                  reason,
		ProfitRuleID:            profitRuleID,
		FilterRuleID:            filterRuleID,
		SalePriceMultiplier:     salePriceMultiplier,
		DiscountPriceMultiplier: discountPriceMultiplier,
		FilterRuleRange:         filterRuleRange,
	}

	return b.CreateMappingRelation(options)
}

// CreateMappingFromTaskContext 从任务上下文创建映射关系（类似result_service.go的实现）
func (b *MappingBuilder) CreateMappingFromTaskContext(
	tenantID, storeID int64,
	skuCode, supplierSku, productID, region, reason string,
	parentProductID, platformParentProductID *string,
	costPrice *float64,
	profitRuleID, filterRuleID *int64,
	salePriceMultiplier, discountPriceMultiplier, filterRuleRange *string,
) (*managementapi.ProductImportMappingRespDTO, error) {
	options := &MappingCreateOptions{
		TenantID:                tenantID,
		StoreID:                 storeID,
		SkuCode:                 skuCode,
		SupplierSku:             supplierSku,
		ProductID:               productID,
		ParentProductID:         parentProductID,
		PlatformParentProductID: platformParentProductID,
		Region:                  region,
		Reason:                  reason,
		CostPrice:               costPrice,
		ProfitRuleID:            profitRuleID,
		FilterRuleID:            filterRuleID,
		SalePriceMultiplier:     salePriceMultiplier,
		DiscountPriceMultiplier: discountPriceMultiplier,
		FilterRuleRange:         filterRuleRange,
	}

	return b.CreateMappingRelation(options)
}

// validateOptions 验证创建选项
func (b *MappingBuilder) validateOptions(options *MappingCreateOptions) error {
	if options.TenantID <= 0 {
		return fmt.Errorf("租户ID不能为空或小于等于0")
	}

	if options.StoreID <= 0 {
		return fmt.Errorf("店铺ID不能为空或小于等于0")
	}

	if options.SkuCode == "" {
		return fmt.Errorf("SKU编码不能为空")
	}

	if options.Region == "" {
		return fmt.Errorf("区域不能为空")
	}

	return nil
}

// buildCreateRequest 构建创建请求
func (b *MappingBuilder) buildCreateRequest(options *MappingCreateOptions) *managementapi.ProductImportMappingCreateReqDTO {
	// 设置默认的导入任务ID
	importTaskID := time.Now().Unix()
	if options.ImportTaskID != nil {
		importTaskID = *options.ImportTaskID
	}

	// 设置默认状态 - 参考result_service.go，修复后的映射关系应该是成功状态
	status := int16(1) // 1表示成功状态
	if options.Status != nil {
		status = *options.Status
	}

	createReq := &managementapi.ProductImportMappingCreateReqDTO{
		TenantID:          options.TenantID,
		ImportTaskId:      importTaskID,
		StoreId:           options.StoreID,
		Platform:          "SHEIN",
		Region:            options.Region,
		ProductId:         options.ProductID, // ASIN或产品ID
		PlatformProductId: &options.SkuCode,  // 平台产品ID使用SKU编码
		Status:            &status,
		Remark:            &options.Reason,
	}

	// 设置供应商SKU - 参考result_service.go的映射逻辑
	if options.SupplierSku != "" {
		createReq.Sku = &options.SupplierSku
	}

	// 设置父产品ID信息
	if options.ParentProductID != nil {
		createReq.ParentProductId = options.ParentProductID
	}

	// 设置平台父产品ID（SPU名称）
	if options.PlatformParentProductID != nil {
		createReq.PlatformParentProductId = options.PlatformParentProductID
	} else if options.SpuName != "" {
		// 如果没有明确设置PlatformParentProductID，但有SpuName，则使用SpuName
		createReq.PlatformParentProductId = &options.SpuName
	}

	// 设置成本价
	if options.CostPrice != nil {
		createReq.CostPrice = options.CostPrice
	}

	// 设置利润规则ID和倍数 - 参考result_service.go的格式化逻辑
	if options.ProfitRuleID != nil {
		createReq.ProfitRuleId = options.ProfitRuleID
	}

	if options.SalePriceMultiplier != nil {
		createReq.SalePriceMultiplier = options.SalePriceMultiplier
	}

	if options.DiscountPriceMultiplier != nil {
		createReq.DiscountPriceMultiplier = options.DiscountPriceMultiplier
	}

	// 设置筛选规则ID和范围 - 参考result_service.go的价格范围格式化逻辑
	if options.FilterRuleID != nil {
		createReq.FilterRuleId = options.FilterRuleID
	}

	if options.FilterRuleRange != nil {
		createReq.FilterRuleRange = options.FilterRuleRange
	}

	return createReq
}

// queryCreatedMapping 查询创建的映射关系
func (b *MappingBuilder) queryCreatedMapping(skuCode string, storeID int64) (*managementapi.ProductImportMappingRespDTO, error) {
	return b.mappingClient.GetProductImportMappingByPlatformProductIdAndStore(
		&managementapi.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO{
			PlatformProductId: skuCode,
			StoreId:           storeID,
		},
	)
}

// determineRegion 确定区域
func (b *MappingBuilder) determineRegion(storeInfo *managementapi.StoreRespDTO) string {
	if storeInfo != nil && storeInfo.Region != "" {
		return storeInfo.Region
	}
	return "US" // 默认区域
}

// enrichOptionsFromSkuInfo 从SKU信息中丰富选项
func (b *MappingBuilder) enrichOptionsFromSkuInfo(options *MappingCreateOptions, skuInfo any) {
	// 这里可以根据SKU信息设置更多字段
	// 例如：成本价、规则ID等
	b.logger.WithField("sku_code", options.SkuCode).Debug("使用SKU详细信息丰富映射关系")

	// TODO: 根据实际的SKU信息结构体来设置字段
	// 例如：
	// if skuInfo.CostPrice > 0 {
	//     options.CostPrice = &skuInfo.CostPrice
	// }
}

// enrichOptionsFromProductInfo 从产品信息中丰富选项
func (b *MappingBuilder) enrichOptionsFromProductInfo(options *MappingCreateOptions, productInfo any) {
	b.logger.WithField("sku_code", options.SkuCode).Debug("使用产品信息丰富映射关系")

	// TODO: 根据实际的产品信息结构体来设置字段
	// 例如：
	// if productInfo.ASIN != "" {
	//     options.ProductID = productInfo.ASIN
	// }
	// if productInfo.ParentASIN != "" {
	//     options.ParentProductID = &productInfo.ParentASIN
	// }
}

// enrichOptionsFromHistory 从历史映射信息中丰富选项
func (b *MappingBuilder) enrichOptionsFromHistory(options *MappingCreateOptions, historyMapping *managementapi.ProductImportMappingRespDTO) {
	b.logger.WithField("sku_code", options.SkuCode).Debug("使用历史映射信息丰富映射关系")

	// 从历史映射中复用一些字段
	if historyMapping.ProductId != "" {
		options.ProductID = historyMapping.ProductId
	}

	if historyMapping.ParentProductId != nil && *historyMapping.ParentProductId != "" {
		options.ParentProductID = historyMapping.ParentProductId
	}

	if historyMapping.PlatformParentProductId != nil && *historyMapping.PlatformParentProductId != "" {
		options.PlatformParentProductID = historyMapping.PlatformParentProductId
	}

	if historyMapping.CostPrice != nil {
		options.CostPrice = historyMapping.CostPrice
	}

	if historyMapping.ProfitRuleId != nil {
		options.ProfitRuleID = historyMapping.ProfitRuleId
	}

	// 转换float64到string类型
	if historyMapping.SalePriceMultiplier != nil {
		salePriceMultiplier := fmt.Sprintf("%.2f", *historyMapping.SalePriceMultiplier)
		options.SalePriceMultiplier = &salePriceMultiplier
	}

	if historyMapping.DiscountPriceMultiplier != nil {
		discountPriceMultiplier := fmt.Sprintf("%.2f", *historyMapping.DiscountPriceMultiplier)
		options.DiscountPriceMultiplier = &discountPriceMultiplier
	}

	if historyMapping.FilterRuleId != nil {
		options.FilterRuleID = historyMapping.FilterRuleId
	}

	if historyMapping.FilterRuleRange != nil {
		options.FilterRuleRange = historyMapping.FilterRuleRange
	}
}

// BatchCreateMappings 批量创建映射关系
func (b *MappingBuilder) BatchCreateMappings(optionsList []*MappingCreateOptions) ([]*managementapi.ProductImportMappingRespDTO, []error) {
	results := make([]*managementapi.ProductImportMappingRespDTO, len(optionsList))
	errors := make([]error, len(optionsList))

	b.logger.WithField("count", len(optionsList)).Info("开始批量创建映射关系")

	for i, options := range optionsList {
		result, err := b.CreateMappingRelation(options)
		results[i] = result
		errors[i] = err

		if err != nil {
			b.logger.WithError(err).WithField("sku_code", options.SkuCode).Error("批量创建中的单个映射关系失败")
		}
	}

	successCount := 0
	for _, err := range errors {
		if err == nil {
			successCount++
		}
	}

	b.logger.WithFields(logrus.Fields{
		"total":   len(optionsList),
		"success": successCount,
		"failed":  len(optionsList) - successCount,
	}).Info("批量创建映射关系完成")

	return results, errors
}
