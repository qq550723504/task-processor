package shein

import sheinclient "task-processor/internal/shein/client"

type RuntimeBaseAPIClient = sheinclient.BaseAPIClient

func newRuntimeBaseAPIClient(apiClient RuntimeAPIClient, storeID int64) *RuntimeBaseAPIClient {
	if apiClient == nil {
		return nil
	}
	baseAPI := sheinclient.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		storeID,
		apiClient.GetHTTPClient(),
	)
	baseAPI.SetAuthRefreshFunc(apiClient.ForceRefreshCookies)
	return baseAPI
}
