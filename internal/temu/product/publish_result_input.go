package product

import (
	"fmt"
	"strings"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/pkg/ptr"
	pkgproduct "task-processor/internal/product"
	temuapi "task-processor/internal/temu/api"
	models "task-processor/internal/temu/api/product"
	temucontext "task-processor/internal/temu/context"
)

type SavePublishResultInput struct {
	Task           *model.Task
	SubmitResponse *temuapi.SubmitResponse
	Product        *models.Product
	AmazonProduct  *model.Product
	StoreInfo      *managementapi.StoreRespDTO
	FilterRule     *managementapi.FilterRuleRespDTO
	ProfitRule     *managementapi.ProfitRuleRespDTO
	AsinSkuMap     map[string]string
}

func (input *SavePublishResultInput) SKCCount() int {
	if input == nil || input.Product == nil {
		return 0
	}
	return len(input.Product.SkcList)
}

func (input *SavePublishResultInput) SKUCount() int {
	if input == nil || input.Product == nil {
		return 0
	}

	count := 0
	for _, skc := range input.Product.SkcList {
		count += len(skc.SkuList)
	}
	return count
}

func (input *SavePublishResultInput) ForEachSKU(fn func(sku *models.Sku)) {
	if input == nil || input.Product == nil || fn == nil {
		return
	}

	for skcIndex := range input.Product.SkcList {
		skc := &input.Product.SkcList[skcIndex]
		for skuIndex := range skc.SkuList {
			fn(&skc.SkuList[skuIndex])
		}
	}
}

func (input *SavePublishResultInput) TaskLogFields() map[string]any {
	if input == nil || input.Task == nil {
		return map[string]any{}
	}

	return map[string]any{
		"task_id":    input.Task.ID,
		"tenant_id":  input.Task.TenantID,
		"store_id":   input.Task.StoreID,
		"platform":   input.Task.Platform,
		"product_id": input.Task.ProductID,
	}
}

func (input *SavePublishResultInput) TenantAndStoreIDs() (tenantID int64, storeID int64, ok bool) {
	if input == nil || input.Task == nil {
		return 0, 0, false
	}

	return input.Task.TenantID, input.Task.StoreID, true
}

func (input *SavePublishResultInput) SubmitResponseLogFields(response string) map[string]any {
	fields := input.TaskLogFields()
	fields["response"] = response
	return fields
}

func (input *SavePublishResultInput) BuildImportMappingCreateReq(sku *models.Sku) *managementapi.ProductImportMappingCreateReqDTO {
	if input == nil || input.Task == nil || sku == nil {
		return nil
	}

	return &managementapi.ProductImportMappingCreateReqDTO{
		ImportTaskId: input.Task.ID,
		TenantID:     input.Task.TenantID,
		StoreId:      input.Task.StoreID,
		Platform:     "TEMU",
		Region:       input.Task.Region,
		Sku:          &sku.OutSkuSN,
		ProductId:    "",
		Status:       ptr.Int16Ptr(1),
	}
}

func (input *SavePublishResultInput) FilterRuleID() (int64, bool) {
	if input == nil || input.FilterRule == nil {
		return 0, false
	}
	return input.FilterRule.ID, true
}

func (input *SavePublishResultInput) ProfitRuleID() (int64, bool) {
	if input == nil || input.ProfitRule == nil {
		return 0, false
	}
	return input.ProfitRule.ID, true
}

func (input *SavePublishResultInput) DailyLimitExceededReason(count int64, dailyLimit int) string {
	return fmt.Sprintf("????????(%d/%d)", count, dailyLimit)
}

func (input *SavePublishResultInput) ProductIDForSKU(sku *models.Sku) (string, bool) {
	if input == nil || sku == nil || input.AsinSkuMap == nil {
		return "", false
	}

	productID, exists := input.AsinSkuMap[sku.OutSkuSN]
	return productID, exists && productID != ""
}

func (input *SavePublishResultInput) ParentProductID() (string, bool) {
	if input == nil || input.AmazonProduct == nil || input.AmazonProduct.ParentAsin == "" {
		return "", false
	}

	return input.AmazonProduct.ParentAsin, true
}

func (input *SavePublishResultInput) CostPrice() (float64, bool) {
	if input == nil || input.AmazonProduct == nil || input.StoreInfo == nil || input.StoreInfo.PriceType == "" {
		return 0, false
	}

	costPrice := pkgproduct.GetProductPrice(input.AmazonProduct, input.StoreInfo.PriceType)
	return costPrice, costPrice > 0
}

