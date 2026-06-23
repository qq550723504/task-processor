package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

func validateStudioSessionExpectedUpdatedAt(current time.Time, expected *string) error {
	if expected == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*expected)
	if trimmed == "" || current.IsZero() {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, trimmed)
	if err != nil {
		return fmt.Errorf("invalid expected_updated_at: %w", err)
	}
	if !current.UTC().Equal(parsed.UTC()) {
		return ErrStudioSessionConflict
	}
	return nil
}

func studioSessionStringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func (s *taskStudioSessionService) loadStudioSessionDetail(ctx context.Context, session *SheinStudioSession) (*SheinStudioSessionDetail, error) {
	designs, err := s.repo.ListSessionDesigns(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	return &SheinStudioSessionDetail{
		Session: session,
		Designs: designs,
	}, nil
}

func studioSessionDetailWithoutDesigns(session *SheinStudioSession) *SheinStudioSessionDetail {
	return &SheinStudioSessionDetail{
		Session: session,
		Designs: []SheinStudioDesign{},
	}
}

func isStudioSessionGenerationMetadataOnlyUpdate(req *UpdateStudioSessionRequest) bool {
	if req == nil {
		return false
	}
	hasGenerationMetadata := req.Status != nil || req.GenerationJobID != nil || req.GenerationJobs != nil || req.GenerationError != nil
	if !hasGenerationMetadata {
		return false
	}
	return req.Prompt == nil &&
		req.PromptMode == nil &&
		req.StyleCount == nil &&
		req.VariationIntensity == nil &&
		req.ProductImageCount == nil &&
		req.ProductImagePrompt == nil &&
		req.ProductImagePrompts == nil &&
		req.ArtworkModel == nil &&
		req.ImageStrategy == nil &&
		req.GroupedImageMode == nil &&
		req.SelectedSDSImages == nil &&
		req.GroupedSelections == nil &&
		req.TransparentBackground == nil &&
		req.RenderSizeImagesWithSDS == nil &&
		req.SheinStoreID == nil &&
		req.ApprovedDesignIDs == nil &&
		req.CreatedTasks == nil
}

func isStudioSessionReviewTaskMetadataOnlyUpdate(req *UpdateStudioSessionRequest) bool {
	if req == nil {
		return false
	}
	hasReviewTaskMetadata := req.ApprovedDesignIDs != nil || req.CreatedTasks != nil
	if !hasReviewTaskMetadata {
		return false
	}
	return req.Status == nil &&
		req.Prompt == nil &&
		req.PromptMode == nil &&
		req.StyleCount == nil &&
		req.VariationIntensity == nil &&
		req.ProductImageCount == nil &&
		req.ProductImagePrompt == nil &&
		req.ProductImagePrompts == nil &&
		req.ArtworkModel == nil &&
		req.ImageStrategy == nil &&
		req.GroupedImageMode == nil &&
		req.SelectedSDSImages == nil &&
		req.GroupedSelections == nil &&
		req.TransparentBackground == nil &&
		req.RenderSizeImagesWithSDS == nil &&
		req.SheinStoreID == nil &&
		req.GenerationJobID == nil &&
		req.GenerationJobs == nil &&
		req.GenerationError == nil
}

func (s *taskStudioSessionService) ensureRunner() {
	if s == nil || s.runner != nil || s.repo == nil {
		return
	}
	s.runner = newListingStudioSessionService(s.repo)
}

func (s *taskStudioSessionService) ensureAsyncJobRunner() {
	if s == nil || s.asyncJobRunner != nil || s.repo == nil {
		return
	}
	s.asyncJobRunner = newListingStudioSessionAsyncJobService(s.repo)
}

func (s *taskStudioSessionService) ensureGenerationMetadataRunner() {
	if s == nil || s.generationMetadataRunner != nil || s.repo == nil {
		return
	}
	s.generationMetadataRunner = newListingStudioSessionGenerationMetadataService(s.repo)
}

func (s *taskStudioSessionService) ensureReviewTaskMetadataRunner() {
	if s == nil || s.reviewTaskMetadataRunner != nil || s.repo == nil {
		return
	}
	s.reviewTaskMetadataRunner = newListingStudioSessionReviewTaskMetadataService(s.repo)
}

func (s *taskStudioSessionService) ensureGeneralMetadataRunner() {
	if s == nil || s.generalMetadataRunner != nil || s.repo == nil {
		return
	}
	s.generalMetadataRunner = newListingStudioSessionGeneralMetadataService(s.repo)
}

func studioSessionLogFields(ctx context.Context, fields logrus.Fields) logrus.Fields {
	if fields == nil {
		fields = logrus.Fields{}
	}
	for key, value := range RequestTraceFromContext(ctx).LogFields() {
		if value == "" || value == 0 {
			continue
		}
		fields[key] = value
	}
	return fields
}
