package publishing

import (
	"strings"
	"testing"
)

func TestSanitizeProductDescriptionNormalizesWhitespaceAndRemovesNonASCIIRunes(t *testing.T) {
	result := SanitizeProductDescription("  Durable\t商品™\nCable  ")

	if result.Description != "Durable  Cable" {
		t.Fatalf("description = %q, want %q", result.Description, "Durable  Cable")
	}
	if len(result.Violations) != 0 {
		t.Fatalf("violations = %v, want none", result.Violations)
	}
}

func TestSanitizeProductDescriptionTruncatesAtTheConfiguredLimit(t *testing.T) {
	description := strings.Repeat("a", 10_000) + " tail"

	result := SanitizeProductDescription(description)

	if len(result.Description) != 10_000 {
		t.Fatalf("description length = %d, want 10000", len(result.Description))
	}
	if !strings.HasSuffix(result.Description, "...") {
		t.Fatalf("description = %q, want ellipsis suffix", result.Description[len(result.Description)-20:])
	}
	if len(result.Violations) != 1 || result.Violations[0] != "描述长度超过限制: 10005 > 10000字符" {
		t.Fatalf("violations = %v, want the length violation", result.Violations)
	}
}
