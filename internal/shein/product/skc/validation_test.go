package skc_test

import (
	"strings"
	"testing"

	sheinattribute "task-processor/internal/shein/api/attribute"
	sheinattr "task-processor/internal/shein/product/attribute"
	"task-processor/internal/shein/product/skc"
)

func newUtils() *skc.SKCValidationUtils {
	return skc.NewSKCValidationUtils()
}

func makeValidationInput(result sheinattr.ResultSaleAttribute) *skc.SKCValidationInput {
	return &skc.SKCValidationInput{
		StrategyData:       result,
		AttributeTemplates: &sheinattribute.AttributeTemplateInfo{Data: []sheinattribute.AttributeTemplate{{AttributeInfos: []sheinattribute.AttributeInfo{{AttributeID: 1}, {AttributeID: 27}}}}},
	}
}

func makeStrategy(primaryID int, primaryValues []string, secondaryID int, secondaryValues []string) sheinattr.AttributeStrategy {
	primary := sheinattr.ResultAttribute{AttrID: primaryID}
	for _, v := range primaryValues {
		primary.AttrValue = append(primary.AttrValue, sheinattr.AttributeValue{Value: v})
	}
	secondary := sheinattr.ResultAttribute{AttrID: secondaryID}
	for _, v := range secondaryValues {
		secondary.AttrValue = append(secondary.AttrValue, sheinattr.AttributeValue{Value: v})
	}
	return sheinattr.AttributeStrategy{
		PrimaryAttribute:   primary,
		SecondaryAttribute: secondary,
		StrategyType:       "test",
	}
}

func makeVariants(entries []map[string]string, prices []float64, asins []string) []sheinattr.Variant {
	variants := make([]sheinattr.Variant, len(entries))
	for i, attrs := range entries {
		variants[i] = sheinattr.Variant{
			Attributes: attrs,
			Price:      prices[i],
			ASIN:       asins[i],
		}
	}
	return variants
}

func TestSKCValidationUtils_ValidateAttributeStrategy_Pass(t *testing.T) {
	utils := newUtils()

	strategy := makeStrategy(1, []string{"Red"}, 0, nil)
	saleAttr := sheinattr.ResultSaleAttribute{
		Variants: makeVariants(
			[]map[string]string{{"color": "Red"}},
			[]float64{10.0},
			[]string{"ASIN001"},
		),
	}

	if err := utils.ValidateAttributeStrategy(makeValidationInput(saleAttr), strategy); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestSKCValidationUtils_ValidateAttributeStrategy_InvalidPrimaryAttrID(t *testing.T) {
	utils := newUtils()

	strategy := makeStrategy(0, []string{"Red"}, 0, nil)
	saleAttr := sheinattr.ResultSaleAttribute{
		Variants: makeVariants(
			[]map[string]string{{"color": "Red"}},
			[]float64{10.0},
			[]string{"ASIN001"},
		),
	}

	err := utils.ValidateAttributeStrategy(makeValidationInput(saleAttr), strategy)
	if err == nil {
		t.Fatal("expected error for invalid primary attr ID, got nil")
	}
	if !strings.Contains(err.Error(), "primary attribute ID is invalid") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSKCValidationUtils_ValidateAttributeStrategy_EmptyPrimaryAttrValue(t *testing.T) {
	utils := newUtils()

	strategy := makeStrategy(1, nil, 0, nil)
	saleAttr := sheinattr.ResultSaleAttribute{
		Variants: makeVariants(
			[]map[string]string{{"color": "Red"}},
			[]float64{10.0},
			[]string{"ASIN001"},
		),
	}

	err := utils.ValidateAttributeStrategy(makeValidationInput(saleAttr), strategy)
	if err == nil {
		t.Fatal("expected error for empty primary attr value, got nil")
	}
	if !strings.Contains(err.Error(), "primary attribute values are empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSKCValidationUtils_ValidateAttributeStrategy_NoValidVariants(t *testing.T) {
	utils := newUtils()

	strategy := makeStrategy(1, []string{"Red"}, 0, nil)
	saleAttr := sheinattr.ResultSaleAttribute{
		Variants: makeVariants(
			[]map[string]string{{"color": "Red"}},
			[]float64{0},
			[]string{""},
		),
	}

	err := utils.ValidateAttributeStrategy(makeValidationInput(saleAttr), strategy)
	if err == nil {
		t.Fatal("expected error for no valid variants, got nil")
	}
	if !strings.Contains(err.Error(), "no valid variants found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSKCValidationUtils_ValidateAttributeStrategy_LowValidVariantRatio(t *testing.T) {
	utils := newUtils()

	strategy := makeStrategy(1, []string{"Red"}, 0, nil)
	saleAttr := sheinattr.ResultSaleAttribute{
		Variants: makeVariants(
			[]map[string]string{
				{"color": "Red"},
				{"color": "Blue"},
				{"color": "Green"},
				{"color": "Black"},
			},
			[]float64{10.0, 0, 0, 0},
			[]string{"ASIN001", "", "", ""},
		),
	}

	err := utils.ValidateAttributeStrategy(makeValidationInput(saleAttr), strategy)
	if err == nil {
		t.Fatal("expected error for low valid variant ratio, got nil")
	}
	if !strings.Contains(err.Error(), "valid variant ratio is too low") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSKCValidationUtils_ValidateAttributeStrategy_SecondaryAttrLowMatchRate(t *testing.T) {
	utils := newUtils()

	strategy := makeStrategy(1, []string{"Red"}, 1, []string{"S", "M", "L", "XL", "XXL"})
	saleAttr := sheinattr.ResultSaleAttribute{
		Variants: makeVariants(
			[]map[string]string{
				{"size": "A"},
				{"size": "B"},
			},
			[]float64{10.0, 10.0},
			[]string{"ASIN001", "ASIN002"},
		),
	}

	err := utils.ValidateAttributeStrategy(makeValidationInput(saleAttr), strategy)
	if err == nil {
		t.Fatal("expected error for low secondary attr match rate, got nil")
	}
	if !strings.Contains(err.Error(), "secondary attribute match rate is too low") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSKCValidationUtils_ValidateAttributeStrategy_SecondaryAttrHighMatchRate(t *testing.T) {
	utils := newUtils()

	strategy := makeStrategy(1, []string{"Red"}, 1, []string{"S", "M", "L"})
	saleAttr := sheinattr.ResultSaleAttribute{
		Variants: makeVariants(
			[]map[string]string{
				{"size": "S"},
				{"size": "M"},
				{"size": "XL"},
			},
			[]float64{10.0, 10.0, 10.0},
			[]string{"ASIN001", "ASIN002", "ASIN003"},
		),
	}

	if err := utils.ValidateAttributeStrategy(makeValidationInput(saleAttr), strategy); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}
