package sale_test

import (
	"testing"

	"task-processor/internal/model"
	sheinattr "task-processor/internal/shein/product/attribute"
	"task-processor/internal/shein/product/attribute/sale"
)

func newFilter() *sale.SaleAttributeValueFilter {
	return sale.NewSaleAttributeValueFilter()
}

func makeCandidate(id int, value string) sheinattr.GenerateAttributeValue {
	return sheinattr.GenerateAttributeValue{ID: id, Value: value}
}

// ---- FilterAttributeValuesByUsage ----

func TestSaleAttributeValueFilter_FilterAttributeValuesByUsage(t *testing.T) {
	f := newFilter()

	candidates := []sheinattr.GenerateAttributeValue{
		makeCandidate(1, "Red"),
		makeCandidate(2, "Blue"),
		makeCandidate(3, "Green"),
		makeCandidate(4, "Black"),
		makeCandidate(5, "White"),
		makeCandidate(6, "Yellow"),
	}

	tests := []struct {
		name         string
		candidates   []sheinattr.GenerateAttributeValue
		actualValues []string
		wantLen      int
		wantValues   []string
	}{
		{
			name:         "matches_subset",
			candidates:   candidates,
			actualValues: []string{"Red", "Blue"},
			wantLen:      2,
			wantValues:   []string{"Red", "Blue"},
		},
		{
			name:         "no_actual_values_returns_first_5",
			candidates:   candidates,
			actualValues: []string{},
			wantLen:      5,
		},
		{
			name:         "no_match_returns_first_3",
			candidates:   candidates,
			actualValues: []string{"Purple", "Orange"},
			wantLen:      3,
		},
		{
			name:         "empty_candidates",
			candidates:   []sheinattr.GenerateAttributeValue{},
			actualValues: []string{"Red"},
			wantLen:      0,
		},
		{
			name:         "fewer_than_5_candidates_no_actual",
			candidates:   candidates[:3],
			actualValues: []string{},
			wantLen:      3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := f.FilterAttributeValuesByUsage(tc.candidates, tc.actualValues, "color")
			if len(got) != tc.wantLen {
				t.Errorf("FilterAttributeValuesByUsage len = %d, want %d", len(got), tc.wantLen)
			}
			for _, wantVal := range tc.wantValues {
				found := false
				for _, g := range got {
					if g.Value == wantVal {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected value %q in result, got %v", wantVal, got)
				}
			}
		})
	}
}

// ---- ExtractActualValuesFromVariations ----

func TestSaleAttributeValueFilter_ExtractActualValuesFromVariations(t *testing.T) {
	f := newFilter()

	variations := []model.VariationValue{
		{VariantName: "Color", Values: []string{"Red", "Blue", "Red"}}, // 重复值应去重
		{VariantName: "Size", Values: []string{"S", "M", "L"}},
	}

	tests := []struct {
		name          string
		attributeName string
		wantValues    []string
		wantLen       int
	}{
		{
			name:          "match_color",
			attributeName: "Color",
			wantValues:    []string{"Red", "Blue"},
			wantLen:       2, // 去重后
		},
		{
			name:          "match_size",
			attributeName: "Size",
			wantValues:    []string{"S", "M", "L"},
			wantLen:       3,
		},
		{
			name:          "case_insensitive",
			attributeName: "color",
			wantLen:       2,
		},
		{
			name:          "not_found",
			attributeName: "Weight",
			wantLen:       0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := f.ExtractActualValuesFromVariations(variations, tc.attributeName)
			if len(got) != tc.wantLen {
				t.Errorf("ExtractActualValuesFromVariations(%q) len = %d, want %d", tc.attributeName, len(got), tc.wantLen)
			}
			for _, wantVal := range tc.wantValues {
				found := false
				for _, g := range got {
					if g == wantVal {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected %q in result %v", wantVal, got)
				}
			}
		})
	}
}

// ---- ExtractActualValuesFromProducts ----

func TestSaleAttributeValueFilter_ExtractActualValuesFromProducts(t *testing.T) {
	f := newFilter()

	products := []sheinattr.ProductVariantData{
		{ASIN: "B001", Attributes: map[string]string{"Color": "Red", "Size": "M"}},
		{ASIN: "B002", Attributes: map[string]string{"Color": "Blue", "Size": "L"}},
		{ASIN: "B003", Attributes: map[string]string{"Color": "Red", "Size": "S"}}, // Red 重复
	}

	tests := []struct {
		name          string
		attributeName string
		wantLen       int
		wantValues    []string
	}{
		{
			name:          "extract_color_deduped",
			attributeName: "Color",
			wantLen:       2, // Red, Blue（去重）
			wantValues:    []string{"Red", "Blue"},
		},
		{
			name:          "extract_size",
			attributeName: "Size",
			wantLen:       3,
			wantValues:    []string{"M", "L", "S"},
		},
		{
			name:          "case_insensitive",
			attributeName: "color",
			wantLen:       2,
		},
		{
			name:          "not_found",
			attributeName: "Weight",
			wantLen:       0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := f.ExtractActualValuesFromProducts(products, tc.attributeName)
			if len(got) != tc.wantLen {
				t.Errorf("ExtractActualValuesFromProducts(%q) len = %d, want %d", tc.attributeName, len(got), tc.wantLen)
			}
			for _, wantVal := range tc.wantValues {
				found := false
				for _, g := range got {
					if g == wantVal {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected %q in result %v", wantVal, got)
				}
			}
		})
	}
}
