package handlers

import (
	"fmt"
	"task-processor/common/pipeline"
	"task-processor/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// BuildSpuHandler SPU构建处理器
type BuildSpuHandler struct {
	logger *logrus.Entry
}

// NewBuildSpuHandler 创建新的SPU构建处理器
func NewBuildSpuHandler() *BuildSpuHandler {
	return &BuildSpuHandler{
		logger: logrus.WithField("handler", "BuildSpuHandler"),
	}
}

// Name 返回处理器名称
func (h *BuildSpuHandler) Name() string {
	return "SPU构建处理器"
}

// Handle 处理任务
func (h *BuildSpuHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始构建SPU")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 构建SPU
	err := h.buildSpu(ctx)
	if err != nil {
		h.logger.Errorf("构建SPU失败: %v", err)
		return fmt.Errorf("构建SPU失败: %w", err)
	}

	h.logger.Info("SPU构建完成")
	return nil
}

// buildSpu 构建SPU
func (h *BuildSpuHandler) buildSpu(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始构建产品SPU信息")

	// 构建基本信息
	err := h.buildBasicInfo(ctx)
	if err != nil {
		return fmt.Errorf("构建基本信息失败: %w", err)
	}

	// 构建SKC和SKU
	err = h.buildSkcAndSku(ctx)
	if err != nil {
		return fmt.Errorf("构建SKC和SKU失败: %w", err)
	}

	// 构建产品属性
	err = h.buildProductProperties(ctx)
	if err != nil {
		return fmt.Errorf("构建产品属性失败: %w", err)
	}

	h.logger.Info("SPU构建完成")
	return nil
}

// buildBasicInfo 构建基本信息
func (h *BuildSpuHandler) buildBasicInfo(ctx *pipeline.TaskContext) error {
	h.logger.Info("构建产品基本信息")

	// 设置商品创建时间
	if ctx.TemuProduct.GoodsBasic.GoodsCreateTime == 0 {
		ctx.TemuProduct.GoodsBasic.GoodsCreateTime = 1700000000 // 模拟时间戳
	}

	// 设置语言
	if ctx.TemuProduct.GoodsBasic.Lang == "" {
		ctx.TemuProduct.GoodsBasic.Lang = "en"
	}

	// 设置允许的站点
	if len(ctx.TemuProduct.GoodsBasic.AllowSite) == 0 {
		ctx.TemuProduct.GoodsBasic.AllowSite = []int{1} // 默认美国站点
	}

	// 设置商品类型
	if ctx.TemuProduct.GoodsBasic.GoodsType == 0 {
		ctx.TemuProduct.GoodsBasic.GoodsType = 1 // 普通商品
	}

	// 设置上架状态
	if ctx.TemuProduct.GoodsBasic.IsOnSale == 0 {
		ctx.TemuProduct.GoodsBasic.IsOnSale = 1 // 上架
	}

	// 设置来源
	if ctx.TemuProduct.GoodsBasic.Source == 0 {
		ctx.TemuProduct.GoodsBasic.Source = 1 // 自建商品
	}

	h.logger.Info("产品基本信息构建完成")
	return nil
}

// buildSkcAndSku 构建SKC和SKU
func (h *BuildSpuHandler) buildSkcAndSku(ctx *pipeline.TaskContext) error {
	h.logger.Info("构建SKC和SKU信息")

	// 如果没有SKC列表，创建默认的
	if len(ctx.TemuProduct.SkcList) == 0 {
		defaultSkc := h.createDefaultSkc(ctx)
		ctx.TemuProduct.SkcList = []types.Skc{defaultSkc}
		h.logger.Info("创建默认SKC")
	}

	// 处理每个SKC
	for i, skc := range ctx.TemuProduct.SkcList {
		// 设置SKC ID
		if skc.SkcID == "" {
			ctx.TemuProduct.SkcList[i].SkcID = h.generateSkcID(i)
		}

		// 设置SKC完成状态
		ctx.TemuProduct.SkcList[i].SkcComplete = true
		ctx.TemuProduct.SkcList[i].Priority = i + 1

		// 处理SKU列表
		if len(skc.SkuList) == 0 {
			defaultSku := h.createDefaultSku(ctx, skc.SkcID)
			ctx.TemuProduct.SkcList[i].SkuList = []types.Sku{defaultSku}
			h.logger.Infof("为SKC[%d]创建默认SKU", i+1)
		}

		// 处理每个SKU
		for j, sku := range skc.SkuList {
			// 设置SKU ID
			if sku.SkuID == "" {
				ctx.TemuProduct.SkcList[i].SkuList[j].SkuID = h.generateSkuID(i, j)
			}

			// 设置SKU完成状态
			ctx.TemuProduct.SkcList[i].SkuList[j].SkuComplete = true
			ctx.TemuProduct.SkcList[i].SkuList[j].Priority = j + 1
			ctx.TemuProduct.SkcList[i].SkuList[j].SkcId = skc.SkcID

			// 设置默认价格（如果没有设置）
			if sku.Price == 0 {
				ctx.TemuProduct.SkcList[i].SkuList[j].Price = 1999 // 默认19.99美元
				ctx.TemuProduct.SkcList[i].SkuList[j].PriceStr = "19.99"
				ctx.TemuProduct.SkcList[i].SkuList[j].Currency = "USD"
			}

			// 设置默认库存
			if sku.Quantity == 0 {
				ctx.TemuProduct.SkcList[i].SkuList[j].Quantity = 100
			}

			h.logger.Infof("处理SKU[%d-%d]: ID=%s, 价格=%s %s, 库存=%d",
				i+1, j+1, sku.SkuID, sku.PriceStr, sku.Currency, sku.Quantity)
		}
	}

	h.logger.Infof("SKC和SKU构建完成: %d个SKC, 总计%d个SKU",
		len(ctx.TemuProduct.SkcList), h.getTotalSkuCount(ctx.TemuProduct.SkcList))
	return nil
}

// buildProductProperties 构建产品属性
func (h *BuildSpuHandler) buildProductProperties(ctx *pipeline.TaskContext) error {
	h.logger.Info("构建产品属性")

	// 设置产品描述
	if ctx.TemuProduct.GoodsExtensionInfo.GoodsDesc == "" {
		if ctx.AmazonProduct != nil && ctx.AmazonProduct.Description != "" {
			ctx.TemuProduct.GoodsExtensionInfo.GoodsDesc = ctx.AmazonProduct.Description
		} else {
			ctx.TemuProduct.GoodsExtensionInfo.GoodsDesc = "High quality product with excellent features."
		}
	}

	// 设置要点描述
	if len(ctx.TemuProduct.GoodsExtensionInfo.BulletPoints) == 0 {
		ctx.TemuProduct.GoodsExtensionInfo.BulletPoints = []string{
			"High quality materials",
			"Excellent craftsmanship",
			"Fast shipping",
			"Great customer service",
		}
	}

	// 设置原产地信息
	if ctx.TemuProduct.GoodsExtensionInfo.GoodsOriginInfo.OriginRegionName1 == "" {
		ctx.TemuProduct.GoodsExtensionInfo.GoodsOriginInfo.OriginRegionName1 = "China"
	}

	h.logger.Info("产品属性构建完成")
	return nil
}

// createDefaultSkc 创建默认SKC
func (h *BuildSpuHandler) createDefaultSkc(ctx *pipeline.TaskContext) types.Skc {
	return types.Skc{
		SkcID:            h.generateSkcID(0),
		SkcComplete:      true,
		Priority:         1,
		CommitDeleteType: 0,
		CommitDeleted:    0,
		CarouselGallery:  make([]types.ImageInfo, 0),
		ColorImageUrl:    "",
		Spec:             make([]types.SpecInfo, 0),
		SkuList:          make([]types.Sku, 0),
	}
}

// createDefaultSku 创建默认SKU
func (h *BuildSpuHandler) createDefaultSku(ctx *pipeline.TaskContext, skcID string) types.Sku {
	return types.Sku{
		SkuID:                    h.generateSkuID(0, 0),
		SkcId:                    skcID,
		SkuComplete:              true,
		Priority:                 1,
		Price:                    1999, // 19.99美元
		PriceStr:                 "19.99",
		Currency:                 "USD",
		Quantity:                 100,
		MaxRetailPrice:           2999, // 29.99美元
		MaxRetailPriceStr:        "29.99",
		RetailPriceCurrency:      "USD",
		SupplierPrice:            999, // 9.99美元
		SupplierPriceStr:         "9.99",
		UseEstimateSupplierPrice: false,
		SkuDeleted:               false,
		CarouselGallery:          make([]types.ImageInfo, 0),
		Spec:                     make([]types.SpecInfo, 0),
		SkuPriceDocuments:        make([]types.SkuPriceDocument, 0),
		ProductExpressInfo: types.ProductExpressInfo{
			WeightInfo: types.WeightInfo{
				Weight: "0.5",
				Unit:   "kg",
			},
			VolumeInfo: types.VolumeInfo{
				Length: "20",
				Width:  "15",
				Height: "10",
				Unit:   "cm",
			},
		},
	}
}

// generateSkcID 生成SKC ID
func (h *BuildSpuHandler) generateSkcID(index int) string {
	return fmt.Sprintf("skc_%d_%d", 1000000+index, 123456)
}

// generateSkuID 生成SKU ID
func (h *BuildSpuHandler) generateSkuID(skcIndex, skuIndex int) string {
	return fmt.Sprintf("sku_%d_%d_%d", 2000000+skcIndex, skuIndex, 654321)
}

// getTotalSkuCount 获取总SKU数量
func (h *BuildSpuHandler) getTotalSkuCount(skcList []types.Skc) int {
	total := 0
	for _, skc := range skcList {
		total += len(skc.SkuList)
	}
	return total
}
