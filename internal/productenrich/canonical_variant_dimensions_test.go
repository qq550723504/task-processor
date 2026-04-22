package productenrich

import "testing"

func TestBuildCanonicalProduct_PreservesVariantDimensions(t *testing.T) {
	product := &ProductJSON{
		Title: "Sneaker",
		VariantDimensions: []ScrapedVariantDimension{
			{Name: "颜色", Values: []string{"黑灰色", "卡其色"}},
			{Name: "尺码", Values: []string{"41", "42"}},
		},
	}

	canonical := BuildCanonicalProduct(&GenerateRequest{}, product)
	if canonical == nil {
		t.Fatal("expected canonical product")
	}
	if len(canonical.VariantDimensions) != 2 {
		t.Fatalf("len(VariantDimensions) = %d, want 2", len(canonical.VariantDimensions))
	}
	if canonical.VariantDimensions[0].Name != "颜色" {
		t.Fatalf("VariantDimensions[0].Name = %q, want 颜色", canonical.VariantDimensions[0].Name)
	}
}
