package attribute

import (
	"strings"

	"task-processor/internal/core/logger"
	"task-processor/internal/pkg/types"
	apiattribute "task-processor/internal/shein/api/attribute"
	sheinctx "task-processor/internal/shein/context"
)

type ValidateRepairSaleAttributeHandler struct{}

func NewValidateRepairSaleAttributeHandler() *ValidateRepairSaleAttributeHandler {
	return &ValidateRepairSaleAttributeHandler{}
}

func (h *ValidateRepairSaleAttributeHandler) Name() string {
	return "validate_repair_sale_attribute"
}

func (h *ValidateRepairSaleAttributeHandler) Handle(ctx *sheinctx.TaskContext) error {
	input, err := buildValidateRepairInput(ctx)
	if err != nil {
		return err
	}

	h.fixAttributeValueIDs(input)
	logger.GetGlobalLogger("shein/product").Info("validated and repaired sale attributes")
	return nil
}

func (h *ValidateRepairSaleAttributeHandler) fixAttributeValueIDs(input *ValidateRepairInput) *ResultSaleAttribute {
	logger.GetGlobalLogger("shein/product").Info("start repairing sale attribute value ids")

	platformValueMap := h.buildPlatformAttributeValueMap(input.AttributeTemplates)
	negativeIDCounter := -1

	for attrIndex := range input.SaleSpecResult.SaleAttributes {
		attr := &input.SaleSpecResult.SaleAttributes[attrIndex]
		attrID := attr.AttrID

		logger.GetGlobalLogger("shein/product").Infof("repairing attribute %d, value_count=%d", attrID, len(attr.AttrValue))

		platformValues, exists := platformValueMap[attrID]
		if !exists {
			logger.GetGlobalLogger("shein/product").Warnf("attribute %d not found in templates, assign negative ids", attrID)
			for valueIndex := range attr.AttrValue {
				attr.AttrValue[valueIndex].ID = types.FlexibleID(negativeIDCounter)
				logger.GetGlobalLogger("shein/product").Debugf("attribute value %q assigned negative id %d", attr.AttrValue[valueIndex].Value, negativeIDCounter)
				negativeIDCounter--
			}
			continue
		}

		for valueIndex := range attr.AttrValue {
			attrValue := &attr.AttrValue[valueIndex]
			originalValue := strings.TrimSpace(attrValue.Value)

			if platformID, found := h.findExactMatch(originalValue, platformValues); found {
				attrValue.ID = types.FlexibleID(platformID)
				logger.GetGlobalLogger("shein/product").Debugf("attribute value %q exact matched platform id %d", originalValue, platformID)
				continue
			}

			if platformID, found := h.findFuzzyMatch(originalValue, platformValues); found {
				attrValue.ID = types.FlexibleID(platformID)
				logger.GetGlobalLogger("shein/product").Debugf("attribute value %q fuzzy matched platform id %d", originalValue, platformID)
				continue
			}

			attrValue.ID = types.FlexibleID(negativeIDCounter)
			logger.GetGlobalLogger("shein/product").Debugf("attribute value %q unmatched, assigned negative id %d", originalValue, negativeIDCounter)
			negativeIDCounter--
		}
	}

	return input.SaleSpecResult
}

func (h *ValidateRepairSaleAttributeHandler) buildPlatformAttributeValueMap(attributeTemplates *apiattribute.AttributeTemplateInfo) map[int]map[string]int {
	platformValueMap := make(map[int]map[string]int)

	if attributeTemplates == nil || len(attributeTemplates.Data) == 0 {
		logger.GetGlobalLogger("shein/product").Warn("attribute templates are empty, skip platform value map build")
		return platformValueMap
	}

	for _, template := range attributeTemplates.Data {
		for _, attrInfo := range template.AttributeInfos {
			attrID := attrInfo.AttributeID
			valueMap := make(map[string]int)

			for _, valueInfo := range attrInfo.AttributeValueInfoList {
				if valueInfo.AttributeValueEn != "" {
					valueMap[strings.TrimSpace(valueInfo.AttributeValueEn)] = valueInfo.AttributeValueID
				}
				if valueInfo.AttributeValue != "" {
					valueMap[strings.TrimSpace(valueInfo.AttributeValue)] = valueInfo.AttributeValueID
				}
			}

			if len(valueMap) > 0 {
				platformValueMap[attrID] = valueMap
				logger.GetGlobalLogger("shein/product").Debugf("built platform value map for attribute %d, total=%d", attrID, len(valueMap))
			}
		}
	}

	logger.GetGlobalLogger("shein/product").Infof("built platform attribute value map, total_attributes=%d", len(platformValueMap))
	return platformValueMap
}

func (h *ValidateRepairSaleAttributeHandler) findExactMatch(value string, platformValues map[string]int) (int, bool) {
	if id, exists := platformValues[value]; exists {
		return id, true
	}

	valueLower := strings.ToLower(value)
	for platformValue, id := range platformValues {
		if strings.ToLower(platformValue) == valueLower {
			return id, true
		}
	}

	return 0, false
}

func (h *ValidateRepairSaleAttributeHandler) findFuzzyMatch(value string, platformValues map[string]int) (int, bool) {
	valueLower := strings.ToLower(strings.TrimSpace(value))

	for platformValue, id := range platformValues {
		platformValueLower := strings.ToLower(strings.TrimSpace(platformValue))
		if valueLower == platformValueLower {
			return id, true
		}
	}

	return 0, false
}
