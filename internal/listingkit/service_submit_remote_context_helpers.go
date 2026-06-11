package listingkit

import (
	"context"
	"fmt"

	sheinother "task-processor/internal/shein/api/other"
	sheinclient "task-processor/internal/shein/client"
)

func (s *service) resolveSheinStoreInfo(ctx context.Context, task *Task) (*SheinStoreInfo, error) {
	return buildSubmitRuntimeContextResolver(s).resolveStoreInfo(ctx, task)
}

func (s *service) newSheinAPIClient(ctx context.Context, task *Task) (*sheinclient.APIClient, int64, error) {
	return buildSubmitRuntimeContextResolver(s).newAPIClient(ctx, task)
}

func (s *service) buildSheinSubmitOtherAPI(ctx context.Context, task *Task) (sheinother.OtherAPI, error) {
	resolver := buildSubmitRuntimeContextResolver(s)
	apiClient, storeID, err := resolver.newAPIClient(ctx, task)
	if err != nil {
		return nil, err
	}
	if !apiClient.HasCookies() {
		if err := apiClient.ForceRefreshCookies(); err != nil {
			return nil, fmt.Errorf("shein other api auth unavailable: %w", err)
		}
	}
	if !apiClient.HasCookies() {
		return nil, fmt.Errorf("shein other api auth unavailable")
	}
	baseAPI := NewSheinRuntimeBaseAPIClient(apiClient, storeID)
	return sheinother.NewClient(baseAPI), nil
}
