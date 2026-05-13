package productenrich

import (
	"context"
	"testing"
)

func TestIsMockLLMEnabled(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"":      false,
		"0":     false,
		"false": false,
		"1":     true,
		"true":  true,
		"TRUE":  true,
		"yes":   true,
		"on":    true,
	}

	for input, want := range cases {
		if got := IsMockLLMEnabled(input); got != want {
			t.Fatalf("IsMockLLMEnabled(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestLocalMockLLMManager_DefaultClient(t *testing.T) {
	t.Parallel()

	mgr := NewLocalMockLLMManager()
	if err := ValidateMockLLMManager(mgr); err != nil {
		t.Fatalf("ValidateMockLLMManager() error = %v", err)
	}

	client, err := mgr.GetClient("default")
	if err != nil {
		t.Fatalf("GetClient(default) error = %v", err)
	}

	resp, err := client.Generate(context.Background(), "Generate a complete product JSON")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp == "" {
		t.Fatal("Generate() returned empty response")
	}

	imageResp, err := client.AnalyzeImage(context.Background(), "https://example.com/image.png", "Analyze this product image")
	if err != nil {
		t.Fatalf("AnalyzeImage() error = %v", err)
	}
	if imageResp == "" {
		t.Fatal("AnalyzeImage() returned empty response")
	}
}
