package listingkit

import (
	"strings"
	"time"

	listingplatform "task-processor/internal/listing/platform"
)

func applyListingKitRevision(result *ListingKitResult, req *ApplyRevisionRequest) error {
	if result == nil {
		return ErrTaskResultUnavailable
	}
	if req == nil {
		return ErrInvalidRevisionRequest
	}
	if err := validateApplyRevisionRequest(req); err != nil {
		return err
	}

	platform, ok := listingplatform.ValidateSelectedPlatform(req.Platform)
	if !ok {
		return ErrUnsupportedPreviewPlatform
	}

	switch platform {
	case "amazon":
		if req.Amazon == nil {
			return ErrInvalidRevisionRequest
		}
		if result.Amazon == nil || result.Amazon.Draft == nil {
			return ErrPreviewPlatformUnavailable
		}
		applyAmazonRevision(result.Amazon, req.Amazon)
	case "shein":
		if req.Shein == nil {
			return ErrInvalidRevisionRequest
		}
		if result.Shein == nil {
			return ErrPreviewPlatformUnavailable
		}
		applySheinRevision(result.Shein, req.Shein)
	case "temu":
		if req.Temu == nil {
			return ErrInvalidRevisionRequest
		}
		if result.Temu == nil {
			return ErrPreviewPlatformUnavailable
		}
		applyTemuRevision(result.Temu, req.Temu)
	case "walmart":
		if req.Walmart == nil {
			return ErrInvalidRevisionRequest
		}
		if result.Walmart == nil {
			return ErrPreviewPlatformUnavailable
		}
		applyWalmartRevision(result.Walmart, req.Walmart)
	}

	result.UpdatedAt = time.Now()
	result.Revision = &ListingKitRevisionSummary{
		UpdatedAt: result.UpdatedAt,
		UpdatedBy: strings.TrimSpace(req.Actor),
		Reason:    strings.TrimSpace(req.Reason),
		Platform:  platform,
	}
	return nil
}
