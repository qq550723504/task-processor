package publishing

import "testing"

func TestNormalizeParenthesesSpacing(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "spaces around words", input: "Widget(Blue)Size", want: "Widget (Blue) Size"},
		{name: "collapses whitespace", input: "  Widget   (Blue)   Size  ", want: "Widget (Blue) Size"},
		{name: "preserves punctuation adjacency", input: "Widget(Blue),Size", want: "Widget (Blue),Size"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeParenthesesSpacing(tt.input); got != tt.want {
				t.Fatalf("NormalizeParenthesesSpacing() = %q, want %q", got, tt.want)
			}
		})
	}
}
