package mathx

import "testing"

func TestAbs(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{5.5, 5.5},
		{-5.5, 5.5},
		{0, 0},
		{-0.001, 0.001},
	}

	for _, tt := range tests {
		result := Abs(tt.input)
		if result != tt.expected {
			t.Errorf("Abs(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestAbsInt(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{5, 5},
		{-5, 5},
		{0, 0},
		{-100, 100},
	}

	for _, tt := range tests {
		result := AbsInt(tt.input)
		if result != tt.expected {
			t.Errorf("AbsInt(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		a, b     int
		expected int
	}{
		{5, 10, 5},
		{10, 5, 5},
		{5, 5, 5},
		{-5, 5, -5},
	}

	for _, tt := range tests {
		result := Min(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("Min(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
		}
	}
}
