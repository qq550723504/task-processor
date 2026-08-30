package publishing

import "testing"

func TestRemoveBrandFromTextRemovesBrandVariantsAndNormalizesSubmissionFormatting(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		brand string
		want  string
	}{
		{
			name:  "original casing",
			text:  "Acme Widget(Blue) , Size . Next",
			brand: "Acme",
			want:  "Widget (Blue), Size. Next",
		},
		{
			name:  "uppercase possessive",
			text:  "ACME's Widget",
			brand: "Acme",
			want:  "Widget",
		},
		{
			name:  "unicode whitespace",
			text:  "Acme\u00a0Widget\u00a0(Blue)\u00a0,\u00a0Size",
			brand: "Acme",
			want:  "Widget (Blue), Size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RemoveBrandFromText(tt.text, tt.brand); got != tt.want {
				t.Fatalf("RemoveBrandFromText(%q, %q) = %q, want %q", tt.text, tt.brand, got, tt.want)
			}
		})
	}
}

func TestRemoveBrandFromTextReturnsInputWhenBrandIsEmpty(t *testing.T) {
	if got := RemoveBrandFromText("Widget(Blue)", ""); got != "Widget(Blue)" {
		t.Fatalf("RemoveBrandFromText() = %q, want input unchanged", got)
	}
}
