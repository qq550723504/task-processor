package attribute

import (
	"testing"
)

func newRepairHandler() *ValidateRepairSaleAttributeHandler {
	return NewValidateRepairSaleAttributeHandler()
}

func TestValidateRepairSaleAttributeHandler_findExactMatch(t *testing.T) {
	h := newRepairHandler()

	platformValues := map[string]int{
		"Black":  101,
		"White":  102,
		"Red":    103,
		"blue":   104,
		"YELLOW": 105,
	}

	tests := []struct {
		name      string
		value     string
		wantID    int
		wantFound bool
	}{
		{"exact_match", "Black", 101, true},
		{"case_insensitive_lower", "black", 101, true},
		{"case_insensitive_upper", "BLACK", 101, true},
		{"case_insensitive_mixed", "bLaCk", 101, true},
		{"exact_match_lowercase_key", "blue", 104, true},
		{"case_insensitive_uppercase_key", "YELLOW", 105, true},
		{"case_insensitive_mixed_key", "Yellow", 105, true},
		{"not_found", "Green", 0, false},
		{"empty_value", "", 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotID, gotFound := h.findExactMatch(tc.value, platformValues)
			if gotFound != tc.wantFound {
				t.Errorf("findExactMatch(%q) found = %v, want %v", tc.value, gotFound, tc.wantFound)
			}
			if gotFound && gotID != tc.wantID {
				t.Errorf("findExactMatch(%q) id = %d, want %d", tc.value, gotID, tc.wantID)
			}
		})
	}
}

func TestValidateRepairSaleAttributeHandler_findFuzzyMatch(t *testing.T) {
	h := newRepairHandler()

	platformValues := map[string]int{
		"Black":  101,
		"White":  102,
		"  Red ": 103,
	}

	tests := []struct {
		name      string
		value     string
		wantID    int
		wantFound bool
	}{
		{"exact_match", "Black", 101, true},
		{"case_insensitive", "black", 101, true},
		{"trimmed_spaces_in_platform", "Red", 103, true},
		{"trimmed_spaces_in_value", "  Black  ", 101, true},
		{"not_found", "Green", 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotID, gotFound := h.findFuzzyMatch(tc.value, platformValues)
			if gotFound != tc.wantFound {
				t.Errorf("findFuzzyMatch(%q) found = %v, want %v", tc.value, gotFound, tc.wantFound)
			}
			if gotFound && gotID != tc.wantID {
				t.Errorf("findFuzzyMatch(%q) id = %d, want %d", tc.value, gotID, tc.wantID)
			}
		})
	}
}

func TestValidateRepairSaleAttributeHandler_buildPlatformAttributeValueMap(t *testing.T) {
	h := newRepairHandler()

	t.Run("nil_templates_returns_empty", func(t *testing.T) {
		result := h.buildPlatformAttributeValueMap(nil)
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d entries", len(result))
		}
	})
}
