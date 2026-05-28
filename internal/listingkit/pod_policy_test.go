package listingkit

import "testing"

func TestDeterminePODExecutionPolicyRequiresSDSBackedTasks(t *testing.T) {
	t.Parallel()

	policy := determinePODExecutionPolicy(&GenerateRequest{
		Platforms: []string{"shein"},
		Options: &GenerateOptions{
			SDS: &SDSSyncOptions{
				VariantID: 901,
			},
		},
	})

	if policy.Provider != podProviderSDS {
		t.Fatalf("provider = %q, want %q", policy.Provider, podProviderSDS)
	}
	if policy.DependencyMode != podDependencyModeRequired {
		t.Fatalf("dependency mode = %q, want %q", policy.DependencyMode, podDependencyModeRequired)
	}
	if policy.DecisionSource != "system_rule" {
		t.Fatalf("decision source = %q, want system_rule", policy.DecisionSource)
	}
}

func TestDeterminePODExecutionPolicyDisablesNonPODTasks(t *testing.T) {
	t.Parallel()

	policy := determinePODExecutionPolicy(&GenerateRequest{
		Platforms: []string{"shein"},
		Options:   &GenerateOptions{},
	})

	if policy.Provider != "" {
		t.Fatalf("provider = %q, want empty", policy.Provider)
	}
	if policy.DependencyMode != podDependencyModeDisabled {
		t.Fatalf("dependency mode = %q, want %q", policy.DependencyMode, podDependencyModeDisabled)
	}
	if policy.DecisionSource != "system_rule" {
		t.Fatalf("decision source = %q, want system_rule", policy.DecisionSource)
	}
}

func TestDeterminePODExecutionPolicyAllowsAIGeneratedSizeImageFallbackAsOptional(t *testing.T) {
	t.Parallel()

	policy := determinePODExecutionPolicy(&GenerateRequest{
		Platforms: []string{"shein"},
		ImageURLs: []string{"https://cdn.example.com/source.png"},
		Options: &GenerateOptions{
			ProcessImages: false,
			ImageStrategy: sheinImageStrategyAIGenerated,
			SheinStudio: &SheinStudioOptions{
				RenderSizeImagesWithSDS: true,
			},
			SDS: &SDSSyncOptions{
				VariantID: 901,
			},
		},
	})

	if policy.Provider != podProviderSDS {
		t.Fatalf("provider = %q, want %q", policy.Provider, podProviderSDS)
	}
	if policy.DependencyMode != podDependencyModeOptional {
		t.Fatalf("dependency mode = %q, want %q", policy.DependencyMode, podDependencyModeOptional)
	}
}
