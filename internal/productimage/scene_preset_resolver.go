package productimage

import "strings"

type scenePresetResolution struct {
	Options *SceneGenerationOptions
	Source  string
}

var platformSceneDefaults = map[string]SceneGenerationOptions{
	"amazon": {
		SceneStyle:     "studio",
		BackgroundTone: "bright",
		Composition:    "centered",
		PropsLevel:     "none",
		AudienceHint:   "premium",
	},
	"shein": {
		SceneStyle:     "lifestyle",
		BackgroundTone: "warm",
		Composition:    "close_up",
		PropsLevel:     "light",
		AudienceHint:   "youthful",
	},
	"temu": {
		SceneStyle:     "lifestyle",
		BackgroundTone: "bright",
		Composition:    "multi_angle",
		PropsLevel:     "moderate",
		AudienceHint:   "sporty",
	},
	"walmart": {
		SceneStyle:     "lifestyle",
		BackgroundTone: "neutral",
		Composition:    "centered",
		PropsLevel:     "light",
		AudienceHint:   "homey",
	},
}

var platformCategorySceneDefaults = map[string]map[string]SceneGenerationOptions{
	"amazon": {
		"shoes": {
			SceneStyle:     "studio",
			BackgroundTone: "bright",
			Composition:    "centered",
			PropsLevel:     "none",
			AudienceHint:   "premium",
		},
		"jewelry": {
			SceneStyle:     "studio",
			BackgroundTone: "cool",
			Composition:    "close_up",
			PropsLevel:     "none",
			AudienceHint:   "premium",
		},
		"bags": {
			SceneStyle:     "studio",
			BackgroundTone: "neutral",
			Composition:    "centered",
			PropsLevel:     "none",
			AudienceHint:   "premium",
		},
	},
	"shein": {
		"shoes": {
			SceneStyle:     "lifestyle",
			BackgroundTone: "warm",
			Composition:    "close_up",
			PropsLevel:     "light",
			AudienceHint:   "youthful",
		},
		"jewelry": {
			SceneStyle:     "lifestyle",
			BackgroundTone: "warm",
			Composition:    "close_up",
			PropsLevel:     "light",
			AudienceHint:   "youthful",
		},
		"bags": {
			SceneStyle:     "lifestyle",
			BackgroundTone: "warm",
			Composition:    "multi_angle",
			PropsLevel:     "light",
			AudienceHint:   "youthful",
		},
	},
	"temu": {
		"shoes": {
			SceneStyle:     "lifestyle",
			BackgroundTone: "bright",
			Composition:    "multi_angle",
			PropsLevel:     "moderate",
			AudienceHint:   "sporty",
		},
		"jewelry": {
			SceneStyle:     "lifestyle",
			BackgroundTone: "bright",
			Composition:    "close_up",
			PropsLevel:     "light",
			AudienceHint:   "youthful",
		},
		"bags": {
			SceneStyle:     "lifestyle",
			BackgroundTone: "bright",
			Composition:    "multi_angle",
			PropsLevel:     "moderate",
			AudienceHint:   "sporty",
		},
	},
	"walmart": {
		"shoes": {
			SceneStyle:     "lifestyle",
			BackgroundTone: "neutral",
			Composition:    "centered",
			PropsLevel:     "light",
			AudienceHint:   "homey",
		},
		"jewelry": {
			SceneStyle:     "studio",
			BackgroundTone: "neutral",
			Composition:    "close_up",
			PropsLevel:     "none",
			AudienceHint:   "premium",
		},
		"bags": {
			SceneStyle:     "lifestyle",
			BackgroundTone: "neutral",
			Composition:    "centered",
			PropsLevel:     "light",
			AudienceHint:   "homey",
		},
	},
}

func resolveScenePreset(marketplace, category string) scenePresetResolution {
	normalizedMarketplace := strings.ToLower(strings.TrimSpace(marketplace))
	normalizedCategory := strings.ToLower(strings.TrimSpace(category))
	if presets, ok := platformCategorySceneDefaults[normalizedMarketplace]; ok {
		if preset, ok := presets[normalizedCategory]; ok {
			cloned := preset
			return scenePresetResolution{
				Options: &cloned,
				Source:  "platform_category",
			}
		}
	}
	if preset, ok := platformSceneDefaults[normalizedMarketplace]; ok {
		cloned := preset
		return scenePresetResolution{
			Options: &cloned,
			Source:  "platform",
		}
	}
	return scenePresetResolution{Source: "fallback"}
}

