package attribute

import (
	"fmt"

	"task-processor/internal/core/logger"
	"task-processor/internal/pkg/types"
	"task-processor/internal/shein/api/attribute"
	sheinctx "task-processor/internal/shein/context"
)

type AttributeMapper struct {
	valueMatcher *AttributeValueMatcher
	processor    *CustomAttributeProcessor
}

func NewAttributeMapper() *AttributeMapper {
	return &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor:    NewCustomAttributeProcessor(),
	}
}

func (m *AttributeMapper) MapAttributeValuesToSheinIDs(ctx *sheinctx.TaskContext, strategy *AttributeStrategy) ([]attribute.CustomAttributeRelation, error) {
	return m.MapAttributeValuesToSheinIDsWithRuntime(ctx, newMapperRuntimeInput(ctx), strategy)
}

func (m *AttributeMapper) MapAttributeValuesToSheinIDsWithRuntime(ctx *sheinctx.TaskContext, runtime *MapperRuntimeInput, strategy *AttributeStrategy) ([]attribute.CustomAttributeRelation, error) {
	if err := runtime.Validate(); err != nil {
		return nil, err
	}

	logger.GetGlobalLogger("shein/product").Info("start attribute value ID mapping")

	var allRelations []attribute.CustomAttributeRelation

	relations, err := m.mapSingleAttributeValues(ctx, runtime, &strategy.PrimaryAttribute, true)
	if err != nil {
		return nil, fmt.Errorf("failed to map primary attribute values: %w", err)
	}
	allRelations = append(allRelations, relations...)

	if strategy.SecondaryAttribute.AttrID > 0 && len(strategy.SecondaryAttribute.AttrValue) > 0 {
		relations, err := m.mapSingleAttributeValues(ctx, runtime, &strategy.SecondaryAttribute, false)
		if err != nil {
			return nil, fmt.Errorf("failed to map secondary attribute values: %w", err)
		}
		allRelations = append(allRelations, relations...)
	}

	return allRelations, nil
}

func (m *AttributeMapper) mapSingleAttributeValues(ctx *sheinctx.TaskContext, runtime *MapperRuntimeInput, attr *ResultAttribute, isRequired bool) ([]attribute.CustomAttributeRelation, error) {
	if attr.AttrID <= 0 || len(attr.AttrValue) == 0 {
		return nil, nil
	}

	var relations []attribute.CustomAttributeRelation
	platformValues := m.valueMatcher.GetPlatformAttributeValues(attr.AttrID, runtime.AttributeTemplates)

	for i := 0; i < len(attr.AttrValue); i++ {
		attrValue := &attr.AttrValue[i]
		if attrValue.ID.Int() > 0 {
			continue
		}

		if platformID := m.valueMatcher.FindMatchingPlatformValue(attrValue.Value, platformValues); platformID > 0 {
			attr.AttrValue[i].ID = types.FlexibleID(platformID)
			continue
		}

		result := m.processor.ProcessCustomAttributeValueWithRuntime(ctx, runtime, attr.AttrID, attrValue.Value, isRequired)
		if !result.Success {
			if !result.ShouldContinue {
				return nil, fmt.Errorf("failed to create custom attribute value: %s", attrValue.Value)
			}
			continue
		}

		attr.AttrValue[i].ID = types.FlexibleID(result.NewValueID)
		relations = append(relations, result.Relations...)
	}

	return relations, nil
}
