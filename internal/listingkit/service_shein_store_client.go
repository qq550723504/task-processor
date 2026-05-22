package listingkit

import (
	"context"
	"fmt"

	sheinclient "task-processor/internal/shein/client"
)

func (s *service) resolveSheinStoreInfo(ctx context.Context, task *Task) (*SheinStoreInfo, error) {
	if s == nil || s.sheinStoreCatalog == nil {
		return nil, fmt.Errorf("shein store catalog is unavailable")
	}

	storeID, err := s.resolveSheinStoreID(ctx, task)
	if err != nil || storeID <= 0 {
		return nil, fmt.Errorf("shein store id is unavailable")
	}

	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		tenantID = tenantIDInt64FromTask(task)
	}
	if tenantID <= 0 {
		return nil, fmt.Errorf("shein store tenant is unavailable")
	}

	storeInfo, err := s.sheinStoreCatalog.GetStoreInfo(ctx, tenantID, storeID)
	if err != nil {
		return nil, fmt.Errorf("load shein store info: %w", err)
	}
	if storeInfo == nil || storeInfo.ID <= 0 {
		return nil, fmt.Errorf("shein store info is unavailable")
	}
	if storeInfo.TenantID <= 0 {
		return nil, fmt.Errorf("shein store tenant is unavailable")
	}
	return storeInfo, nil
}

func (s *service) newSheinAPIClient(ctx context.Context, task *Task) (*sheinclient.APIClient, int64, error) {
	if s == nil || s.sheinAPIClientFactory == nil {
		return nil, 0, fmt.Errorf("shein api client factory is unavailable")
	}
	storeInfo, err := s.resolveSheinStoreInfo(ctx, task)
	if err != nil {
		return nil, 0, err
	}
	return s.sheinAPIClientFactory.NewSheinAPIClient(storeInfo.ID, storeInfo), storeInfo.ID, nil
}
