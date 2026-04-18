package productimage

import (
	"embed"
	"image/color"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed presets/*.yaml
var rendererPresetFS embed.FS

type sceneProfile struct {
	name                 string
	group                string
	blurRadius           float64
	backgroundBrightness float64
	backgroundContrast   float64
	subjectScale         float64
	cardColor            color.NRGBA
	backgroundTemplate   string
	overlayTemplate      string
	layoutVariant        string
	visualMode           string
	copySlots            []string
	badgeSlots           []string
	measurementSlots     []string
	detailAnchorSlots    []string
	maxCopyLines         int
	maxBadges            int
	measurementMode      string
	detailAnchorMode     string
}

type rendererPresetRegistry struct {
	Default string               `yaml:"default"`
	Presets []rendererPresetYAML `yaml:"presets"`

	byName map[string]sceneProfile
}

type rendererPresetYAML struct {
	Name                 string            `yaml:"name"`
	Group                string            `yaml:"group"`
	BlurRadius           float64           `yaml:"blur_radius"`
	BackgroundBrightness float64           `yaml:"background_brightness"`
	BackgroundContrast   float64           `yaml:"background_contrast"`
	SubjectScale         float64           `yaml:"subject_scale"`
	CardColor            rendererColorYAML `yaml:"card_color"`
	BackgroundTemplate   string            `yaml:"background_template"`
	OverlayTemplate      string            `yaml:"overlay_template"`
	LayoutVariant        string            `yaml:"layout_variant"`
	VisualMode           string            `yaml:"visual_mode"`
	CopySlots            []string          `yaml:"copy_slots"`
	BadgeSlots           []string          `yaml:"badge_slots"`
	MeasurementSlots     []string          `yaml:"measurement_slots"`
	DetailAnchorSlots    []string          `yaml:"detail_anchor_slots"`
	MaxCopyLines         int               `yaml:"max_copy_lines"`
	MaxBadges            int               `yaml:"max_badges"`
	MeasurementMode      string            `yaml:"measurement_mode"`
	DetailAnchorMode     string            `yaml:"detail_anchor_mode"`
}

type rendererColorYAML struct {
	R uint8 `yaml:"r"`
	G uint8 `yaml:"g"`
	B uint8 `yaml:"b"`
	A uint8 `yaml:"a"`
}

var (
	rendererPresetRegistryOnce sync.Once
	rendererPresetRegistryInst *rendererPresetRegistry
	rendererPresetRegistryErr  error
)

func resolveSceneProfile(asset *ImageAsset) sceneProfile {
	profile := strings.TrimSpace(assetMetadataValue(asset, "render_profile"))
	registry, err := loadRendererPresetRegistry()
	if err != nil || registry == nil {
		return defaultSceneProfile(profile)
	}
	return registry.Resolve(profile)
}

func ApplyScenePresetMetadata(metadata map[string]string, profileName string) map[string]string {
	if metadata == nil {
		metadata = map[string]string{}
	}
	profile := defaultSceneProfile(profileName)
	if registry, err := loadRendererPresetRegistry(); err == nil && registry != nil {
		profile = registry.Resolve(profileName)
	}
	setScenePresetMetadata(metadata, profile)
	return metadata
}

func loadRendererPresetRegistry() (*rendererPresetRegistry, error) {
	rendererPresetRegistryOnce.Do(func() {
		data, err := rendererPresetFS.ReadFile("presets/scene_profiles.yaml")
		if err != nil {
			rendererPresetRegistryErr = err
			return
		}
		var registry rendererPresetRegistry
		if err := yaml.Unmarshal(data, &registry); err != nil {
			rendererPresetRegistryErr = err
			return
		}
		registry.byName = make(map[string]sceneProfile, len(registry.Presets))
		for _, preset := range registry.Presets {
			name := strings.TrimSpace(preset.Name)
			if name == "" {
				continue
			}
			normalized := normalizeRendererPreset(preset)
			registry.byName[name] = sceneProfile{
				name:                 name,
				group:                strings.TrimSpace(normalized.Group),
				blurRadius:           normalized.BlurRadius,
				backgroundBrightness: normalized.BackgroundBrightness,
				backgroundContrast:   normalized.BackgroundContrast,
				subjectScale:         normalized.SubjectScale,
				backgroundTemplate:   strings.TrimSpace(normalized.BackgroundTemplate),
				overlayTemplate:      strings.TrimSpace(normalized.OverlayTemplate),
				layoutVariant:        strings.TrimSpace(normalized.LayoutVariant),
				visualMode:           strings.TrimSpace(normalized.VisualMode),
				copySlots:            append([]string(nil), normalized.CopySlots...),
				badgeSlots:           append([]string(nil), normalized.BadgeSlots...),
				measurementSlots:     append([]string(nil), normalized.MeasurementSlots...),
				detailAnchorSlots:    append([]string(nil), normalized.DetailAnchorSlots...),
				maxCopyLines:         normalized.MaxCopyLines,
				maxBadges:            normalized.MaxBadges,
				measurementMode:      strings.TrimSpace(normalized.MeasurementMode),
				detailAnchorMode:     strings.TrimSpace(normalized.DetailAnchorMode),
				cardColor: color.NRGBA{
					R: normalized.CardColor.R,
					G: normalized.CardColor.G,
					B: normalized.CardColor.B,
					A: normalized.CardColor.A,
				},
			}
		}
		rendererPresetRegistryInst = &registry
	})
	return rendererPresetRegistryInst, rendererPresetRegistryErr
}

func (r *rendererPresetRegistry) Resolve(profile string) sceneProfile {
	if r == nil {
		return defaultSceneProfile(profile)
	}
	profile = strings.TrimSpace(profile)
	if profile != "" {
		if preset, ok := r.byName[profile]; ok {
			return preset
		}
	}
	if preset, ok := r.byName[strings.TrimSpace(r.Default)]; ok {
		return preset
	}
	return defaultSceneProfile(profile)
}

func defaultSceneProfile(profile string) sceneProfile {
	return sceneProfile{
		name:                 firstNonEmpty(strings.TrimSpace(profile), "local_canvas_default"),
		group:                "default",
		blurRadius:           14,
		backgroundBrightness: -6,
		backgroundContrast:   8,
		subjectScale:         0.78,
		backgroundTemplate:   "background/default-soft",
		overlayTemplate:      "overlay/neutral-card",
		layoutVariant:        "center_card",
		visualMode:           "catalog",
		copySlots:            []string{"headline"},
		badgeSlots:           []string{"badge_top_left"},
		measurementSlots:     []string{"measurement_bottom"},
		detailAnchorSlots:    []string{"detail_right"},
		maxCopyLines:         2,
		maxBadges:            1,
		measurementMode:      "single_axis",
		detailAnchorMode:     "single_anchor",
		cardColor:            color.NRGBA{R: 245, G: 245, B: 245, A: 230},
	}
}

func assetMetadataValue(asset *ImageAsset, key string) string {
	if asset == nil || asset.Metadata == nil {
		return ""
	}
	return strings.TrimSpace(asset.Metadata[key])
}

func setScenePresetMetadata(metadata map[string]string, profile sceneProfile) {
	if metadata == nil {
		return
	}
	setMetadataDefault(metadata, "background_template", profile.backgroundTemplate)
	setMetadataDefault(metadata, "overlay_template", profile.overlayTemplate)
	setMetadataDefault(metadata, "layout_variant", profile.layoutVariant)
	setMetadataDefault(metadata, "visual_mode", profile.visualMode)
	setMetadataDefault(metadata, "copy_slots", strings.Join(profile.copySlots, ","))
	setMetadataDefault(metadata, "badge_slots", strings.Join(profile.badgeSlots, ","))
	setMetadataDefault(metadata, "measurement_slots", strings.Join(profile.measurementSlots, ","))
	setMetadataDefault(metadata, "detail_anchor_slots", strings.Join(profile.detailAnchorSlots, ","))
	if profile.maxCopyLines > 0 {
		setMetadataDefault(metadata, "max_copy_lines", intToString(profile.maxCopyLines))
	}
	if profile.maxBadges > 0 {
		setMetadataDefault(metadata, "max_badges", intToString(profile.maxBadges))
	}
	setMetadataDefault(metadata, "measurement_mode", profile.measurementMode)
	setMetadataDefault(metadata, "detail_anchor_mode", profile.detailAnchorMode)
	applySellingPointSlotPlanMetadata(metadata, profile)
}

func setMetadataDefault(metadata map[string]string, key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	if strings.TrimSpace(metadata[key]) != "" {
		return
	}
	metadata[key] = value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeRendererPreset(preset rendererPresetYAML) rendererPresetYAML {
	if preset.MaxCopyLines <= 0 {
		preset.MaxCopyLines = 2
	}
	if preset.MaxBadges <= 0 {
		preset.MaxBadges = 1
	}
	if strings.TrimSpace(preset.MeasurementMode) == "" {
		preset.MeasurementMode = "single_axis"
	}
	if strings.TrimSpace(preset.DetailAnchorMode) == "" {
		preset.DetailAnchorMode = "single_anchor"
	}
	return preset
}

func intToString(value int) string {
	return strconv.Itoa(value)
}
