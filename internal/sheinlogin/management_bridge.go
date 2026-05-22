package sheinlogin

import (
	"context"
	"strings"

	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
)

func NewManagementStoreSyncClientFactory(client *management.ClientManager) func(tenantID int64) StoreSyncClient {
	return func(tenantID int64) StoreSyncClient {
		if client == nil {
			return nil
		}
		return client.GetStoreClientWithTenant(tenantID)
	}
}

func NewManagementDuplicateStoreLookup(client *management.ClientManager) func(ctx context.Context, account Account, actualStoreID string) (*managementapi.StoreRespDTO, error) {
	return func(ctx context.Context, account Account, actualStoreID string) (*managementapi.StoreRespDTO, error) {
		if client == nil || strings.TrimSpace(actualStoreID) == "" {
			return nil, nil
		}

		storeClient := client.GetStoreClient()
		if storeClient == nil {
			return nil, nil
		}

		pageNo := 1
		pageSize := 200
		for {
			page, err := storeClient.PageStores(&managementapi.StorePageReqDTO{
				Platform: "SHEIN",
				PageNo:   pageNo,
				PageSize: pageSize,
			})
			if err != nil {
				return nil, err
			}
			for _, item := range page.List {
				if item == nil || item.ID == account.StoreID {
					continue
				}
				if strings.EqualFold(strings.TrimSpace(item.Platform), "shein") && strings.TrimSpace(item.StoreID) == actualStoreID {
					return item, nil
				}
			}
			if int64(pageNo*pageSize) >= page.Total || len(page.List) == 0 {
				break
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			pageNo++
		}
		return nil, nil
	}
}
