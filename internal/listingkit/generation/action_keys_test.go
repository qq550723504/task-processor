package generation

import "testing"

func TestAllowedActionKeysContainsGenerateMissingAssets(t *testing.T) {
	for _, actionKey := range AllowedActionKeys() {
		if actionKey == ActionGenerateMissingAssets {
			return
		}
	}
	t.Fatalf("AllowedActionKeys() does not include %q", ActionGenerateMissingAssets)
}

func TestIsAllowedActionKeyNormalizesInput(t *testing.T) {
	if !IsAllowedActionKey("  GENERATE_MISSING_ASSETS  ") {
		t.Fatalf("IsAllowedActionKey() should trim spaces and ignore case")
	}
	if IsAllowedActionKey("unknown_action") {
		t.Fatalf("IsAllowedActionKey() accepted an unknown action")
	}
	if IsAllowedActionKey("   ") {
		t.Fatalf("IsAllowedActionKey() accepted blank input")
	}
}
