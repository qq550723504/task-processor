package attribute_test

import (
	"testing"

	"task-processor/internal/shein/product/attribute"
)

func newMatcher() *attribute.AttributeValueMatcher {
	return attribute.NewAttributeValueMatcher()
}

func TestAttributeValueMatcher_FindMatchingPlatformValue(t *testing.T) {
	m := newMatcher()

	platformValues := map[string]int{
		"Black":  101,
		"White":  102,
		"red":    103,
		"YELLOW": 104,
	}

	tests := []struct {
		name   string
		value  string
		wantID int
	}{
		{"exact_match", "Black", 101},
		{"case_insensitive_lower", "black", 101},
		{"case_insensitive_upper", "BLACK", 101},
		{"lowercase_key_exact", "red", 103},
		{"lowercase_key_upper", "RED", 103},
		{"uppercase_key_lower", "yellow", 104},
		{"not_found", "Green", 0},
		{"empty_value", "", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.FindMatchingPlatformValue(tc.value, platformValues)
			if got != tc.wantID {
				t.Errorf("FindMatchingPlatformValue(%q) = %d, want %d", tc.value, got, tc.wantID)
			}
		})
	}
}

func TestAttributeValueMatcher_FindMatchingPlatformValue_EmptyMap(t *testing.T) {
	m := newMatcher()

	got := m.FindMatchingPlatformValue("Black", map[string]int{})
	if got != 0 {
		t.Errorf("expected 0 for empty map, got %d", got)
	}
}

func TestAttributeValueMatcher_GetPlatformAttributeValues_NilTemplates(t *testing.T) {
	m := newMatcher()

	result := m.GetPlatformAttributeValues(1, nil)
	if len(result) != 0 {
		t.Errorf("expected empty map for nil templates, got %d entries", len(result))
	}
}
