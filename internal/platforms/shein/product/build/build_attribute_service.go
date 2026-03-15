package build

import (
	"errors"
	"fmt"
	"task-processor/internal/platforms/shein/api/attribute"
	"task-processor/internal/platforms/shein/model"
	"task-processor/internal/platforms/shein/validation"

	"github.com/sirupsen/logrus"
)

// BuildAttributeHandler 构建属性信息处理器
type BuildAttributeHandler struct {
	validator  *validation.AttributeValidator
	builder    *AttributeBuilder
	classifier *AttributeClassifier
}

// NewBuildAttributeHandler 创建新的构建属性信息处理器
func NewBuildAttributeHandler() *BuildAttributeHandler {
	validator := validation.NewAttributeValidator()
	builder := NewAttributeBuilder(validator)
	classifier := NewAttributeClassifier(builder)

	return &BuildAttributeHandler{
		validator:  validator,
		builder:    builder,
		classifier: classifier,
	}
}

// Name 返回处理器名称
func (h *BuildAttributeHandler) Name() string {
	return "构建属性信息"
}

// Handle 执行构建属性信息处理
func (h *BuildAttributeHandler) Handle(ctx *model.TaskContext) error {
	// 检查是否已获取产品数据
	if ctx.ProductData == nil {
		return fmt.Errorf("产品数据未获取，请先执行获取产品数据步骤")
	}

	// 使用带上下文的智能筛选版本
	buildInfo, err := h.BuildAttributeDataWithContext(ctx)
	if err != nil {
		return err
	}
	ctx.BuildAttributeData = &buildInfo

	return nil
}

// BuildAttributeData 构建属性数据（智能筛选版本）
func (h *BuildAttributeHandler) BuildAttributeData(attributeTemplates *attribute.AttributeTemplateInfo) (model.BuildAttributeInfo, error) {
	if len(attributeTemplates.Data) == 0 {
		return model.BuildAttributeInfo{}, errors.New("attributeTemplates is empty")
	}

	attributeInfo := model.BuildAttributeInfo{
		AttributeData:     []model.GenerateAttribute{},
		SaleAttributeData: []model.GenerateAttribute{},
	}

	// 基于attributeTemplates数据动态判断必填属性
	for _, attr := range attributeTemplates.Data[0].AttributeInfos {
		h.classifier.ClassifyAndBuildAttribute(attr, &attributeInfo)
	}

	return attributeInfo, nil
}

// BuildAttributeDataWithContext 构建属性数据（带上下文的智能筛选版本）
func (h *BuildAttributeHandler) BuildAttributeDataWithContext(ctx *model.TaskContext) (model.BuildAttributeInfo, error) {
	if ctx.AttributeTemplates == nil || len(ctx.AttributeTemplates.Data) == 0 {
		return model.BuildAttributeInfo{}, errors.New("attributeTemplates is empty")
	}

	attributeInfo := model.BuildAttributeInfo{
		AttributeData:     []model.GenerateAttribute{},
		SaleAttributeData: []model.GenerateAttribute{},
	}

	// 使用智能筛选器筛选相关的销售属性
	relevantSaleAttributes := h.classifier.filter.FilterRelevantAttributes(ctx, ctx.AttributeTemplates)

	logrus.Infof("🎯 智能筛选结果: 筛选出 %d 个相关销售属性", len(relevantSaleAttributes))

	// 处理所有属性，但只有相关的销售属性会被标记为必填
	relevantSaleAttrMap := make(map[int]bool)
	for _, attr := range relevantSaleAttributes {
		relevantSaleAttrMap[attr.AttributeID] = true
	}

	for _, attr := range ctx.AttributeTemplates.Data[0].AttributeInfos {
		switch attr.AttributeType {
		case 4: // 产品属性
			generateAttr := h.builder.BuildGenerateAttribute(attr)
			attributeInfo.AttributeData = append(attributeInfo.AttributeData, generateAttr)
		case 1: // 销售规格
			generateAttr := h.builder.BuildGenerateAttribute(attr)
			// 只有相关的销售属性才标记为必填
			if relevantSaleAttrMap[attr.AttributeID] {
				generateAttr.Required = true
				logrus.Infof("✅ 销售属性 %s (ID:%d) 标记为必填", attr.AttributeNameEn, attr.AttributeID)
			} else {
				generateAttr.Required = false
				logrus.Infof("❌ 销售属性 %s (ID:%d) 标记为非必填", attr.AttributeNameEn, attr.AttributeID)
			}
			attributeInfo.SaleAttributeData = append(attributeInfo.SaleAttributeData, generateAttr)
		case 3: // 成分属性
			generateAttr := h.builder.BuildGenerateAttribute(attr)
			attributeInfo.AttributeData = append(attributeInfo.AttributeData, generateAttr)
		}
	}

	return attributeInfo, nil
}

