package publishing

import "testing"

func TestNormalizeSubmitWeightGramsConvertsRoundsAndClamps(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		value float64
		unit  string
		want  float64
	}{
		"kilograms": {
			value: 2,
			unit:  "kg",
			want:  2000,
		},
		"pounds rounded": {
			value: 1,
			unit:  "lb",
			want:  453.59,
		},
		"milligrams clamp to min": {
			value: 1,
			unit:  "mg",
			want:  minSubmitWeightGrams,
		},
		"zero clamp to min": {
			value: 0,
			unit:  "g",
			want:  minSubmitWeightGrams,
		},
		"max clamp": {
			value: 60000000,
			unit:  "g",
			want:  maxSubmitWeightGrams,
		},
		"unknown unit treated as grams": {
			value: 3.456,
			unit:  "stone",
			want:  3.46,
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := NormalizeSubmitWeightGrams(tt.value, tt.unit); got != tt.want {
				t.Fatalf("NormalizeSubmitWeightGrams(%v, %q) = %v, want %v", tt.value, tt.unit, got, tt.want)
			}
		})
	}
}
