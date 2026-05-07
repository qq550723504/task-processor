package api

import (
	"context"
	"errors"

	"task-processor/internal/listingkit"
	sheinpub "task-processor/internal/publishing/shein"
)

func (s *stubGenerationTaskService) GetSheinSettings(ctx context.Context) (*listingkit.SheinSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTaskService) UpdateSheinSettings(ctx context.Context, req *listingkit.SheinSettings) (*listingkit.SheinSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTaskService) PreviewSheinPrice(ctx context.Context, taskID string, req *listingkit.SheinPricePreviewRequest) (*sheinpub.PricingReview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTaskService) UpdateSheinFinalDraft(ctx context.Context, taskID string, req *listingkit.SheinFinalDraftUpdateRequest) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTaskService) GetSubmissionEvents(ctx context.Context, taskID string) (*listingkit.SheinSubmissionEventPage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTaskService) RefreshSubmissionStatus(ctx context.Context, taskID string) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) GetSheinSettings(ctx context.Context) (*listingkit.SheinSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) UpdateSheinSettings(ctx context.Context, req *listingkit.SheinSettings) (*listingkit.SheinSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) PreviewSheinPrice(ctx context.Context, taskID string, req *listingkit.SheinPricePreviewRequest) (*sheinpub.PricingReview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) UpdateSheinFinalDraft(ctx context.Context, taskID string, req *listingkit.SheinFinalDraftUpdateRequest) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) GetSubmissionEvents(ctx context.Context, taskID string) (*listingkit.SheinSubmissionEventPage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) RefreshSubmissionStatus(ctx context.Context, taskID string) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) GetSheinSettings(ctx context.Context) (*listingkit.SheinSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) UpdateSheinSettings(ctx context.Context, req *listingkit.SheinSettings) (*listingkit.SheinSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) PreviewSheinPrice(ctx context.Context, taskID string, req *listingkit.SheinPricePreviewRequest) (*sheinpub.PricingReview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) UpdateSheinFinalDraft(ctx context.Context, taskID string, req *listingkit.SheinFinalDraftUpdateRequest) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) GetSubmissionEvents(ctx context.Context, taskID string) (*listingkit.SheinSubmissionEventPage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) RefreshSubmissionStatus(ctx context.Context, taskID string) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionService) GetSheinSettings(ctx context.Context) (*listingkit.SheinSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionService) UpdateSheinSettings(ctx context.Context, req *listingkit.SheinSettings) (*listingkit.SheinSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionService) PreviewSheinPrice(ctx context.Context, taskID string, req *listingkit.SheinPricePreviewRequest) (*sheinpub.PricingReview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionService) UpdateSheinFinalDraft(ctx context.Context, taskID string, req *listingkit.SheinFinalDraftUpdateRequest) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionService) GetSubmissionEvents(ctx context.Context, taskID string) (*listingkit.SheinSubmissionEventPage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionService) RefreshSubmissionStatus(ctx context.Context, taskID string) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionValidateService) GetSheinSettings(ctx context.Context) (*listingkit.SheinSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionValidateService) UpdateSheinSettings(ctx context.Context, req *listingkit.SheinSettings) (*listingkit.SheinSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionValidateService) PreviewSheinPrice(ctx context.Context, taskID string, req *listingkit.SheinPricePreviewRequest) (*sheinpub.PricingReview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionValidateService) UpdateSheinFinalDraft(ctx context.Context, taskID string, req *listingkit.SheinFinalDraftUpdateRequest) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionValidateService) GetSubmissionEvents(ctx context.Context, taskID string) (*listingkit.SheinSubmissionEventPage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionValidateService) RefreshSubmissionStatus(ctx context.Context, taskID string) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubSubmitService) GetSheinSettings(ctx context.Context) (*listingkit.SheinSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubSubmitService) UpdateSheinSettings(ctx context.Context, req *listingkit.SheinSettings) (*listingkit.SheinSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubSubmitService) PreviewSheinPrice(ctx context.Context, taskID string, req *listingkit.SheinPricePreviewRequest) (*sheinpub.PricingReview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubSubmitService) UpdateSheinFinalDraft(ctx context.Context, taskID string, req *listingkit.SheinFinalDraftUpdateRequest) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubSubmitService) GetSubmissionEvents(ctx context.Context, taskID string) (*listingkit.SheinSubmissionEventPage, error) {
	return nil, errors.New("not implemented")
}
