package skc_test

import (
	"strings"
	"testing"

	sheinattr "task-processor/internal/shein/product/attribute"
	"task-processor/internal/shein/product/skc"
)

// newUtils 创建 SKCValidationUtils，taskContext 在 ValidateAttributeStrategy 中未被使用，传 nil 即可
func newUtils() *skc.SKCValidationUtils {
	return skc.NewSKCValidationUtils(nil)
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

	if err := utils.ValidateAttributeStrategy(strategy, saleAttr); err != nil {
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

	err := utils.ValidateAttributeStrategy(strategy, saleAttr)
	if err == nil {
		t.Fatal("expected error for invalid primary attr ID, got nil")
	}
	if !strings.Contains(err.Error(), "主要属性ID无效") {
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

	err := utils.ValidateAttributeStrategy(strategy, saleAttr)
	if err == nil {
		t.Fatal("expected error for empty primary attr value, got nil")
	}
	if !strings.Contains(err.Error(), "主要属性值为空") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSKCValidationUtils_ValidateAttributeStrategy_NoValidVariants(t *testing.T) {
	utils := newUtils()

	strategy := makeStrategy(1, []string{"Red"}, 0, nil)
	// 变体 price=0 且 ASIN 为空，均无效
	saleAttr := sheinattr.ResultSaleAttribute{
		Variants: makeVariants(
			[]map[string]string{{"color": "Red"}},
			[]float64{0},
			[]string{""},
		),
	}

	err := utils.ValidateAttributeStrategy(strategy, saleAttr)
	if err == nil {
		t.Fatal("expected error for no valid variants, got nil")
	}
	if !strings.Contains(err.Error(), "没有有效的变体数据") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSKCValidationUtils_ValidateAttributeStrategy_LowValidVariantRatio(t *testing.T) {
	utils := newUtils()

	strategy := makeStrategy(1, []string{"Red"}, 0, nil)
	// 4个变体只有1个有效，比例 25% < 50%
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

	err := utils.ValidateAttributeStrategy(strategy, saleAttr)
	if err == nil {
		t.Fatal("expected error for low valid variant ratio, got nil")
	}
	if !strings.Contains(err.Error(), "有效变体比例过低") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSKCValidationUtils_ValidateAttributeStrategy_SecondaryAttrLowMatchRate(t *testing.T) {
	utils := newUtils()

	// 次要属性有5个值，但变体中只能匹配0个 → 匹配率0% < 30%
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

	err := utils.ValidateAttributeStrategy(strategy, saleAttr)
	if err == nil {
		t.Fatal("expected error for low secondary attr match rate, got nil")
	}
	if !strings.Contains(err.Error(), "匹配率过低") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSKCValidationUtils_ValidateAttributeStrategy_SecondaryAttrHighMatchRate(t *testing.T) {
	utils := newUtils()

	// 次要属性3个值，变体中能匹配2个 → 匹配率 66% > 30%，通过
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

	if err := utils.ValidateAttributeStrategy(strategy, saleAttr); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}
