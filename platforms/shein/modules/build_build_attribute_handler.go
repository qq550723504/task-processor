package modules

import (
	"errors"
	"fmt"
	"task-processor/common/shein/api/attribute"
)

// BuildAttributeHandler 构建属性信息处理器
type BuildAttributeHandler struct {
	validator  *AttributeValidator
	builder    *AttributeBuilder
	classifier *AttributeClassifier
}

// NewBuildAttributeHandler 创建新的构建属性信息处理器
func NewBuildAttributeHandler() *BuildAttributeHandler {
	validator := NewAttributeValidator()
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
func (h *BuildAttributeHandler) Handle(ctx *TaskContext) error {

	// 检查是否已获取产品数据
	if ctx.ProductData == nil {
		return fmt.Errorf("产品数据未获取，请先执行获取产品数据步骤")
	}

	buildInfo, err := h.BuildAttributeData(ctx.AttributeTemplates)
	if err != nil {
		return err
	}
	ctx.BuildAttributeData = &buildInfo

	return nil
}

// BuildAttributeData 构建属性数据
func (h *BuildAttributeHandler) BuildAttributeData(attributeTemplates *attribute.AttributeTemplateInfo) (BuildAttributeInfo, error) {
	if len(attributeTemplates.Data) == 0 {
		return BuildAttributeInfo{}, errors.New("attributeTemplates is empty")
	}

	attributeInfo := BuildAttributeInfo{
		AttributeData:     []GenerateAttribute{},
		SaleAttributeData: []GenerateAttribute{},
	}

	// 基于attributeTemplates数据动态判断必填属性
	for _, attr := range attributeTemplates.Data[0].AttributeInfos {
		h.classifier.ClassifyAndBuildAttribute(attr, &attributeInfo)
	}

	return attributeInfo, nil
}
