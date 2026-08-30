package studio

import (
	"fmt"
	"strings"
)

type BackgroundRemovalStatus string

const (
	BackgroundRemovalStatusNotRequested BackgroundRemovalStatus = "not_requested"
	BackgroundRemovalStatusPending      BackgroundRemovalStatus = "pending"
	BackgroundRemovalStatusSucceeded    BackgroundRemovalStatus = "succeeded"
	BackgroundRemovalStatusFailed       BackgroundRemovalStatus = "failed"
)

// BackgroundRemovalDesign is the transport-neutral subset needed to select
// designs eligible for a background-removal retry.
type BackgroundRemovalDesign struct {
	ID                        string
	OriginalImageURL          string
	ImageURL                  string
	TransparentBackgroundMode TransparencyMode
	BackgroundRemovalStatus   BackgroundRemovalStatus
}

type BackgroundRemovalTarget struct {
	DesignIndex int
	DesignID    string
	SourceURL   string
}

type BackgroundRemovalValidationError struct{ Message string }

func (e *BackgroundRemovalValidationError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func NewBackgroundRemovalValidationError(message string) error {
	return &BackgroundRemovalValidationError{Message: message}
}

// SelectBackgroundRemovalTargets applies the stable Studio eligibility rules.
// An explicit request may retry a succeeded design and may use ImageURL when
// OriginalImageURL is unavailable; an implicit retry only selects failed or
// otherwise unfinished removal designs and requires the original source URL.
func SelectBackgroundRemovalTargets(designs []BackgroundRemovalDesign, requestedIDs []string) ([]BackgroundRemovalTarget, error) {
	requested := normalizeBackgroundRemovalIDs(requestedIDs)
	requestedSet := make(map[string]struct{}, len(requested))
	for _, designID := range requested {
		requestedSet[designID] = struct{}{}
	}

	targets := make([]BackgroundRemovalTarget, 0)
	for index, design := range designs {
		designID := strings.TrimSpace(design.ID)
		if len(requestedSet) > 0 {
			if _, ok := requestedSet[designID]; !ok {
				continue
			}
		} else if design.TransparentBackgroundMode != TransparencyModeRemoval || design.BackgroundRemovalStatus == BackgroundRemovalStatusSucceeded {
			continue
		}

		sourceURL := strings.TrimSpace(design.OriginalImageURL)
		if len(requestedSet) > 0 && sourceURL == "" {
			sourceURL = strings.TrimSpace(design.ImageURL)
		}
		if sourceURL == "" {
			return nil, NewBackgroundRemovalValidationError(fmt.Sprintf("design %s has no original image", designID))
		}
		if design.BackgroundRemovalStatus == BackgroundRemovalStatusPending {
			return nil, NewBackgroundRemovalValidationError(fmt.Sprintf("design %s background removal is already in progress", designID))
		}
		targets = append(targets, BackgroundRemovalTarget{DesignIndex: index, DesignID: designID, SourceURL: sourceURL})
	}
	if len(requestedSet) > 0 && len(targets) != len(requestedSet) {
		return nil, NewBackgroundRemovalValidationError("one or more requested designs are not eligible for background removal")
	}
	return targets, nil
}

func normalizeBackgroundRemovalIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	result := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		normalized := strings.TrimSpace(id)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}
