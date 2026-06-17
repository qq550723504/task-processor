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
		logger.GetGlobalLogger("shein/product").Warnf(
			"custom attribute value rejected after sanitize: attrID=%d raw=%q sanitized=%q required=%v shouldContinue=%v",
			attrID, attrValue, sanitizedValue, isRequired, !isRequired,
		)
		return CustomAttributeResult{Success: false, ShouldContinue: !isRequired}
	}
	if sanitizedValue != attrValue {
		logger.GetGlobalLogger("shein/product").Infof(
			"custom attribute value sanitized: attrID=%d raw=%q sanitized=%q",
			attrID, attrValue, sanitizedValue,
		)
	}

	validateResponse, err := runtime.AttributeAPI.ValidateCustomAttributeValue(attrID, sanitizedValue, runtime.CategoryID, runtime.ProductTitle)
	if err != nil {
		logger.GetGlobalLogger("shein/product").Warnf(
			"validate custom attribute value failed: attrID=%d value=%q categoryID=%d required=%v shouldContinue=%v err=%v",
			attrID, sanitizedValue, runtime.CategoryID, isRequired, !isRequired, err,
		)
		return CustomAttributeResult{Success: false, ShouldContinue: !isRequired}
	}
	if validateResponse == nil {
		logger.GetGlobalLogger("shein/product").Warnf(
			"validate custom attribute value returned nil response: attrID=%d value=%q categoryID=%d required=%v shouldContinue=%v",
			attrID, sanitizedValue, runtime.CategoryID, isRequired, !isRequired,
		)
		return CustomAttributeResult{Success: false, ShouldContinue: !isRequired}
	}
	if validateResponse.Data.AttributeID == 0 || validateResponse.Data.PreAttributeValueID == 0 {
		logger.GetGlobalLogger("shein/product").Warnf(
			"validate custom attribute value not accepted: attrID=%d value=%q categoryID=%d responseAttrID=%d preAttrValueID=%d required=%v shouldContinue=%v",
			attrID,
			sanitizedValue,
			runtime.CategoryID,
			validateResponse.Data.AttributeID,
			validateResponse.Data.PreAttributeValueID,
			isRequired,
			!isRequired,
		)
		return CustomAttributeResult{Success: false, ShouldContinue: !isRequired}
	}

	p.normalizeTranslatedNameMultis(&validateResponse.Data.AttributeValueNameMultis, sanitizedValue)
	nameMultis := p.convertToAttributeValueNameMultis(validateResponse.Data.AttributeValueNameMultis)
	if len(nameMultis) == 0 {
		logger.GetGlobalLogger("shein/product").Warnf(
			"custom attribute value missing translated names: attrID=%d value=%q preAttrValueID=%d required=%v shouldContinue=%v",
			attrID, sanitizedValue, validateResponse.Data.PreAttributeValueID, isRequired, !isRequired,
		)
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
		logger.GetGlobalLogger("shein/product").Warnf(
			"add custom attribute value failed: attrID=%d value=%q categoryID=%d preAttrValueID=%d required=%v shouldContinue=%v err=%v",
			attrID,
			sanitizedValue,
			runtime.CategoryID,
			validateResponse.Data.PreAttributeValueID,
			isRequired,
			!isRequired,
			err,
		)
		return CustomAttributeResult{Success: false, ShouldContinue: !isRequired}
	}
	if addResponse == nil {
		logger.GetGlobalLogger("shein/product").Warnf(
			"add custom attribute value returned nil response: attrID=%d value=%q categoryID=%d preAttrValueID=%d required=%v shouldContinue=%v",
			attrID,
			sanitizedValue,
			runtime.CategoryID,
			validateResponse.Data.PreAttributeValueID,
			isRequired,
			!isRequired,
		)
		return CustomAttributeResult{Success: false, ShouldContinue: !isRequired}
	}

	if len(addResponse.Info.Data.CustomAttributeRelation) > 0 {
		newValueID := int(addResponse.Info.Data.CustomAttributeRelation[0].AttributeValueID)
		logger.GetGlobalLogger("shein/product").Infof(
			"custom attribute value created: attrID=%d value=%q newValueID=%d relationCount=%d",
			attrID, sanitizedValue, newValueID, len(addResponse.Info.Data.CustomAttributeRelation),
		)
		return CustomAttributeResult{
			Success:        true,
			NewValueID:     newValueID,
			Relations:      addResponse.Info.Data.CustomAttributeRelation,
			ShouldContinue: true,
		}
	}

	logger.GetGlobalLogger("shein/product").Warnf(
		"add custom attribute value returned no relation: attrID=%d value=%q categoryID=%d code=%q msg=%q required=%v shouldContinue=%v",
		attrID,
		sanitizedValue,
		runtime.CategoryID,
		addResponse.Code,
		addResponse.Msg,
		isRequired,
		!isRequired,
	)
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
