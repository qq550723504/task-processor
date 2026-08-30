package studio

import (
	"fmt"
	"strings"
)

// FindManualBackgroundRemovalDesign locates a design for the manual upload
// flow and enforces the single state that cannot be manually overwritten.
// The bool is false when the caller should produce its batch-specific
// not-found error.
func FindManualBackgroundRemovalDesign(designs []BackgroundRemovalDesign, designID string) (int, bool, error) {
	normalizedID := strings.TrimSpace(designID)
	if normalizedID == "" {
		return -1, false, NewBackgroundRemovalValidationError("design_id is required")
	}
	for index, design := range designs {
		if strings.TrimSpace(design.ID) != normalizedID {
			continue
		}
		if design.BackgroundRemovalStatus == BackgroundRemovalStatusPending {
			return -1, false, NewBackgroundRemovalValidationError(fmt.Sprintf("design %s background removal is already in progress", normalizedID))
		}
		return index, true, nil
	}
	return -1, false, nil
}

type ManualBackgroundRemovalInput struct {
	DesignID            string
	OriginalImageURL    string
	ImageURL            string
	ReplacementImageURL string
}

type ManualBackgroundRemovalFields struct {
	OriginalImageURL          string
	ImageURL                  string
	TransparentBackgroundMode TransparencyMode
	BackgroundRemovalStatus   BackgroundRemovalStatus
	BackgroundRemovalError    string
	BackgroundRemovalModel    string
}

// PrepareManualBackgroundRemoval builds the persisted state for a successful
// manual replacement while keeping validation and normalization outside the
// ListingKit repository adapter.
func PrepareManualBackgroundRemoval(input ManualBackgroundRemovalInput) (ManualBackgroundRemovalFields, error) {
	if strings.TrimSpace(input.DesignID) == "" {
		return ManualBackgroundRemovalFields{}, NewBackgroundRemovalValidationError("manual background removal design is required")
	}
	replacementURL := strings.TrimSpace(input.ReplacementImageURL)
	if replacementURL == "" {
		return ManualBackgroundRemovalFields{}, NewBackgroundRemovalValidationError("manual background removal image URL is required")
	}
	sourceURL := strings.TrimSpace(input.OriginalImageURL)
	if sourceURL == "" {
		sourceURL = strings.TrimSpace(input.ImageURL)
	}
	if sourceURL == "" {
		return ManualBackgroundRemovalFields{}, NewBackgroundRemovalValidationError(fmt.Sprintf("design %s has no original image", strings.TrimSpace(input.DesignID)))
	}
	return ManualBackgroundRemovalFields{
		OriginalImageURL:          sourceURL,
		ImageURL:                  replacementURL,
		TransparentBackgroundMode: TransparencyModeRemoval,
		BackgroundRemovalStatus:   BackgroundRemovalStatusSucceeded,
	}, nil
}
