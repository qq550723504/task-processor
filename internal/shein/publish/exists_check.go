package publish

import (
	"fmt"

	"task-processor/internal/core/logger"
	management_api "task-processor/internal/infra/clients/management/api"
	shein "task-processor/internal/shein"
)

// ProductExistsCheckHandler checks whether the product or variants already exist.
type ProductExistsCheckHandler struct {
	checker *PublishProductChecker
}

// NewProductExistsCheckHandler creates a new product existence handler.
func NewProductExistsCheckHandler() *ProductExistsCheckHandler {
	return &ProductExistsCheckHandler{
		checker: NewPublishProductChecker(),
	}
}

// Name returns the handler name.
func (h *ProductExistsCheckHandler) Name() string {
	return "产品存在性检查"
}

// Handle checks both the main product and variants before publish.
func (h *ProductExistsCheckHandler) Handle(ctx *shein.TaskContext) error {
	logger.GetGlobalLogger("shein/publish").Info("🔍 开始检查产品是否已上架...")

	input, err := buildExistenceCheckInput(ctx)
	if err != nil {
		return err
	}
	if input.ManagementClientMgr == nil {
		logger.GetGlobalLogger("shein/publish").Warn("management client manager is nil, skip existence check")
		return nil
	}
	if input.Task == nil {
		return shein.NewNonRetryableError("任务信息未初始化", nil)
	}

	mappingClient := input.ManagementClientMgr.GetProductImportMappingClient()
	if mappingClient == nil {
		logger.GetGlobalLogger("shein/publish").Warn("product import mapping client is nil, skip existence check")
		return nil
	}

	if err := h.checkMainProduct(input, mappingClient); err != nil {
		return err
	}
	if err := h.checkVariantProducts(input, mappingClient); err != nil {
		return err
	}

	logger.GetGlobalLogger("shein/publish").Info("✅ 产品存在性检查完成")
	return nil
}

func (h *ProductExistsCheckHandler) checkMainProduct(input *ExistenceCheckInput, mappingClient management_api.ProductImportMappingAPI) error {
	if input.Task.ProductID == "" {
		logger.GetGlobalLogger("shein/publish").Debug("main product id is empty, skip main product existence check")
		return nil
	}

	req := &management_api.ProductImportMappingCheckReqDTO{
		StoreId:   input.Task.StoreID,
		Platform:  input.Task.Platform,
		Region:    input.Task.Region,
		ProductId: input.Task.ProductID,
	}

	exists, err := mappingClient.CheckProductExists(req)
	if err != nil {
		logger.GetGlobalLogger("shein/publish").Errorf("check main product %s existence failed: %v", input.Task.ProductID, err)
		return shein.NewRetryableError("检查主产品是否已上架失败", err)
	}
	if exists {
		logger.GetGlobalLogger("shein/publish").Warnf("main product %s already exists, skip publish", input.Task.ProductID)
		return shein.NewNonRetryableError(fmt.Sprintf("主产品 %s 已经上架过", input.Task.ProductID), nil)
	}

	logger.GetGlobalLogger("shein/publish").Infof("main product %s is not published yet", input.Task.ProductID)
	return nil
}

func (h *ProductExistsCheckHandler) checkVariantProducts(input *ExistenceCheckInput, mappingClient management_api.ProductImportMappingAPI) error {
	if input.Variants == nil || len(*input.Variants) == 0 {
		logger.GetGlobalLogger("shein/publish").Debug("no variants, skip variant existence check")
		return nil
	}

	logger.GetGlobalLogger("shein/publish").Infof("checking %d variants for existing publish records", len(*input.Variants))
	for i, variant := range *input.Variants {
		if variant.Asin == "" {
			logger.GetGlobalLogger("shein/publish").Debugf("variant[%d/%d] ASIN is empty, skip", i+1, len(*input.Variants))
			continue
		}
		if err := h.checkSingleVariant(input, mappingClient, variant.Asin, i+1, len(*input.Variants)); err != nil {
			logger.GetGlobalLogger("shein/publish").Warnf("variant[%d/%d] %s existence check failed: %v", i+1, len(*input.Variants), variant.Asin, err)
		}
	}
	return nil
}

func (h *ProductExistsCheckHandler) checkSingleVariant(input *ExistenceCheckInput, mappingClient management_api.ProductImportMappingAPI, asin string, index, total int) error {
	req := &management_api.ProductImportMappingCheckReqDTO{
		StoreId:   input.Task.StoreID,
		Platform:  input.Task.Platform,
		Region:    input.Task.Region,
		ProductId: asin,
	}

	exists, err := mappingClient.CheckProductExists(req)
	if err != nil {
		logger.GetGlobalLogger("shein/publish").Errorf("check variant[%d/%d] %s existence failed: %v", index, total, asin, err)
		return err
	}

	if exists {
		logger.GetGlobalLogger("shein/publish").Warnf("variant[%d/%d] %s already exists", index, total, asin)
		if input.SetVariantFilteredFn != nil {
			input.SetVariantFilteredFn(asin, true, fmt.Sprintf("产品 %s 已经上架过", asin))
		}
		return nil
	}

	logger.GetGlobalLogger("shein/publish").Debugf("variant[%d/%d] %s is not published yet", index, total, asin)
	return nil
}
