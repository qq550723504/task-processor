package sale

import (
	"testing"
)

func TestParseFloat(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  float64
	}{
		{"empty_string", "", 0},
		{"integer", "42", 42},
		{"decimal", "3.14", 3.14},
		{"with_unit_suffix", "100cm", 100},
		{"with_spaces", "  25.5  ", 25.5},
		{"non_numeric", "abc", 0},
		{"leading_number_with_text", "12.5kg", 12.5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseFloat(tc.input)
			if got != tc.want {
				t.Errorf("parseFloat(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestIsAttributeNameSimilar(t *testing.T) {
	tests := []struct {
		name  string
		name1 string
		name2 string
		want  bool
	}{
		{"exact_match", "color", "color", true},
		{"case_insensitive", "Color", "color", true},
		{"with_spaces_normalized", "color name", "colorname", true},
		{"with_underscores_normalized", "color_name", "colorname", true},
		{"with_dashes_normalized", "color-name", "colorname", true},
		{"contains_match", "color", "mycolor", true},
		{"known_mapping_colour", "color", "colour", true},
		{"known_mapping_style", "style", "stylename", true},
		{"known_mapping_scent", "scent", "scentname", true},
		{"no_match", "color", "weight", false},
		{"completely_different", "size", "brand", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isAttributeNameSimilar(tc.name1, tc.name2)
			if got != tc.want {
				t.Errorf("isAttributeNameSimilar(%q, %q) = %v, want %v", tc.name1, tc.name2, got, tc.want)
			}
		})
	}
}

func TestConvertToSet(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		wantIn  []string // 期望在集合中的值（小写）
		wantNot []string // 期望不在集合中的值
	}{
		{
			name:   "basic_values",
			input:  []string{"Red", "Blue", "Green"},
			wantIn: []string{"red", "blue", "green"},
		},
		{
			name:   "with_spaces_trimmed",
			input:  []string{"  Red  ", " Blue"},
			wantIn: []string{"red", "blue"},
		},
		{
			name:    "empty_slice",
			input:   []string{},
			wantNot: []string{"red"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := convertToSet(tc.input)
			for _, v := range tc.wantIn {
				if !got[v] {
					t.Errorf("convertToSet: expected %q in set, got %v", v, got)
				}
			}
			for _, v := range tc.wantNot {
				if got[v] {
					t.Errorf("convertToSet: expected %q NOT in set, got %v", v, got)
				}
			}
		})
	}
}

func TestIsValueSetEqual(t *testing.T) {
	tests := []struct {
		name string
		set1 map[string]bool
		set2 map[string]bool
		want bool
	}{
		{
			name: "equal_sets",
			set1: map[string]bool{"a": true, "b": true},
			set2: map[string]bool{"a": true, "b": true},
			want: true,
		},
		{
			name: "different_size",
			set1: map[string]bool{"a": true, "b": true},
			set2: map[string]bool{"a": true},
			want: false,
		},
		{
			name: "same_size_different_values",
			set1: map[string]bool{"a": true, "b": true},
			set2: map[string]bool{"a": true, "c": true},
			want: false,
		},
		{
			name: "both_empty",
			set1: map[string]bool{},
			set2: map[string]bool{},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isValueSetEqual(tc.set1, tc.set2)
			if got != tc.want {
				t.Errorf("isValueSetEqual(%v, %v) = %v, want %v", tc.set1, tc.set2, got, tc.want)
			}
		})
	}
}

func TestLooksLikeCompleteJson(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid_simple_object", `{"key": "value"}`, true},
		{"valid_nested", `{"key": {"inner": [1, 2]}}`, true},
		{"missing_closing_brace", `{"key": "value"`, false},
		{"starts_with_array", `[{"key": "value"}]`, false},
		{"empty_object", `{}`, true},
		{"empty_string", ``, false},
		{"mismatched_brackets", `{"key": [1, 2}`, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := looksLikeCompleteJson(tc.input)
			if got != tc.want {
				t.Errorf("looksLikeCompleteJson(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
