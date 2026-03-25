package attribute

import (
	"task-processor/internal/core/logger"
	apiattribute "task-processor/internal/shein/api/attribute"
	"task-processor/internal/shein/api/product"
	sheinctx "task-processor/internal/shein/context"
)

// FillAttributeHandler fills generated attributes into product data.
type FillAttributeHandler struct{}

func NewFillAttributeHandler() *FillAttributeHandler {
	return &FillAttributeHandler{}
}

func (h *FillAttributeHandler) Name() string {
	return "????"
}

func (h *FillAttributeHandler) Handle(ctx *sheinctx.TaskContext) error {
	input, err := buildFillAttributeInput(ctx)
	if err != nil {
		return err
	}

	h.fillProductAttributes(input)
	logger.GetGlobalLogger("shein/product").Info("??????")
	return nil
}

func (h *FillAttributeHandler) fillProductAttributes(input *FillAttributeInput) {
	productAttributeList := []product.ProductAttribute{}
	skipNonRequiredIDs := map[int]bool{
		62:      true,
		1001675: true,
		8:       true,
		1000462: true,
		147:     true,
		1002019: true,
		1000101: true,
		164:     true,
		1001466: true,
		1000100: true,
		1000088: true,
		1002027: true,
		1000099: true,
		1000105: true,
	}

	requiredAttributeMap := make(map[int]bool)
	if input.AttributeTemplates != nil && len(input.AttributeTemplates.Data) > 0 {
		for _, templateAttribute := range input.AttributeTemplates.Data[0].AttributeInfos {
			requiredAttributeMap[templateAttribute.AttributeID] = h.isAttributeRequired(templateAttribute)
		}
	}

	for _, generatedAttribute := range input.GenerateAttribute.AttributeData {
		if len(generatedAttribute.AttrValue) == 0 {
			continue
		}
		if skipNonRequiredIDs[generatedAttribute.AttrID] {
			if isRequired, exists := requiredAttributeMap[generatedAttribute.AttrID]; exists && !isRequired {
				continue
			}
		}

		lastAttrValue := generatedAttribute.AttrValue[len(generatedAttribute.AttrValue)-1]
		valueIDInt := lastAttrValue.ID.Int()
		valueID := &valueIDInt
		extraValue := ""
		if valueIDInt == 0 {
			extraValue = lastAttrValue.Value
		}

		switch generatedAttribute.AttrID {
		case 1000411:
			if valueIDInt != 0 {
				extraValue = "1"
			}
		case 62:
			if valueIDInt != 0 {
				extraValue = "100"
			}
		case 1000546:
			extraValue = "/"
			valueID = nil
		case 1002189, 1002188:
			extraValue = "150"
			valueID = nil
		case 1000078:
			extraValue = "100"
		case 1000105:
			if valueIDInt != 0 {
				extraValue = "100"
			}
		}

		productAttributeList = append(productAttributeList, product.ProductAttribute{
			AttributeID:         generatedAttribute.AttrID,
			AttributeValueID:    valueID,
			CVSuggestType:       "",
			AttributeExtraValue: extraValue,
		})
	}

	input.ProductData.ProductAttributeList = productAttributeList
}

func (h *FillAttributeHandler) isAttributeRequired(attribute apiattribute.AttributeInfo) bool {
	switch {
	case len(attribute.AttributeRemarkList) > 0:
		return true
	case attribute.AttributeLabel == 1:
		return true
	case attribute.AttributeStatus == 3:
		return true
	case attribute.AttributeIsShow == 1:
		return false
	default:
		return false
	}
}
