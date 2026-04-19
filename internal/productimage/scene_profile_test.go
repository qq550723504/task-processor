package productimage

import "testing"

func TestRendererPresetRegistryResolvesKnownProfiles(t *testing.T) {
	t.Parallel()

	registry, err := loadRendererPresetRegistry()
	if err != nil {
		t.Fatalf("loadRendererPresetRegistry() error = %v", err)
	}

	cases := []struct {
		profile string
		group   string
	}{
		{profile: "shein_model_editorial", group: "editorial"},
		{profile: "amazon_lifestyle_scene", group: "lifestyle"},
		{profile: "shein_selling_point", group: "spec_detail"},
		{profile: "walmart_catalog_scene", group: "lifestyle"},
	}

	for _, tc := range cases {
		preset := registry.Resolve(tc.profile)
		if preset.name != tc.profile {
			t.Fatalf("preset for %q = %+v, want same name", tc.profile, preset)
		}
		if preset.group != tc.group {
			t.Fatalf("preset for %q = %+v, want group %q", tc.profile, preset, tc.group)
		}
	}
}

func TestRendererPresetRegistryFallsBackToDefaultPreset(t *testing.T) {
	t.Parallel()

	registry, err := loadRendererPresetRegistry()
	if err != nil {
		t.Fatalf("loadRendererPresetRegistry() error = %v", err)
	}

	preset := registry.Resolve("unknown_profile")
	if preset.name != "local_canvas_default" {
		t.Fatalf("preset = %+v, want local_canvas_default", preset)
	}
	if preset.group != "default" {
		t.Fatalf("preset = %+v, want default group", preset)
	}
}

func TestResolveSceneProfileUsesPresetRegistry(t *testing.T) {
	t.Parallel()

	asset := &ImageAsset{
		Metadata: map[string]string{
			"render_profile": "shein_model_editorial",
		},
	}

	profile := resolveSceneProfile(asset)
	if profile.name != "shein_model_editorial" {
		t.Fatalf("profile = %+v, want shein_model_editorial", profile)
	}
	editorialScale := profile.subjectScale

	asset.Metadata["render_profile"] = "shein_selling_point"
	specProfile := resolveSceneProfile(asset)
	if specProfile.subjectScale >= editorialScale {
		t.Fatalf("spec profile = %+v, want smaller subject scale than editorial", specProfile)
	}
	if specProfile.backgroundContrast == profile.backgroundContrast {
		t.Fatalf("spec profile = %+v, want different contrast from editorial", specProfile)
	}
}

func TestRendererPresetRegistryIncludesTemplateResourcesAndPlaceholders(t *testing.T) {
	t.Parallel()

	registry, err := loadRendererPresetRegistry()
	if err != nil {
		t.Fatalf("loadRendererPresetRegistry() error = %v", err)
	}

	preset := registry.Resolve("shein_selling_point")
	if preset.backgroundTemplate == "" || preset.overlayTemplate == "" || preset.visualMode == "" {
		t.Fatalf("preset = %+v, want template resources", preset)
	}
	if len(preset.copySlots) == 0 || len(preset.badgeSlots) == 0 {
		t.Fatalf("preset = %+v, want placeholder metadata", preset)
	}
	if preset.maxCopyLines == 0 || preset.maxBadges == 0 || preset.measurementMode == "" || preset.detailAnchorMode == "" {
		t.Fatalf("preset = %+v, want slot constraint metadata", preset)
	}
}

func TestApplyScenePresetMetadataAddsSellingPointSlotPlan(t *testing.T) {
	t.Parallel()

	metadata := ApplyScenePresetMetadata(map[string]string{}, "shein_selling_point")
	if metadata["layout_slots.copy"] == "" || metadata["layout_slots.badges"] == "" {
		t.Fatalf("metadata = %+v, want selling-point slot plan", metadata)
	}
	if metadata["layout_constraints.max_copy_lines"] == "" || metadata["layout_constraints.max_badges"] == "" {
		t.Fatalf("metadata = %+v, want selling-point constraint plan", metadata)
	}
	if metadata["layout_engine_version"] != "v1" || metadata["slot_plan_version"] != "v1" {
		t.Fatalf("metadata = %+v, want plan versions", metadata)
	}
}

func TestRendererPresetRegistryFallsBackToDefaultConstraints(t *testing.T) {
	t.Parallel()

	registry, err := loadRendererPresetRegistry()
	if err != nil {
		t.Fatalf("loadRendererPresetRegistry() error = %v", err)
	}

	preset := registry.Resolve("unknown_profile")
	if preset.maxCopyLines == 0 || preset.maxBadges == 0 || preset.measurementMode == "" || preset.detailAnchorMode == "" {
		t.Fatalf("preset = %+v, want default constraint metadata", preset)
	}
}
