package productimage

import "testing"

func TestBuildFaithfulEditPromptForWhiteBackgroundUsesSafeEcommerceLanguage(t *testing.T) {
	prompt := buildFaithfulEditPrompt(&FaithfulEditRequest{
		Operation: "render_white_background",
		ProductContext: &ProductContext{
			ProductType: "sneaker",
		},
	})

	if prompt == "" {
		t.Fatal("prompt is empty")
	}
	if containsInsensitive(prompt, "watermark") {
		t.Fatalf("prompt should avoid watermark wording: %q", prompt)
	}
	if containsInsensitive(prompt, "remove") {
		t.Fatalf("prompt should avoid aggressive removal wording: %q", prompt)
	}
	if !containsInsensitive(prompt, "plain white ecommerce background") {
		t.Fatalf("prompt should request plain white ecommerce background: %q", prompt)
	}
}

func TestBuildFaithfulEditPromptForSubjectExtractionUsesSafeIsolationLanguage(t *testing.T) {
	prompt := buildFaithfulEditPrompt(&FaithfulEditRequest{
		Operation: "extract_subject",
		ProductContext: &ProductContext{
			ProductType: "sneaker",
		},
	})

	if prompt == "" {
		t.Fatal("prompt is empty")
	}
	if containsInsensitive(prompt, "watermark") {
		t.Fatalf("prompt should avoid watermark wording: %q", prompt)
	}
	if containsInsensitive(prompt, "remove") {
		t.Fatalf("prompt should avoid aggressive removal wording: %q", prompt)
	}
	if !containsInsensitive(prompt, "isolate the sneaker") {
		t.Fatalf("prompt should request subject isolation: %q", prompt)
	}
}

func containsInsensitive(value string, needle string) bool {
	return len(value) >= len(needle) && (indexFold(value, needle) >= 0)
}

func indexFold(s string, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if equalFoldASCII(s[i:i+len(substr)], substr) {
			return i
		}
	}
	return -1
}

func equalFoldASCII(a string, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		aa := a[i]
		bb := b[i]
		if 'A' <= aa && aa <= 'Z' {
			aa = aa + ('a' - 'A')
		}
		if 'A' <= bb && bb <= 'Z' {
			bb = bb + ('a' - 'A')
		}
		if aa != bb {
			return false
		}
	}
	return true
}
