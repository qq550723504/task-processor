package image

import (
	"fmt"
	"task-processor/internal/core/logger"
	"task-processor/internal/shein"
)

// ImageValidationHandler 图片验证处理器
// 在获取产品信息后立即验证图片数量，避免后续处理失败
type ImageValidationHandler struct {
	minImageCount int // 最小图片数量要求
}

// NewImageValidationHandler 创建新的图片验证处理器
func NewImageValidationHandler(minImageCount int) *ImageValidationHandler {
	// SHEIN 要求至少 2 张细节图 + 1 张主图 = 3 张
	if minImageCount <= 0 {
		minImageCount = 3
	}
	return &ImageValidationHandler{
		minImageCount: minImageCount,
	}
}

// Name 返回处理器名称
func (h *ImageValidationHandler) Name() string {
	return "图片数量验证"
}

// Handle 执行图片数量验证
func (h *ImageValidationHandler) Handle(ctx *shein.TaskContext) error {
	logger.GetGlobalLogger("shein/product").Infof("=== 开始图片数量验证 ===")

	// 检查产品数据是否存在
	if ctx.AmazonProduct == nil {
		logger.GetGlobalLogger("shein/product").Errorf("❌ Amazon产品数据未获取")
		return fmt.Errorf("Amazon产品数据未获取，请先执行获取产品数据步骤")
	}

	// 统计有效图片数量
	validImageCount := 0
	for _, img := range ctx.AmazonProduct.Images {
		if img != "" {
			validImageCount++
		}
	}

	logger.GetGlobalLogger("shein/product").Infof("📊 产品图片统计:")
	logger.GetGlobalLogger("shein/product").Infof("  - ASIN: %s", ctx.AmazonProduct.Asin)
	logger.GetGlobalLogger("shein/product").Infof("  - 总图片数量: %d", len(ctx.AmazonProduct.Images))
	logger.GetGlobalLogger("shein/product").Infof("  - 有效图片数量: %d", validImageCount)
	logger.GetGlobalLogger("shein/product").Infof("  - 最小要求数量: %d", h.minImageCount)

	// 验证图片数量
	if validImageCount < h.minImageCount {
		logger.GetGlobalLogger("shein/product").Errorf("❌ 图片数量不足: 当前=%d, 要求=%d", validImageCount, h.minImageCount)
		logger.GetGlobalLogger("shein/product").Errorf("❌ SHEIN平台要求: 至少需要1张主图 + 2张细节图")

		// 返回不可重试错误，避免浪费资源
		return shein.NewNonRetryableError(
			fmt.Sprintf("产品图片数量不足，当前有%d张有效图片，SHEIN要求至少%d张（1张主图+2张细节图）",
				validImageCount, h.minImageCount),
			nil,
		)
	}

	logger.GetGlobalLogger("shein/product").Infof("✅ 图片数量验证通过")
	logger.GetGlobalLogger("shein/product").Infof("=== 图片数量验证完成 ===")
	return nil
}
