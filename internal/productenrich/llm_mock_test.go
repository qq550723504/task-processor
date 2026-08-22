package productenrich

import (
	"context"
	"errors"
	"testing"
)

func TestLocalMockLLMManagerImplementsAndValidatesRoutedContract(t *testing.T) {
	validator := &mockRouteValidator{wantClient: "fast", wantRoute: LLMClientRoute{
		CredentialReference: "fast", ConfigurationVersion: "static:v1",
	}}
	mgr := NewLocalMockLLMManager(validator)
	routed, ok := mgr.(RoutedLLMManager)
	if !ok {
		t.Fatal("local mock manager does not implement RoutedLLMManager")
	}
	client, err := routed.GetClientWithRoute(context.Background(), "fast", validator.wantRoute)
	if err != nil || client == nil {
		t.Fatalf("GetClientWithRoute = (%v, %v)", client, err)
	}
	if validator.calls != 1 {
		t.Fatalf("route validator calls = %d, want 1", validator.calls)
	}
	for _, bad := range []struct {
		name   string
		client string
		route  LLMClientRoute
	}{
		{name: "blank reference", client: "fast", route: LLMClientRoute{ConfigurationVersion: "static:v1"}},
		{name: "reference mismatch", client: "fast", route: LLMClientRoute{CredentialReference: "default", ConfigurationVersion: "static:v1"}},
		{name: "blank version", client: "fast", route: LLMClientRoute{CredentialReference: "fast"}},
	} {
		t.Run(bad.name, func(t *testing.T) {
			if _, err := routed.GetClientWithRoute(context.Background(), bad.client, bad.route); err == nil {
				t.Fatal("invalid mock route was accepted")
			}
		})
	}
}

func TestValidateGovernedLLMManagerRejectsMissingRoutedSupportAtAssembly(t *testing.T) {
	if err := ValidateGovernedLLMManager(nonRoutedLLMManager{}, true); err == nil {
		t.Fatal("governed assembly accepted manager without routed support")
	}
	if err := ValidateGovernedLLMManager(nonRoutedLLMManager{}, false); err != nil {
		t.Fatalf("legacy-only manager rejected: %v", err)
	}
}

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

type mockRouteValidator struct {
	wantClient string
	wantRoute  LLMClientRoute
	calls      int
}

func (m *mockRouteValidator) GetClient(string) (LLMClient, error) { return &localMockLLMClient{}, nil }
func (m *mockRouteValidator) GetDefaultClient() LLMClient         { return &localMockLLMClient{} }
func (m *mockRouteValidator) GetClientWithRoute(_ context.Context, clientName string, route LLMClientRoute) (LLMClient, error) {
	m.calls++
	if clientName != m.wantClient || route != m.wantRoute {
		return nil, errors.New("unexpected route")
	}
	return &localMockLLMClient{}, nil
}

type nonRoutedLLMManager struct{}

func (nonRoutedLLMManager) GetClient(string) (LLMClient, error) { return &localMockLLMClient{}, nil }
func (nonRoutedLLMManager) GetDefaultClient() LLMClient         { return &localMockLLMClient{} }
