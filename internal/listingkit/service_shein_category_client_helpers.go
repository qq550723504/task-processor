package listingkit

import (
	"context"
	"fmt"

	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
	sheinclient "task-processor/internal/shein/client"
)

func (s *service) buildSheinAttributeAPI(ctx context.Context, task *Task) (sheinpub.AttributeAPI, error) {
	apiClient, storeID, err := s.newSheinAPIClient(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("%w for attribute resolution", err)
	}
	if !apiClient.HasCookies() {
		if err := apiClient.ForceRefreshCookies(); err != nil {
			return nil, fmt.Errorf("shein store cookies are unavailable for attribute resolution: %w", err)
		}
	}
	if !apiClient.HasCookies() {
		return nil, fmt.Errorf("shein store cookies are unavailable for attribute resolution")
	}

	baseAPI := sheinclient.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		storeID,
		apiClient.GetHTTPClient(),
	)
	baseAPI.SetAuthRefreshFunc(apiClient.ForceRefreshCookies)
	return sheinattribute.NewClient(baseAPI), nil
}

func (s *service) buildSheinCategoryAPI(ctx context.Context, task *Task) (sheincategory.CategoryAPI, error) {
	apiClient, storeID, err := s.newSheinAPIClient(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("%w for category resolution", err)
	}
	if !apiClient.HasCookies() {
		if err := apiClient.ForceRefreshCookies(); err != nil {
			return nil, fmt.Errorf("shein store cookies are unavailable for category resolution: %w", err)
		}
	}
	if !apiClient.HasCookies() {
		return nil, fmt.Errorf("shein store cookies are unavailable for category resolution")
	}

	baseAPI := sheinclient.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		storeID,
		apiClient.GetHTTPClient(),
	)
	baseAPI.SetAuthRefreshFunc(apiClient.ForceRefreshCookies)
	return sheincategory.NewClient(baseAPI), nil
}
