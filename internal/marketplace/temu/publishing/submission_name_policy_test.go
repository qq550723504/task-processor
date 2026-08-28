package publishing

import "testing"

func TestNormalizeProductSubmissionName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "parentheses and whitespace", input: "  Widget(Blue)   Size  ", want: "Widget (Blue) Size"},
		{name: "comma and punctuation spacing", input: "Widget(Blue) , Size . Next", want: "Widget (Blue), Size. Next"},
		{name: "unicode whitespace", input: "  Widget\u00a0(Blue)\u00a0,\u00a0Size  ", want: "Widget (Blue), Size"},
		{name: "already normalized", input: "Widget (Blue), Size", want: "Widget (Blue), Size"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeProductSubmissionName(tt.input); got != tt.want {
				t.Fatalf("NormalizeProductSubmissionName() = %q, want %q", got, tt.want)
			}
		})
	}
}
