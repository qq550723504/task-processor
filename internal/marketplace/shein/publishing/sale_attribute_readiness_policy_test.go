package publishing

import "testing"

func TestSaleDimensionMatchesNormalizesCommonLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		left  string
		right string
		want  bool
	}{
		{"颜色分类", "Color", true},
		{"colour", "color", true},
		{"尺码", "size", true},
		{"件数", "quantity", true},
		{"style type", "款式", true},
		{"size", "color", false},
		{"", "color", false},
	}
	for _, tt := range tests {
		if got := SaleDimensionMatches(tt.left, tt.right); got != tt.want {
			t.Fatalf("SaleDimensionMatches(%q, %q) = %v, want %v", tt.left, tt.right, got, tt.want)
		}
	}
}

func TestResolvedSaleAttributeValueReady(t *testing.T) {
	t.Parallel()

	valueID := 10
	if !ResolvedSaleAttributeValueReady(1001, &valueID) {
		t.Fatal("ResolvedSaleAttributeValueReady(valid) = false, want true")
	}
	zero := 0
	if ResolvedSaleAttributeValueReady(1001, &zero) {
		t.Fatal("ResolvedSaleAttributeValueReady(zero value id) = true, want false")
	}
	if ResolvedSaleAttributeValueReady(0, &valueID) {
		t.Fatal("ResolvedSaleAttributeValueReady(zero attribute id) = true, want false")
	}
	if ResolvedSaleAttributeValueReady(1001, nil) {
		t.Fatal("ResolvedSaleAttributeValueReady(nil value id) = true, want false")
	}
}
