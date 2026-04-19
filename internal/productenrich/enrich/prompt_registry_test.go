package enrich_test

import (
	"context"
	"testing"

	productenrich "task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	"task-processor/internal/prompt"

	"github.com/sirupsen/logrus"
)

type productenrichPromptRegistryStub struct {
	templates map[string]string
}

func (s *productenrichPromptRegistryStub) Get(key string, fallback string) string {
	if value, ok := s.templates[key]; ok {
		return value
	}
	return fallback
}

func (s *productenrichPromptRegistryStub) Render(key string, vars map[string]any, fallback string) (string, error) {
	if value, ok := s.templates[key]; ok {
		if text, ok := vars["text"].(string); ok && text != "" {
			value = replaceToken(value, "{{.text}}", text)
		}
		return value, nil
	}
	return fallback, nil
}

func (s *productenrichPromptRegistryStub) Keys() []string {
	keys := make([]string, 0, len(s.templates))
	for key := range s.templates {
		keys = append(keys, key)
	}
	return keys
}

func TestJSONGeneratorUsesRegistryPromptWhenAvailable(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &productenrichPromptRegistryStub{
		templates: map[string]string{
			"productenrich.generation.product_json": "Registry JSON prompt",
		},
	}
	t.Cleanup(func() { prompt.GlobalRegistry = previous })

	mgr := newMockLLMManager(`{"title":"Widget","category":[],"attributes":{},"selling_points":[],"seo_keywords":[],"description":"ok"}`)
	generator, err := productenrichenrich.NewJSONGenerator(logrus.New(), mgr)
	if err != nil {
		t.Fatalf("NewJSONGenerator() error = %v", err)
	}

	_, err = generator.GenerateJSON(context.Background(), &productenrich.ProductAnalysis{}, nil, false)
	if err != nil {
		t.Fatalf("GenerateJSON() error = %v", err)
	}

	if mgr.def.lastGeneratePrompt != "Registry JSON prompt" {
		t.Fatalf("prompt = %q", mgr.def.lastGeneratePrompt)
	}
}

func TestVariantGeneratorUsesRegistryPromptForSpecsWhenAvailable(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &productenrichPromptRegistryStub{
		templates: map[string]string{
			"productenrich.generation.specs": "Registry specs prompt",
		},
	}
	t.Cleanup(func() { prompt.GlobalRegistry = previous })

	mgr := newMockLLMManager(`{"technical":{}}`)
	generator, err := productenrichenrich.NewVariantGenerator(mgr)
	if err != nil {
		t.Fatalf("NewVariantGenerator() error = %v", err)
	}

	_, err = generator.GenerateSpecs(context.Background(), &productenrich.ProductAnalysis{})
	if err != nil {
		t.Fatalf("GenerateSpecs() error = %v", err)
	}

	if mgr.def.lastGeneratePrompt != "Registry specs prompt" {
		t.Fatalf("prompt = %q", mgr.def.lastGeneratePrompt)
	}
}

func TestVariantGeneratorUsesRegistryPromptForVariantsWhenAvailable(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &productenrichPromptRegistryStub{
		templates: map[string]string{
			"productenrich.generation.variants": "Registry variants prompt",
		},
	}
	t.Cleanup(func() { prompt.GlobalRegistry = previous })

	mgr := newMockLLMManager(`[{"sku":"DEFAULT-001","attributes":{},"is_default":true}]`)
	generator, err := productenrichenrich.NewVariantGenerator(mgr)
	if err != nil {
		t.Fatalf("NewVariantGenerator() error = %v", err)
	}

	_, err = generator.GenerateVariants(context.Background(), &productenrich.ProductAnalysis{})
	if err != nil {
		t.Fatalf("GenerateVariants() error = %v", err)
	}

	if mgr.def.lastGeneratePrompt != "Registry variants prompt" {
		t.Fatalf("prompt = %q", mgr.def.lastGeneratePrompt)
	}
}

func TestVariantGeneratorUsesRegistryPromptForDimensionsWhenAvailable(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &productenrichPromptRegistryStub{
		templates: map[string]string{
			"productenrich.generation.extract_dimensions": "Registry dimensions prompt {{.text}}",
		},
	}
	t.Cleanup(func() { prompt.GlobalRegistry = previous })

	mgr := newMockLLMManager(`{"length":20,"width":10,"height":5,"unit":"cm"}`)
	generator, err := productenrichenrich.NewVariantGenerator(mgr)
	if err != nil {
		t.Fatalf("NewVariantGenerator() error = %v", err)
	}

	_, err = generator.ExtractDimensions(context.Background(), "20x10x5 cm")
	if err != nil {
		t.Fatalf("ExtractDimensions() error = %v", err)
	}

	if mgr.def.lastGeneratePrompt != "Registry dimensions prompt 20x10x5 cm" {
		t.Fatalf("prompt = %q", mgr.def.lastGeneratePrompt)
	}
}

func TestVariantGeneratorUsesRegistryPromptForWeightWhenAvailable(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &productenrichPromptRegistryStub{
		templates: map[string]string{
			"productenrich.generation.extract_weight": "Registry weight prompt {{.text}}",
		},
	}
	t.Cleanup(func() { prompt.GlobalRegistry = previous })

	mgr := newMockLLMManager(`{"value":1.5,"unit":"kg"}`)
	generator, err := productenrichenrich.NewVariantGenerator(mgr)
	if err != nil {
		t.Fatalf("NewVariantGenerator() error = %v", err)
	}

	_, err = generator.ExtractWeight(context.Background(), "1.5kg product")
	if err != nil {
		t.Fatalf("ExtractWeight() error = %v", err)
	}

	if mgr.def.lastGeneratePrompt != "Registry weight prompt 1.5kg product" {
		t.Fatalf("prompt = %q", mgr.def.lastGeneratePrompt)
	}
}

func replaceToken(value string, token string, replacement string) string {
	for {
		idx := indexOf(value, token)
		if idx < 0 {
			return value
		}
		value = value[:idx] + replacement + value[idx+len(token):]
	}
}

func indexOf(value string, needle string) int {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
