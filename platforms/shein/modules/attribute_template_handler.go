package modules

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// AttributeTemplateHandler 属性模板处理器
type AttributeTemplateHandler struct {
}

// NewAttributeTemplateHandler 创建新的属性模板处理器
func NewAttributeTemplateHandler() *AttributeTemplateHandler {
	return &AttributeTemplateHandler{}
}

// Name 返回处理器名称
func (h *AttributeTemplateHandler) Name() string {
	return "获取属性模板"
}

// Handle 执行获取属性模板处理
func (h *AttributeTemplateHandler) Handle(ctx *TaskContext) error {
	// 获取分类ID，优先使用任务中的分类ID，否则使用产品数据中的分类ID
	categoryID := int(ctx.Task.CategoryID)
	if categoryID <= 0 && ctx.ProductData != nil {
		categoryID = ctx.ProductData.CategoryID
	}

	// 如果仍然没有有效的分类ID，则返回错误
	if categoryID <= 0 {
		return fmt.Errorf("无效的分类ID: %d", categoryID)
	}

	// 调用API获取属性模板
	attributeTemplates, err := ctx.ShopClient.GetAttributeTemplates(categoryID)
	if err != nil {
		return fmt.Errorf("获取属性模板失败: %w", err)
	}

	logrus.Infof("成功获取属性模板，模板数量: %d\n", len(attributeTemplates.Data))

	// 将属性模板信息存储到上下文中
	ctx.AttributeTemplates = attributeTemplates

	return nil
}
