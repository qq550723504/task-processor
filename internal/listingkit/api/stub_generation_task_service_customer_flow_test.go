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

func (s *stubGenerationTaskService) ListSheinStoreProfiles(ctx context.Context) ([]listingkit.ListingKitStoreProfile, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]listingkit.ListingKitStoreProfile(nil), s.storeProfiles...), nil
}

func (s *stubGenerationTaskService) UpsertSheinStoreProfile(ctx context.Context, req *listingkit.ListingKitStoreProfile) (*listingkit.ListingKitStoreProfile, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.upsertStoreProfileReq = req
	if s.upsertedStoreProfile != nil {
		return s.upsertedStoreProfile, nil
	}
	return req, nil
}

func (s *stubGenerationTaskService) DeleteSheinStoreProfile(ctx context.Context, id int64) error {
	return s.err
}

func (s *stubGenerationTaskService) UpdateSheinSettings(ctx context.Context, req *listingkit.SheinSettings) (*listingkit.SheinSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTaskService) GetAIClientSettings(ctx context.Context, scope string, clientName string) (*listingkit.AIClientSettings, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.aiSettings != nil {
		return s.aiSettings, nil
	}
	return &listingkit.AIClientSettings{
		Scope:      scope,
		ClientName: clientName,
	}, nil
}

func (s *stubGenerationTaskService) UpdateAIClientSettings(ctx context.Context, req *listingkit.AIClientSettings) (*listingkit.AIClientSettings, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.aiSettingsReq = req
	if s.aiSettings != nil {
		return s.aiSettings, nil
	}
	return req, nil
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

func (s *stubHistoryDetailService) ListSheinStoreProfiles(ctx context.Context) ([]listingkit.ListingKitStoreProfile, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) UpsertSheinStoreProfile(ctx context.Context, req *listingkit.ListingKitStoreProfile) (*listingkit.ListingKitStoreProfile, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) DeleteSheinStoreProfile(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func (s *stubHistoryDetailService) UpdateSheinSettings(ctx context.Context, req *listingkit.SheinSettings) (*listingkit.SheinSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) GetAIClientSettings(ctx context.Context, scope string, clientName string) (*listingkit.AIClientSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) UpdateAIClientSettings(ctx context.Context, req *listingkit.AIClientSettings) (*listingkit.AIClientSettings, error) {
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

func (s *stubHistoryService) ListSheinStoreProfiles(ctx context.Context) ([]listingkit.ListingKitStoreProfile, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) UpsertSheinStoreProfile(ctx context.Context, req *listingkit.ListingKitStoreProfile) (*listingkit.ListingKitStoreProfile, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) DeleteSheinStoreProfile(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func (s *stubHistoryService) UpdateSheinSettings(ctx context.Context, req *listingkit.SheinSettings) (*listingkit.SheinSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) GetAIClientSettings(ctx context.Context, scope string, clientName string) (*listingkit.AIClientSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) UpdateAIClientSettings(ctx context.Context, req *listingkit.AIClientSettings) (*listingkit.AIClientSettings, error) {
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

func (s *stubRevisionService) ListSheinStoreProfiles(ctx context.Context) ([]listingkit.ListingKitStoreProfile, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionService) UpsertSheinStoreProfile(ctx context.Context, req *listingkit.ListingKitStoreProfile) (*listingkit.ListingKitStoreProfile, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionService) DeleteSheinStoreProfile(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func (s *stubRevisionService) UpdateSheinSettings(ctx context.Context, req *listingkit.SheinSettings) (*listingkit.SheinSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionService) GetAIClientSettings(ctx context.Context, scope string, clientName string) (*listingkit.AIClientSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionService) UpdateAIClientSettings(ctx context.Context, req *listingkit.AIClientSettings) (*listingkit.AIClientSettings, error) {
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

func (s *stubRevisionValidateService) ListSheinStoreProfiles(ctx context.Context) ([]listingkit.ListingKitStoreProfile, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionValidateService) UpsertSheinStoreProfile(ctx context.Context, req *listingkit.ListingKitStoreProfile) (*listingkit.ListingKitStoreProfile, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionValidateService) DeleteSheinStoreProfile(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func (s *stubRevisionValidateService) UpdateSheinSettings(ctx context.Context, req *listingkit.SheinSettings) (*listingkit.SheinSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionValidateService) GetAIClientSettings(ctx context.Context, scope string, clientName string) (*listingkit.AIClientSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionValidateService) UpdateAIClientSettings(ctx context.Context, req *listingkit.AIClientSettings) (*listingkit.AIClientSettings, error) {
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

func (s *stubSubmitService) ListSheinStoreProfiles(ctx context.Context) ([]listingkit.ListingKitStoreProfile, error) {
	return nil, errors.New("not implemented")
}

func (s *stubSubmitService) UpsertSheinStoreProfile(ctx context.Context, req *listingkit.ListingKitStoreProfile) (*listingkit.ListingKitStoreProfile, error) {
	return nil, errors.New("not implemented")
}

func (s *stubSubmitService) DeleteSheinStoreProfile(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func (s *stubSubmitService) UpdateSheinSettings(ctx context.Context, req *listingkit.SheinSettings) (*listingkit.SheinSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubSubmitService) GetAIClientSettings(ctx context.Context, scope string, clientName string) (*listingkit.AIClientSettings, error) {
	return nil, errors.New("not implemented")
}

func (s *stubSubmitService) UpdateAIClientSettings(ctx context.Context, req *listingkit.AIClientSettings) (*listingkit.AIClientSettings, error) {
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
