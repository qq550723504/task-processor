package productimage

import "testing"

func TestResolveScenePromptOptionsUsesPlatformCategoryPresetWhenNoExplicitOptions(t *testing.T) {
	options := resolveScenePromptOptions(nil, &ProductContext{
		ProductType: "running sneaker",
		Attributes: map[string]string{
			"marketplace": "amazon",
		},
	})

	if options.Category != "shoes" {
		t.Fatalf("category = %q", options.Category)
	}
	if options.SceneStyle != "studio" ||
		options.BackgroundTone != "bright" ||
		options.Composition != "centered" ||
		options.PropsLevel != "none" ||
		options.AudienceHint != "premium" {
		t.Fatalf("options = %+v", options)
	}
	if options.DefaultsSource != "platform_category" {
		t.Fatalf("defaults source = %q", options.DefaultsSource)
	}
}

func TestResolveScenePromptOptionsPrefersExplicitValuesOverPreset(t *testing.T) {
	options := resolveScenePromptOptions(&SceneGenerationRequest{
		SceneStyle: "lifestyle",
	}, &ProductContext{
		ProductType: "running sneaker",
		Attributes: map[string]string{
			"marketplace": "amazon",
		},
	})

	if options.SceneStyle != "lifestyle" {
		t.Fatalf("expected explicit scene style, got %+v", options)
	}
	if options.BackgroundTone != "bright" ||
		options.Composition != "centered" ||
		options.PropsLevel != "none" ||
		options.AudienceHint != "premium" {
		t.Fatalf("expected preset defaults to fill non-explicit fields, got %+v", options)
	}
	if options.DefaultsSource != "explicit" {
		t.Fatalf("defaults source = %q", options.DefaultsSource)
	}
}

func TestResolveScenePromptOptionsFallsBackToPlatformPresetWhenCategoryUnknown(t *testing.T) {
	options := resolveScenePromptOptions(nil, &ProductContext{
		ProductType: "desk lamp",
		Attributes: map[string]string{
			"marketplace": "walmart",
		},
	})

	if options.Category != "" {
		t.Fatalf("category = %q", options.Category)
	}
	if options.SceneStyle != "lifestyle" ||
		options.BackgroundTone != "neutral" ||
		options.Composition != "centered" ||
		options.PropsLevel != "light" ||
		options.AudienceHint != "homey" {
		t.Fatalf("options = %+v", options)
	}
	if options.DefaultsSource != "platform" {
		t.Fatalf("defaults source = %q", options.DefaultsSource)
	}
}
