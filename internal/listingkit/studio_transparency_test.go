package listingkit

import (
	"encoding/json"
	"testing"
)

func TestNormalizeStudioTransparencyMode(t *testing.T) {
	tests := []struct {
		name   string
		mode   string
		legacy *bool
		want   StudioTransparencyMode
	}{
		{name: "explicit removal wins over legacy native", mode: "removal", legacy: studioBoolPtr(true), want: StudioTransparencyModeRemoval},
		{name: "explicit native", mode: "native", legacy: studioBoolPtr(false), want: StudioTransparencyModeNative},
		{name: "legacy native", legacy: studioBoolPtr(true), want: StudioTransparencyModeNative},
		{name: "legacy none", legacy: studioBoolPtr(false), want: StudioTransparencyModeNone},
		{name: "empty defaults to none", want: StudioTransparencyModeNone},
		{name: "invalid defaults to none", mode: "mask", legacy: studioBoolPtr(true), want: StudioTransparencyModeNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeStudioTransparencyMode(tt.mode, tt.legacy); got != tt.want {
				t.Fatalf("NormalizeStudioTransparencyMode(%q, %v) = %q, want %q", tt.mode, tt.legacy, got, tt.want)
			}
		})
	}
}

func TestStudioTransparencyStatusValues(t *testing.T) {
	if StudioBackgroundRemovalStatusNotRequested != "not_requested" ||
		StudioBackgroundRemovalStatusPending != "pending" ||
		StudioBackgroundRemovalStatusSucceeded != "succeeded" ||
		StudioBackgroundRemovalStatusFailed != "failed" {
		t.Fatalf("unexpected background removal status values")
	}
}

func TestStudioTransparencyFieldsSerialize(t *testing.T) {
	payload, err := json.Marshal(StudioGeneratedImage{
		ID:                        "design-1",
		ImageURL:                  "https://example.test/final.png",
		OriginalImageURL:          "https://example.test/source.png",
		TransparentBackground:     true,
		TransparentBackgroundMode: StudioTransparencyModeRemoval,
		BackgroundRemovalStatus:   StudioBackgroundRemovalStatusSucceeded,
		BackgroundRemovalModel:    "rmbg-model",
	})
	if err != nil {
		t.Fatalf("marshal generated image: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal generated image: %v", err)
	}
	for key, want := range map[string]any{
		"original_image_url":          "https://example.test/source.png",
		"transparent_background_mode": "removal",
		"background_removal_status":   "succeeded",
		"background_removal_model":    "rmbg-model",
	} {
		if got[key] != want {
			t.Fatalf("%s = %#v, want %#v", key, got[key], want)
		}
	}
}

func studioBoolPtr(value bool) *bool {
	return &value
}
