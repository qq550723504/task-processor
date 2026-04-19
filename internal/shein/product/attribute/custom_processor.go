package attribute

import (
	"strings"

	"task-processor/internal/core/logger"
	"task-processor/internal/shein/api/attribute"
	"task-processor/internal/shein/content"
	sheinctx "task-processor/internal/shein/context"
)

type CustomAttributeProcessor struct{}

func NewCustomAttributeProcessor() *CustomAttributeProcessor {
	return &CustomAttributeProcessor{}
}

func (p *CustomAttributeProcessor) ProcessCustomAttributeValue(ctx *sheinctx.TaskContext, attrID int, attrValue string, isRequired bool) CustomAttributeResult {
	return p.ProcessCustomAttributeValueWithRuntime(ctx, newMapperRuntimeInput(ctx), attrID, attrValue, isRequired)
}

func (p *CustomAttributeProcessor) ProcessCustomAttributeValueWithRuntime(ctx *sheinctx.TaskContext, runtime *MapperRuntimeInput, attrID int, attrValue string, isRequired bool) CustomAttributeResult {
	logger.GetGlobalLogger("shein/product").Infof("processing custom attribute value: attrID=%d value=%q required=%v", attrID, attrValue, isRequired)

	sanitizedValue := content.SanitizeForSheinAttribute(attrValue)
	if !content.IsValidForSheinAttribute(sanitizedValue) {
		return CustomAttributeResult{Success: false, ShouldContinue: !isRequired}
	}

	validateResponse, err := runtime.AttributeAPI.ValidateCustomAttributeValue(attrID, sanitizedValue, runtime.CategoryID, runtime.ProductTitle)
	if err != nil {
		return CustomAttributeResult{Success: false, ShouldContinue: !isRequired}
	}
	if validateResponse.Data.AttributeID == 0 {
		return CustomAttributeResult{Success: false, ShouldContinue: !isRequired}
	}

	p.normalizeTranslatedNameMultis(&validateResponse.Data.AttributeValueNameMultis, sanitizedValue)
	nameMultis := p.convertToAttributeValueNameMultis(validateResponse.Data.AttributeValueNameMultis)
	if len(nameMultis) == 0 {
		return CustomAttributeResult{Success: false, ShouldContinue: !isRequired}
	}

	addResponse, err := runtime.AttributeAPI.AddCustomAttributeValue(&attribute.AddCustomAttributeValueRequest{
		CategoryID: runtime.CategoryID,
		PreAttributeValueList: []attribute.PreAttributeValue{{
			AttributeID:              attrID,
			AttributeValue:           sanitizedValue,
			PreAttributeValueID:      int64(validateResponse.Data.PreAttributeValueID),
			AttributeValueNameMultis: nameMultis,
		}},
	})
	if err != nil {
		return CustomAttributeResult{Success: false, ShouldContinue: !isRequired}
	}

	if len(addResponse.Info.Data.CustomAttributeRelation) > 0 {
		newValueID := int(addResponse.Info.Data.CustomAttributeRelation[0].AttributeValueID)
		return CustomAttributeResult{
			Success:        true,
			NewValueID:     newValueID,
			Relations:      addResponse.Info.Data.CustomAttributeRelation,
			ShouldContinue: true,
		}
	}

	return CustomAttributeResult{Success: false, ShouldContinue: !isRequired}
}

func (p *CustomAttributeProcessor) convertToAttributeValueNameMultis(source []struct {
	Language                string `json:"language"`
	AttributeValueNameMulti string `json:"attribute_value_name_multi"`
	WarningType             int    `json:"warning_type"`
}) []attribute.AttributeValueNameMulti {
	if len(source) == 0 {
		return []attribute.AttributeValueNameMulti{}
	}

	result := make([]attribute.AttributeValueNameMulti, 0, len(source))
	for _, item := range source {
		if item.Language == "" || item.AttributeValueNameMulti == "" {
			continue
		}
		result = append(result, attribute.AttributeValueNameMulti{
			Language:           item.Language,
			AttributeValueName: item.AttributeValueNameMulti,
			WarningType:        item.WarningType,
		})
	}
	return result
}

func (p *CustomAttributeProcessor) normalizeTranslatedNameMultis(nameMultis *[]struct {
	Language                string `json:"language"`
	AttributeValueNameMulti string `json:"attribute_value_name_multi"`
	WarningType             int    `json:"warning_type"`
}, fallbackValue string) {
	if nameMultis == nil || len(*nameMultis) == 0 {
		return
	}

	for i := range *nameMultis {
		original := (*nameMultis)[i].AttributeValueNameMulti
		fixed := strings.ReplaceAll(original, "锛?", ",")
		fixed = strings.TrimSpace(fixed)
		if fixed == "" {
			fixed = fallbackValue
			logger.GetGlobalLogger("shein/product").Warnf(
				"SHEIN custom attribute translation is empty, fallback to source value: language=%s value=%q",
				(*nameMultis)[i].Language,
				fallbackValue,
			)
		}
		(*nameMultis)[i].AttributeValueNameMulti = fixed
	}
}
