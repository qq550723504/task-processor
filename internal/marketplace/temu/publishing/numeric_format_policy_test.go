package publishing

import "testing"

func TestFormatWeightUsesTEMUBoundsAndPrecision(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "blank defaults", input: "", want: "0.22"},
		{name: "unit is removed", input: "2.345 lb", want: "2.35"},
		{name: "upper bound is enforced", input: "1000.5", want: "999.99"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatWeight(tt.input); got != tt.want {
				t.Fatalf("FormatWeight(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatDimensionUsesTEMUBoundsAndPrecision(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "blank defaults", input: "", want: "3.9"},
		{name: "unit is removed", input: "15.67 in", want: "15.7"},
		{name: "non-positive defaults", input: "0", want: "3.9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatDimension(tt.input); got != tt.want {
				t.Fatalf("FormatDimension(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
