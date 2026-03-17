package image

import (
	"fmt"
	"task-processor/internal/shein"

	"github.com/sirupsen/logrus"
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
	logrus.Infof("=== 开始图片数量验证 ===")

	// 检查产品数据是否存在
	if ctx.AmazonProduct == nil {
		logrus.Errorf("❌ Amazon产品数据未获取")
		return fmt.Errorf("Amazon产品数据未获取，请先执行获取产品数据步骤")
	}

	// 统计有效图片数量
	validImageCount := 0
	for _, img := range ctx.AmazonProduct.Images {
		if img != "" {
			validImageCount++
		}
	}

	logrus.Infof("📊 产品图片统计:")
	logrus.Infof("  - ASIN: %s", ctx.AmazonProduct.Asin)
	logrus.Infof("  - 总图片数量: %d", len(ctx.AmazonProduct.Images))
	logrus.Infof("  - 有效图片数量: %d", validImageCount)
	logrus.Infof("  - 最小要求数量: %d", h.minImageCount)

	// 验证图片数量
	if validImageCount < h.minImageCount {
		logrus.Errorf("❌ 图片数量不足: 当前=%d, 要求=%d", validImageCount, h.minImageCount)
		logrus.Errorf("❌ SHEIN平台要求: 至少需要1张主图 + 2张细节图")

		// 返回不可重试错误，避免浪费资源
		return shein.NewNonRetryableError(
			fmt.Sprintf("产品图片数量不足，当前有%d张有效图片，SHEIN要求至少%d张（1张主图+2张细节图）",
				validImageCount, h.minImageCount),
			nil,
		)
	}

	logrus.Infof("✅ 图片数量验证通过")
	logrus.Infof("=== 图片数量验证完成 ===")
	return nil
}
