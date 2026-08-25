package listingkit

import studiodomain "task-processor/internal/listing/studio"

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
	return StudioTransparencyMode(studiodomain.NormalizeTransparencyMode(mode, legacy))
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
	return studiodomain.ValidateTransparentPNG(data)
}
