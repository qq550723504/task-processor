package handlers

import (
	"fmt"
	"task-processor/internal/common/pipeline"

	"github.com/sirupsen/logrus"
)

// CategoryHandler 分类处理器
type CategoryHandler struct {
	logger *logrus.Entry
}

// NewCategoryHandler 创建新的分类处理器
func NewCategoryHandler() *CategoryHandler {
	return &CategoryHandler{
		logger: logrus.WithField("handler", "CategoryHandler"),
	}
}

// Name 返回处理器名称
func (h *CategoryHandler) Name() string {
	return "分类处理器"
}

// Handle 处理任务
func (h *CategoryHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始处理分类信息")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 处理分类相关信息
	err := h.processCategoryInfo(ctx)
	if err != nil {
		h.logger.Errorf("处理分类信息失败: %v", err)
		return fmt.Errorf("处理分类信息失败: %w", err)
	}

	h.logger.Info("分类信息处理完成")
	return nil
}

// processCategoryInfo 处理分类信息
func (h *CategoryHandler) processCategoryInfo(ctx *pipeline.TaskContext) error {
	catID := ctx.TemuProduct.GoodsBasic.CatID
	if catID == 0 {
		return fmt.Errorf("分类ID为空")
	}

	h.logger.Infof("处理分类信息: CatID=%d", catID)

	// 设置分类相关的产品属性
	h.setCategoryBasedProperties(ctx, catID)

	// 设置分类相关的服务承诺
	h.setCategoryBasedServicePromise(ctx, catID)

	h.logger.Info("分类信息处理完成")
	return nil
}

// setCategoryBasedProperties 根据分类设置产品属性
func (h *CategoryHandler) setCategoryBasedProperties(ctx *pipeline.TaskContext, catID int) {
	h.logger.Infof("根据分类设置产品属性: CatID=%d", catID)

	// 不要覆盖TEMU API返回的IsClothes值！
	// CommitDetailHandler已经从TEMU API获取了正确的IsClothes值
	// 这里只记录当前状态，不做修改
	h.logger.Infof("当前产品分类状态: IsClothes=%v, IsBooks=%v, CatType=%d",
		ctx.TemuProduct.GoodsBasic.IsClothes,
		ctx.TemuProduct.GoodsBasic.IsBooks,
		ctx.TemuProduct.GoodsBasic.CatType)

	// 设置其他通用属性
	ctx.TemuProduct.GoodsBasic.CanSkipRequiredProperty = false
	ctx.TemuProduct.GoodsBasic.IsShop = false
	ctx.TemuProduct.GoodsBasic.FromCopy = false
	ctx.TemuProduct.GoodsBasic.HasSubmitted = false
}

// setCategoryBasedServicePromise 根据分类设置服务承诺
func (h *CategoryHandler) setCategoryBasedServicePromise(ctx *pipeline.TaskContext, catID int) {
	h.logger.Infof("根据分类设置服务承诺: CatID=%d", catID)

	// 根据IsClothes标志设置发货时限（而不是根据CatID范围）
	if ctx.TemuProduct.GoodsBasic.IsClothes {
		ctx.TemuProduct.GoodsServicePromise.ShipmentLimitSecond = 172800 // 48小时
		h.logger.Info("设置服装类发货时限: 48小时")
	} else if ctx.TemuProduct.GoodsBasic.IsBooks {
		ctx.TemuProduct.GoodsServicePromise.ShipmentLimitSecond = 259200 // 72小时
		h.logger.Info("设置图书类发货时限: 72小时")
	} else {
		ctx.TemuProduct.GoodsServicePromise.ShipmentLimitSecond = 86400 // 24小时
		h.logger.Info("设置通用商品发货时限: 24小时")
	}

	// 设置履约类型
	ctx.TemuProduct.GoodsServicePromise.FulfillmentType = 1 // 自发货
}
