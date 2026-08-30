package product

import "testing"

func TestProductNameValidatorCleanSpacesNormalizesSubmissionFormatting(t *testing.T) {
	validator := &ProductNameValidator{}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "parentheses and whitespace", input: "  Widget(Blue)   Size  ", want: "Widget (Blue) Size"},
		{name: "comma and punctuation spacing", input: "Widget(Blue) , Size . Next", want: "Widget (Blue), Size. Next"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validator.cleanSpaces(tt.input); got != tt.want {
				t.Fatalf("cleanSpaces() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProductNameValidatorNormalizesOptimizedNameWithSubmissionPolicy(t *testing.T) {
	validator := &ProductNameValidator{}

	got := validator.normalizeOptimizedName("Widget(Blue) , Size")
	want := "Widget (Blue), Size"

	if got != want {
		t.Fatalf("normalizeOptimizedName() = %q, want %q", got, want)
	}
}

func TestProductNameValidatorDelegatesSanitizationToPublishingPolicy(t *testing.T) {
	validator := &ProductNameValidator{}

	got, violations := validator.validateAndCleanProductName("Widget® 中文!")
	if got != "Widget (R)" {
		t.Fatalf("validateAndCleanProductName() = %q, want canonical sanitized name", got)
	}
	if len(violations) != 3 {
		t.Fatalf("violations = %#v, want decorative/high-ascii/Chinese violations", violations)
	}
}
