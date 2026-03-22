package product

import (
	"fmt"
	"regexp"
	"strings"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/temu/context"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// BrandClearHandler 品牌清除处理器 - 从TEMU产品的文本字段中清除品牌名称
type BrandClearHandler struct {
	logger *logrus.Entry
}

// NewBrandClearHandler 创建品牌清除处理器
func NewBrandClearHandler() *BrandClearHandler {
	return &BrandClearHandler{
		logger: logger.GetGlobalLogger("brand_clear"),
	}
}

// Name 返回处理器名称
func (h *BrandClearHandler) Name() string {
	return "BrandClearHandler"
}

// HandleTemu 处理品牌清除逻辑（实现TemuHandler接口）
func (h *BrandClearHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始从TEMU产品文本字段中清除品牌名称")

	// 直接从强类型上下文获取品牌名称
	brandName := ""
	amazonProduct := temuCtx.GetAmazonProduct()
	if amazonProduct != nil && amazonProduct.Brand != "" {
		brandName = amazonProduct.Brand
		h.logger.Infof("检测到品牌名称: %s", brandName)
	}

	if brandName == "" {
		h.logger.Info("未检测到品牌名称，跳过清除")
		return nil
	}

	temuProduct := temuCtx.TemuProduct

	// 清除商品名称中的品牌
	if temuProduct.GoodsBasic.GoodsName != "" {
		originalName := temuProduct.GoodsBasic.GoodsName
		temuProduct.GoodsBasic.GoodsName = h.removeBrandFromText(temuProduct.GoodsBasic.GoodsName, brandName)
		if originalName != temuProduct.GoodsBasic.GoodsName {
			h.logger.Infof("商品名称已清除品牌: %s -> %s", originalName, temuProduct.GoodsBasic.GoodsName)
		}
	}

	// 清除商品描述中的品牌
	if temuProduct.GoodsExtensionInfo.GoodsDesc != "" {
		originalDesc := temuProduct.GoodsExtensionInfo.GoodsDesc
		temuProduct.GoodsExtensionInfo.GoodsDesc = h.removeBrandFromText(temuProduct.GoodsExtensionInfo.GoodsDesc, brandName)
		if originalDesc != temuProduct.GoodsExtensionInfo.GoodsDesc {
			h.logger.Info("商品描述已清除品牌")
		}
	}

	// 清除要点描述中的品牌
	if len(temuProduct.GoodsExtensionInfo.BulletPoints) > 0 {
		for i, point := range temuProduct.GoodsExtensionInfo.BulletPoints {
			originalPoint := point
			temuProduct.GoodsExtensionInfo.BulletPoints[i] = h.removeBrandFromText(point, brandName)
			if originalPoint != temuProduct.GoodsExtensionInfo.BulletPoints[i] {
				h.logger.Infof("要点[%d]已清除品牌", i)
			}
		}
	}

	// 清除GoodsBrandProperties（品牌属性列表）
	if len(temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsBrandProperties) > 0 {
		h.logger.Infof("清除GoodsBrandProperties，原有%d个品牌属性",
			len(temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsBrandProperties))
		temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsBrandProperties = []any{}
	}

	// 设置NotSelectBrand为true（表示不选择品牌）
	if !temuProduct.GoodsExtensionInfo.GoodsTrademark.NotSelectBrand {
		h.logger.Info("设置NotSelectBrand为true")
		temuProduct.GoodsExtensionInfo.GoodsTrademark.NotSelectBrand = true
	}

	h.logger.Info("TEMU产品文本字段中的品牌名称已清除")
	return nil
}

// removeBrandFromText 从文本中移除品牌名称
func (h *BrandClearHandler) removeBrandFromText(text, brandName string) string {
	if text == "" || brandName == "" {
		return text
	}

	// 创建多种品牌名称的变体进行匹配
	brandVariants := []string{
		brandName,                  // 原始品牌名
		strings.ToUpper(brandName), // 全大写
		strings.ToLower(brandName), // 全小写
	}

	result := text
	for _, variant := range brandVariants {
		// 移除品牌名称（包括前后可能的空格、逗号、破折号等）
		result = strings.ReplaceAll(result, variant+" ", "")
		result = strings.ReplaceAll(result, " "+variant, "")
		result = strings.ReplaceAll(result, variant+",", "")
		result = strings.ReplaceAll(result, ","+variant, "")
		result = strings.ReplaceAll(result, variant+"-", "")
		result = strings.ReplaceAll(result, "-"+variant, "")
		result = strings.ReplaceAll(result, variant+"'s", "")
		result = strings.ReplaceAll(result, variant, "")
	}

	// 清理多余的空格
	result = strings.TrimSpace(result)
	result = strings.Join(strings.Fields(result), " ")

	// 移除逗号前的空格（TEMU要求：逗号前不能有空格）
	result = regexp.MustCompile(`\s+,`).ReplaceAllString(result, ",")

	// 移除其他标点符号前的空格
	result = regexp.MustCompile(`\s+([.!?;:])`).ReplaceAllString(result, "$1")

	// 确保左括号前有空格（TEMU要求：左括号前必须有空格）
	result = regexp.MustCompile(`(\S)\(`).ReplaceAllString(result, "$1 (")

	// 确保右括号后有空格（如果后面还有字符的话）
	result = regexp.MustCompile(`\)(\S)`).ReplaceAllString(result, ") $1")

	return result
}

// Handle 兼容原有的Handler接口（用于pipeline.AddHandler）
func (h *BrandClearHandler) Handle(ctx pipeline.TaskContext) error {
	// 尝试类型断言为TemuTaskContext
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return h.HandleTemu(temuCtx)
	}
	// 如果不是TemuTaskContext，返回错误
	return fmt.Errorf("上下文类型错误，期望*temucontext.TemuTaskContext，实际类型: %T", ctx)
}
