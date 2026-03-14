package product

import (
	"fmt"
	"task-processor/internal/domain/model"
	"task-processor/internal/pipeline"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// ProductExistsCheckHandler 产品存在性检查处理器（检查主产品和变体是否已上架）
type ProductExistsCheckHandler struct {
	logger        *logrus.Entry
	mappingClient api.ProductImportMappingAPI
}

// NewProductExistsCheckHandler 创建新的产品存在性检查处理器
func NewProductExistsCheckHandler(mappingClient api.ProductImportMappingAPI) *ProductExistsCheckHandler {
	return &ProductExistsCheckHandler{
		logger:        logrus.WithField("handler", "ProductExistsCheckHandler"),
		mappingClient: mappingClient,
	}
}

// Name 返回处理器名称
func (h *ProductExistsCheckHandler) Name() string {
	return "产品存在性检查处理器"
}

// Handle 处理任务 - 检查主产品和所有变体是否已上架
func (h *ProductExistsCheckHandler) Handle(ctx pipeline.TaskContext) error {
	h.logger.Info("🔍 开始检查产品和变体是否已上架")

	// 检查必要的上下文信息
	if h.mappingClient == nil {
		h.logger.Error("产品导入映射客户端未初始化，无法检查产品是否已上架")
		return fmt.Errorf("产品导入映射客户端未初始化")
	}

	task := ctx.GetTask()
	if task == nil {
		h.logger.Error("任务信息未初始化，无法检查产品是否已上架")
		return fmt.Errorf("任务信息未初始化")
	}

	// 检查主产品是否已上架
	if task.ProductID != "" {
		if err := h.checkProductExists(task.StoreID, task.Platform, task.Region, task.ProductID, "主产品"); err != nil {
			return err
		}
	}

	// 从主产品信息中检查所有变体ASIN是否已上架
	var amazonProduct *model.Product
	if amazonCtx, ok := ctx.(pipeline.AmazonContext); ok {
		amazonProduct = amazonCtx.GetAmazonProduct()
	}

	if amazonProduct != nil && len(amazonProduct.Variations) > 0 {
		variantCount := len(amazonProduct.Variations)
		h.logger.Infof("📦 检查 %d 个变体是否已上架...", variantCount)

		// 检查变体数量限制
		if variantCount > 100 {
			h.logger.Errorf("❌ 变体数量过多（%d个），超过限制（100个），停止处理", variantCount)
			return types.NewNonRetryableError(fmt.Sprintf("变体数量过多（%d个），超过限制（100个）", variantCount), nil)
		}

		for i, variation := range amazonProduct.Variations {
			if variation.Asin != "" {
				if err := h.checkProductExists(task.StoreID, task.Platform, task.Region, variation.Asin, fmt.Sprintf("变体[%d/%d]", i+1, variantCount)); err != nil {
					return err
				}
			}
		}
		h.logger.Infof("✅ 所有变体检查完成，均未上架")
	}

	h.logger.Info("✅ 产品存在性检查通过")
	return nil
}

// checkProductExists 检查单个产品是否已上架
func (h *ProductExistsCheckHandler) checkProductExists(storeID int64, platform, region, productID, productType string) error {
	req := &api.ProductImportMappingCheckReqDTO{
		StoreId:   storeID,
		Platform:  platform,
		Region:    region,
		ProductId: productID,
	}

	exists, err := h.mappingClient.CheckProductExists(req)
	if err != nil {
		h.logger.Errorf("检查%s %s 是否已上架失败: %v", productType, productID, err)
		// 检查失败可能是网络问题，返回可重试错误
		return fmt.Errorf("检查%s是否已上架失败: %w", productType, err)
	}

	if exists {
		h.logger.Warnf("⚠️ %s %s 已经上架过，跳过本次上架", productType, productID)
		return fmt.Errorf("NONRETRYABLE: %s %s 已经上架过", productType, productID)
	}

	h.logger.Debugf("✅ %s %s 未上架", productType, productID)
	return nil
}
