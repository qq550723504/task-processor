package listingkit

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"strings"
)

type StudioTransparencyMode string

const (
	StudioTransparencyModeNone    StudioTransparencyMode = "none"
	StudioTransparencyModeNative  StudioTransparencyMode = "native"
	StudioTransparencyModeRemoval StudioTransparencyMode = "removal"
)

type StudioBackgroundRemovalStatus string

const (
	StudioBackgroundRemovalStatusNotRequested StudioBackgroundRemovalStatus = "not_requested"
	StudioBackgroundRemovalStatusPending      StudioBackgroundRemovalStatus = "pending"
	StudioBackgroundRemovalStatusSucceeded    StudioBackgroundRemovalStatus = "succeeded"
	StudioBackgroundRemovalStatusFailed       StudioBackgroundRemovalStatus = "failed"
)

func NormalizeStudioTransparencyMode(mode string, legacy *bool) StudioTransparencyMode {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	switch StudioTransparencyMode(normalized) {
	case StudioTransparencyModeNone:
		return StudioTransparencyModeNone
	case StudioTransparencyModeNative:
		return StudioTransparencyModeNative
	case StudioTransparencyModeRemoval:
		return StudioTransparencyModeRemoval
	}
	if normalized != "" {
		return StudioTransparencyModeNone
	}
	if legacy != nil && *legacy {
		return StudioTransparencyModeNative
	}
	return StudioTransparencyModeNone
}

func studioDesignTransparencyMode(req *StudioDesignRequest) StudioTransparencyMode {
	if req == nil {
		return StudioTransparencyModeNone
	}
	legacy := req.TransparentBackground
	return NormalizeStudioTransparencyMode(string(req.TransparentBackgroundMode), &legacy)
}

func studioDesignUsesNativeTransparency(req *StudioDesignRequest) bool {
	return studioDesignTransparencyMode(req) == StudioTransparencyModeNative
}

func validateStudioTransparentPNG(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("transparent image is empty")
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode transparent PNG: %w", err)
	}
	switch decoded.ColorModel() {
	case color.RGBAModel, color.NRGBAModel, color.RGBA64Model, color.NRGBA64Model,
		color.AlphaModel, color.Alpha16Model, color.NYCbCrAModel:
		// Continue below to verify that at least one pixel is actually transparent.
	default:
		return fmt.Errorf("transparent image does not expose alpha")
	}
	for y := decoded.Bounds().Min.Y; y < decoded.Bounds().Max.Y; y++ {
		for x := decoded.Bounds().Min.X; x < decoded.Bounds().Max.X; x++ {
			_, _, _, alpha := decoded.At(x, y).RGBA()
			if alpha < 0xffff {
				return nil
			}
		}
	}
	return fmt.Errorf("transparent image contains no transparent pixels")
}