func (input *SavePublishResultInput) FilterRuleRange() (string, bool) {
	if input == nil || input.FilterRule == nil {
		return "", false
	}

	var rangeParts []string

	if input.FilterRule.PriceMin != nil || input.FilterRule.PriceMax != nil {
		var priceRange string
		if input.FilterRule.PriceMin != nil && input.FilterRule.PriceMax != nil {
			priceRange = fmt.Sprintf("???:%.2f-%.2f", *input.FilterRule.PriceMin, *input.FilterRule.PriceMax)
		} else if input.FilterRule.PriceMin != nil {
			priceRange = fmt.Sprintf("???:>=%.2f", *input.FilterRule.PriceMin)
		} else if input.FilterRule.PriceMax != nil {
			priceRange = fmt.Sprintf("???:<=%.2f", *input.FilterRule.PriceMax)
		}
		if priceRange != "" {
			rangeParts = append(rangeParts, priceRange)
		}
	}

	if input.FilterRule.StockMin != nil {
		rangeParts = append(rangeParts, fmt.Sprintf("???:>=%d", *input.FilterRule.StockMin))
	}
	if input.FilterRule.RatingMin != nil {
		rangeParts = append(rangeParts, fmt.Sprintf("???:>=%.1f", *input.FilterRule.RatingMin))
	}
	if input.FilterRule.ReviewCountMin != nil {
		rangeParts = append(rangeParts, fmt.Sprintf("?????>=%d", *input.FilterRule.ReviewCountMin))
	}
	if input.FilterRule.DeliveryTimeMax != nil {
		rangeParts = append(rangeParts, fmt.Sprintf("??????:<=%d??", *input.FilterRule.DeliveryTimeMax))
	}
	if input.FilterRule.FulfillmentType != "" && input.FilterRule.FulfillmentType != "ALL" {
		rangeParts = append(rangeParts, fmt.Sprintf("????%s", input.FilterRule.FulfillmentType))
	}

	if len(rangeParts) == 0 {
		return "", false
	}

	return fmt.Sprintf("[%s]", strings.Join(rangeParts, ",")), true
}

func (input *SavePublishResultInput) SalePriceMultiplier() (string, bool) {
	if input == nil || input.ProfitRule == nil || input.ProfitRule.SalePriceMultiplier <= 0 {
		return "", false
	}
	return fmt.Sprintf("%.4f", input.ProfitRule.SalePriceMultiplier), true
}

func (input *SavePublishResultInput) DiscountPriceMultiplier() (string, bool) {
	if input == nil || input.ProfitRule == nil || input.ProfitRule.DiscountPriceMultiplier <= 0 {
		return "", false
	}
	return fmt.Sprintf("%.4f", input.ProfitRule.DiscountPriceMultiplier), true
}

func (input *SavePublishResultInput) DailyLimitConfig() (limit int, limitType string, ok bool) {
	if input == nil || input.StoreInfo == nil || input.StoreInfo.DailyLimit == nil {
		return 0, "", false
	}
	if *input.StoreInfo.DailyLimit <= 0 {
		return 0, "", false
	}

	limitType = input.StoreInfo.DailyLimitType
	if limitType == "" {
		limitType = "SPU"
	}

	return *input.StoreInfo.DailyLimit, limitType, true
}

func (input *SavePublishResultInput) DailyLimitIncrement(limitType string) int64 {
	if input == nil || input.Product == nil {
		return 0
	}

	switch limitType {
	case "SPU":
		return 1
	case "SKC":
		return int64(input.SKCCount())
	case "SKU":
		return int64(input.SKUCount())
	default:
		return 1
	}
}

func buildSavePublishResultInput(temuCtx *temucontext.TemuTaskContext) (*SavePublishResultInput, error) {
	if temuCtx == nil {
		return nil, fmt.Errorf("temu context is nil")
	}

	task := temuCtx.GetTask()
	if task == nil {
		return nil, fmt.Errorf("task is not initialized")
	}

	submitResponse, exists := getSubmitResponseFromContext(temuCtx)
	if !exists {
		return nil, fmt.Errorf("submit response is not initialized")
	}

	product := temuCtx.ProductData
	if product == nil {
		product = temuCtx.TemuProduct
	}

	return &SavePublishResultInput{
		Task:           task,
		SubmitResponse: submitResponse,
		Product:        product,
		AmazonProduct:  temuCtx.GetAmazonProduct(),
		StoreInfo:      temuCtx.StoreInfo,
		FilterRule:     temuCtx.FilterRule,
		ProfitRule:     temuCtx.ProfitRule,
		AsinSkuMap:     temuCtx.AsinSkuMap,
	}, nil
}
