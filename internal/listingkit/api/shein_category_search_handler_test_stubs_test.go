package api

import (
	"context"
	"errors"

	"task-processor/internal/listingkit"
)

func (s *stubGenerationTaskService) SearchSheinCategories(ctx context.Context, taskID string, query string) (*listingkit.SheinCategorySearchResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) SearchSheinCategories(ctx context.Context, taskID string, query string) (*listingkit.SheinCategorySearchResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) SearchSheinCategories(ctx context.Context, taskID string, query string) (*listingkit.SheinCategorySearchResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionService) SearchSheinCategories(ctx context.Context, taskID string, query string) (*listingkit.SheinCategorySearchResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionValidateService) SearchSheinCategories(ctx context.Context, taskID string, query string) (*listingkit.SheinCategorySearchResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubSubmitService) SearchSheinCategories(ctx context.Context, taskID string, query string) (*listingkit.SheinCategorySearchResult, error) {
	return nil, errors.New("not implemented")
}
