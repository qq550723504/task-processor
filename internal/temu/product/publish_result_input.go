package product

import (
	"fmt"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/pkg/ptr"
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
