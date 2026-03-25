package skc

import (
	"fmt"
	"strings"

	"task-processor/internal/core/logger"
	sheinattr "task-processor/internal/shein/product/attribute"
)

type SKCValidationUtils struct{}

func NewSKCValidationUtils() *SKCValidationUtils {
	return &SKCValidationUtils{}
}

func (v *SKCValidationUtils) ValidateAttributeStrategy(input *SKCValidationInput, strategy sheinattr.AttributeStrategy) error {
	if err := input.Validate(); err != nil {
		return err
	}

	var warnings []string
	if strategy.PrimaryAttribute.AttrID <= 0 {
		warnings = append(warnings, "primary attribute ID is invalid")
	} else if len(strategy.PrimaryAttribute.AttrValue) == 0 {
		warnings = append(warnings, "primary attribute values are empty")
	}

	hasSecondaryAttribute := strategy.SecondaryAttribute.AttrID > 0 && len(strategy.SecondaryAttribute.AttrValue) > 0
	if hasSecondaryAttribute {
		secondaryAttrNames := []string{"size", "Size", "dimension", "sizing"}
		if strategy.SecondaryAttribute.AttrID == 27 {
			secondaryAttrNames = []string{"color", "Color", "colour"}
		}

		matchedCount := 0
		totalValues := len(strategy.SecondaryAttribute.AttrValue)
		for _, attrValue := range strategy.SecondaryAttribute.AttrValue {
			found := false
			for _, variant := range input.StrategyData.Variants {
				for _, attrName := range secondaryAttrNames {
					if variantValue, exists := variant.Attributes[attrName]; exists && strings.EqualFold(variantValue, attrValue.Value) {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if found {
				matchedCount++
			}
		}

		validationRate := float64(matchedCount) / float64(totalValues)
		if validationRate < 0.3 {
			warnings = append(warnings, fmt.Sprintf("secondary attribute match rate is too low: %.1f%% (%d/%d)", validationRate*100, matchedCount, totalValues))
		}

		logger.GetGlobalLogger("shein/product").Infof("secondary attribute validation: attr_id=%d match_rate=%.1f%% (%d/%d)",
			strategy.SecondaryAttribute.AttrID, validationRate*100, matchedCount, totalValues)
	}

	validVariantCount := 0
	for _, variant := range input.StrategyData.Variants {
		if variant.Price > 0 && variant.ASIN != "" {
			validVariantCount++
		}
	}

	if validVariantCount == 0 {
		warnings = append(warnings, "no valid variants found")
	} else if float64(validVariantCount)/float64(len(input.StrategyData.Variants)) < 0.5 {
		warnings = append(warnings, fmt.Sprintf("valid variant ratio is too low: %.1f%% (%d/%d)",
			float64(validVariantCount)*100/float64(len(input.StrategyData.Variants)),
			validVariantCount, len(input.StrategyData.Variants)))
	}

	if len(warnings) > 0 {
		return fmt.Errorf("strategy validation warnings: %s", strings.Join(warnings, "; "))
	}

	logger.GetGlobalLogger("shein/product").Infof("attribute strategy validated: strategy=%s valid_variants=%d/%d",
		strategy.StrategyType, validVariantCount, len(input.StrategyData.Variants))
	return nil
}
