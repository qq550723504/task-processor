package publishing

import "testing"

func TestSanitizeProductNameRemovesUnsupportedContentAndNormalizesSpacing(t *testing.T) {
	result := SanitizeProductName("  Widget® 中文! _ (Blue) , Size . Next  ")
	if result.Name != "Widget (R) (Blue), Size. Next" {
		t.Fatalf("Name = %q, want normalized sanitized name", result.Name)
	}
	if len(result.Violations) != 4 {
		t.Fatalf("Violations = %#v, want two decorative plus high-ascii/Chinese violations", result.Violations)
	}
}

func TestSanitizeProductNamePreservesAllowedPunctuation(t *testing.T) {
	result := SanitizeProductName(`Shoe - Sport + Size = 10 (Red) [US] / Men's 50%`)
	if result.Name != `Shoe - Sport + Size = 10 (Red) [US] / Men's 50%` {
		t.Fatalf("Name = %q, want allowed punctuation preserved", result.Name)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("Violations = %#v, want none", result.Violations)
	}
}

func TestSanitizeProductNameConvertsTrademarkAndDropsOtherHighASCII(t *testing.T) {
	result := SanitizeProductName("Basic™ Café")
	if result.Name != "Basic (TM) Caf" {
		t.Fatalf("Name = %q, want trademark conversion and unsupported rune removal", result.Name)
	}
	if len(result.Violations) != 1 || result.Violations[0] != "包含高ASCII字符（已转换或移除）" {
		t.Fatalf("Violations = %#v, want high-ASCII violation", result.Violations)
	}
}
