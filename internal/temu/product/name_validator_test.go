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
