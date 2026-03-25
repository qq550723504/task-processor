package build

import (
	"errors"

	"task-processor/internal/core/logger"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/api/attribute"
	sheinattr "task-processor/internal/shein/product/attribute"
	"task-processor/internal/shein/validation"
)

type BuildAttributeHandler struct {
	validator  *validation.AttributeValidator
	builder    *AttributeBuilder
	classifier *AttributeClassifier
}

func NewBuildAttributeHandler() *BuildAttributeHandler {
	validator := validation.NewAttributeValidator()
	builder := NewAttributeBuilder(validator)
	classifier := NewAttributeClassifier(builder)
	return &BuildAttributeHandler{validator: validator, builder: builder, classifier: classifier}
}

func (h *BuildAttributeHandler) Name() string {
	return "build_attribute"
}

func (h *BuildAttributeHandler) Handle(ctx *shein.TaskContext) error {
	input, err := buildAttributeInput(ctx)
	if err != nil {
		return err
	}
	buildInfo, err := h.BuildAttributeDataWithInput(input)
	if err != nil {
		return err
	}
	ctx.SetBuildAttributeData(&buildInfo)
	return nil
}

func (h *BuildAttributeHandler) BuildAttributeData(attributeTemplates *attribute.AttributeTemplateInfo) (sheinattr.BuildAttributeInfo, error) {
	if len(attributeTemplates.Data) == 0 {
		return sheinattr.BuildAttributeInfo{}, errors.New("attribute templates are empty")
	}

	attributeInfo := sheinattr.BuildAttributeInfo{AttributeData: []sheinattr.GenerateAttribute{}, SaleAttributeData: []sheinattr.GenerateAttribute{}}
	for _, attr := range attributeTemplates.Data[0].AttributeInfos {
		h.classifier.ClassifyAndBuildAttribute(attr, &attributeInfo)
	}
	return attributeInfo, nil
}

func (h *BuildAttributeHandler) BuildAttributeDataWithInput(input *BuildAttributeInput) (sheinattr.BuildAttributeInfo, error) {
	attributeInfo := sheinattr.BuildAttributeInfo{AttributeData: []sheinattr.GenerateAttribute{}, SaleAttributeData: []sheinattr.GenerateAttribute{}}
	relevantSaleAttributes := h.classifier.filter.FilterRelevantAttributes(input.SmartFilterInput, input.AttributeTemplates)
	logger.GetGlobalLogger("shein/product").Infof("filtered relevant sale attributes: %d", len(relevantSaleAttributes))

	relevantSaleAttrMap := make(map[int]bool)
	for _, attr := range relevantSaleAttributes {
		relevantSaleAttrMap[attr.AttributeID] = true
	}

	for _, attr := range input.AttributeTemplates.Data[0].AttributeInfos {
		switch attr.AttributeType {
		case 4, 3:
			attributeInfo.AttributeData = append(attributeInfo.AttributeData, h.builder.BuildGenerateAttribute(attr))
		case 1:
			generateAttr := h.builder.BuildGenerateAttribute(attr)
			generateAttr.Required = relevantSaleAttrMap[attr.AttributeID]
			attributeInfo.SaleAttributeData = append(attributeInfo.SaleAttributeData, generateAttr)
		}
	}

	return attributeInfo, nil
}
