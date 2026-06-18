package httpapi

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"task-processor/internal/listingkit"
	"task-processor/internal/shein/activity"
)

type sheinPromotionBridgeRuntimeFactory struct {
	storeCatalog listingkit.SheinStoreCatalog
	apiFactory   listingkit.SheinAPIClientFactory
}

func buildSheinPromotionBridgeRuntimeFactory(input BuildServiceInput, repositories *builtRepositories) sheinPromotionBridgeRuntimeFactory {
	return sheinPromotionBridgeRuntimeFactory{
		storeCatalog: sheinManagementStoreCatalog{repo: repositories.storeRepository},
		apiFactory:   input.Hooks.SheinAPIClientFactoryBuilder(repositories.storeRepository),
	}
}

func (f sheinPromotionBridgeRuntimeFactory) BuildPromotionBridge(ctx context.Context, storeID int64) (activity.PromotionRegistrationBridge, error) {
	if f.storeCatalog == nil {
		return nil, fmt.Errorf("SHEIN store catalog is not configured")
	}
	if f.apiFactory == nil {
		return nil, fmt.Errorf("SHEIN API client factory is not configured")
	}

	tenantID, err := sheinRuntimeTenantID(ctx)
	if err != nil {
		return nil, err
	}
	storeInfo, err := f.storeCatalog.GetStoreInfo(ctx, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	apiClient := f.apiFactory.NewSheinAPIClient(storeID, storeInfo)
	if apiClient == nil {
		return nil, fmt.Errorf("SHEIN API client is unavailable")
	}

	return buildListingKitPromotionRegistrationBridge(apiClient), nil
}

func sheinRuntimeTenantID(ctx context.Context) (int64, error) {
	value := strings.TrimSpace(listingkit.TenantIDFromContext(ctx))
	if value == "" {
		return 0, fmt.Errorf("tenant id is required")
	}
	tenantID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || tenantID <= 0 {
		return 0, fmt.Errorf("tenant id must be numeric")
	}
	return tenantID, nil
}
