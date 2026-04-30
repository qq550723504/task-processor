package sku_test

import (
	"testing"

	"task-processor/internal/shein/api/attribute"
	"task-processor/internal/shein/product/sku"
)

func newSKUUtils() *sku.SKUUtils {
	return sku.NewSKUUtils()
}

// ---- GetAttributeName ----

func TestSKUUtils_GetAttributeName(t *testing.T) {
	templates := []attribute.AttributeTemplate{
		{
			AttributeInfos: []attribute.AttributeInfo{
				{AttributeID: 1, AttributeNameEn: "Color"},
				{AttributeID: 2, AttributeNameEn: "Size"},
			},
		},
	}

	tests := []struct {
		name   string
		attrID int
		want   string
	}{
		{"found_first", 1, "Color"},
		{"found_second", 2, "Size"},
		{"not_found", 99, ""},
	}

	u := newSKUUtils()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := u.GetAttributeName(tc.attrID, templates)
			if got != tc.want {
				t.Errorf("GetAttributeName(%d) = %q, want %q", tc.attrID, got, tc.want)
			}
		})
	}
}

// ---- GetAttributeNameAlternatives ----

func TestSKUUtils_GetAttributeNameAlternatives(t *testing.T) {
	templates := []attribute.AttributeTemplate{
		{
			AttributeInfos: []attribute.AttributeInfo{
				{AttributeID: 1, AttributeNameEn: "Color"},
			},
		},
	}
	u := newSKUUtils()

	t.Run("found_returns_lower_and_upper", func(t *testing.T) {
		alts := u.GetAttributeNameAlternatives(1, templates)
		if len(alts) != 2 {
			t.Fatalf("expected 2 alternatives, got %d", len(alts))
		}
		if alts[0] != "color" || alts[1] != "COLOR" {
			t.Errorf("unexpected alternatives: %v", alts)
		}
	})

	t.Run("not_found_returns_empty", func(t *testing.T) {
		alts := u.GetAttributeNameAlternatives(99, templates)
		if len(alts) != 0 {
			t.Errorf("expected empty, got %v", alts)
		}
	})
}

// ---- ParseWeight ----

func TestSKUUtils_ParseWeight(t *testing.T) {
	u := newSKUUtils()

	tests := []struct {
		name  string
		input string
		want  float64
	}{
		{"empty_string", "", 0},
		{"grams", "100g", 100},
		{"kilograms", "2.5kg", 2500},
		{"pounds", "3lb", 1360.78},
		{"ounces", "16oz", 453.59},
		{"milligrams", "250mg", 0.25},
		{"plain_number", "50", 50},
		{"with_spaces", "  75  ", 75},
		{"invalid", "abc", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := u.ParseWeight(tc.input)
			if got != tc.want {
				t.Errorf("ParseWeight(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestSKUUtils_NormalizeWeightForShein(t *testing.T) {
	u := newSKUUtils()

	tests := []struct {
		name  string
		input float64
		want  float64
	}{
		{"zero_defaults_to_min", 0, 0.01},
		{"negative_defaults_to_min", -1, 0.01},
		{"tiny_clamped_up", 0.001, 0.01},
		{"valid_kept", 350, 350},
		{"too_large_clamped_down", 50000001, 50000000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := u.NormalizeWeightForShein(tc.input)
			if got != tc.want {
				t.Errorf("NormalizeWeightForShein(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ---- FormatPriceByCurrency ----

func TestSKUUtils_FormatPriceByCurrency(t *testing.T) {
	u := newSKUUtils()

	tests := []struct {
		name     string
		price    float64
		currency string
		want     float64
	}{
		{"JPY_truncates_decimal", 1234.56, "JPY", 1234},
		{"KRW_truncates_decimal", 9999.99, "KRW", 9999},
		{"USD_keeps_decimal", 19.99, "USD", 19.99},
		{"EUR_keeps_decimal", 9.50, "EUR", 9.50},
		{"empty_currency_keeps_decimal", 5.55, "", 5.55},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := u.FormatPriceByCurrency(tc.price, tc.currency)
			if got != tc.want {
				t.Errorf("FormatPriceByCurrency(%v, %q) = %v, want %v", tc.price, tc.currency, got, tc.want)
			}
		})
	}
}

// ---- CorrectQuantityTypeAndValue ----

func TestSKUUtils_CorrectQuantityTypeAndValue(t *testing.T) {
	u := newSKUUtils()

	tests := []struct {
		name         string
		quantityType int
		quantity     int
		wantType     int
		wantQuantity int
	}{
		// 合法组合，无需修正
		{"valid_single_item", 1, 1, 1, 1},
		{"valid_multi_same", 2, 3, 2, 3},
		{"valid_single_set", 3, 1, 3, 1},
		{"valid_multi_set", 4, 2, 4, 2},
		// 修正策略1：同款多件但数量=1 → 单品
		{"fix_multi_same_qty1", 2, 1, 1, 1},
		// 修正策略2：多套但数量=1 → 单套
		{"fix_multi_set_qty1", 4, 1, 3, 1},
		// 修正策略3：单品但数量>1 → 同款多件
		{"fix_single_item_qty_gt1", 1, 5, 2, 5},
		// 修正策略3：单套但数量>1 → 多套
		{"fix_single_set_qty_gt1", 3, 3, 4, 3},
		// 修正策略4：同款多件但数量<2 → 强制数量=2
		{"fix_multi_same_qty0", 2, 0, 2, 2},
		// 修正策略4：多套但数量<2 → 强制数量=2
		{"fix_multi_set_qty0", 4, 0, 4, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotQty := u.CorrectQuantityTypeAndValue(tc.quantityType, tc.quantity, "TEST-ASIN")
			if gotType != tc.wantType || gotQty != tc.wantQuantity {
				t.Errorf("CorrectQuantityTypeAndValue(%d, %d) = (%d, %d), want (%d, %d)",
					tc.quantityType, tc.quantity, gotType, gotQty, tc.wantType, tc.wantQuantity)
			}
		})
	}
}
