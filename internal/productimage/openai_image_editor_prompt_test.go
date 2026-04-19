package productimage

import (
	"testing"

	"task-processor/internal/prompt"
)

type promptRegistryStub struct {
	templates map[string]string
}

func (s *promptRegistryStub) Get(key string, fallback string) string {
	if value, ok := s.templates[key]; ok {
		return value
	}
	return fallback
}

func (s *promptRegistryStub) Render(key string, vars map[string]any, fallback string) (string, error) {
	if value, ok := s.templates[key]; ok {
		if productType, ok := vars["product_type"].(string); ok && productType != "" {
			value = replaceToken(value, "{{.product_type}}", productType)
		}
		if title, ok := vars["title"].(string); ok && title != "" {
			value = replaceToken(value, "{{.title}}", title)
		}
		if operation, ok := vars["operation"].(string); ok && operation != "" {
			value = replaceToken(value, "{{.operation}}", operation)
		}
		if sceneIntent, ok := vars["scene_intent"].(string); ok && sceneIntent != "" {
			value = replaceToken(value, "{{.scene_intent}}", sceneIntent)
		}
		if summaryJSON, ok := vars["summary_json"].(string); ok && summaryJSON != "" {
			value = replaceToken(value, "{{.summary_json}}", summaryJSON)
		}
		return value, nil
	}
	return fallback, nil
}

func (s *promptRegistryStub) Keys() []string {
	keys := make([]string, 0, len(s.templates))
	for key := range s.templates {
		keys = append(keys, key)
	}
	return keys
}

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

func TestBuildFaithfulEditPromptUsesRegistryTemplateWhenAvailable(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &promptRegistryStub{
		templates: map[string]string{
			prompt.KProductImageWhiteBackgroundDefault: "Registry white background prompt for {{.product_type}}",
		},
	}
	t.Cleanup(func() {
		prompt.GlobalRegistry = previous
	})

	rendered := buildFaithfulEditPrompt(&FaithfulEditRequest{
		Operation: "render_white_background",
		ProductContext: &ProductContext{
			ProductType: "sneaker",
		},
	})

	if rendered != "Registry white background prompt for sneaker" {
		t.Fatalf("prompt = %q", rendered)
	}
}

func TestBuildFaithfulEditPromptNormalizesLegacyPromptRef(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &promptRegistryStub{
		templates: map[string]string{
			prompt.KProductImageWhiteBackgroundDefault: "Registry white background prompt for {{.product_type}}",
		},
	}
	t.Cleanup(func() {
		prompt.GlobalRegistry = previous
	})

	rendered := buildFaithfulEditPrompt(&FaithfulEditRequest{
		Operation: "render_white_background",
		PromptRef: "productimage/white-background/default",
		ProductContext: &ProductContext{
			ProductType: "sneaker",
		},
	})

	if rendered != "Registry white background prompt for sneaker" {
		t.Fatalf("prompt = %q", rendered)
	}
}

func TestNormalizeProductImagePromptKeyUsesDefaultWhenBlank(t *testing.T) {
	if got := normalizeProductImagePromptKey("", "productimage.white_background.default"); got != "productimage.white_background.default" {
		t.Fatalf("normalizeProductImagePromptKey() = %q", got)
	}
}

func replaceToken(value string, token string, replacement string) string {
	for {
		next := indexFold(value, token)
		if next < 0 {
			return value
		}
		value = value[:next] + replacement + value[next+len(token):]
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
