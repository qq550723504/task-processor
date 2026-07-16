package httpapi

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingruntime"
	"task-processor/internal/shein/activity"
	"task-processor/internal/tenantbridge"
)

type sheinPromotionBridgeRuntimeFactory struct {
	storeCatalog         listingkit.SheinStoreCatalog
	storeAccessValidator listingkit.StoreAccessValidator
	apiFactory           listingkit.SheinAPIClientFactory
	storeRepository      listingadmin.StoreRepository
	mappingRepo          listingadmin.ProductImportMappingRepository
	productDataRepo      listingadmin.ProductDataRepository
}

func buildSheinPromotionBridgeRuntimeFactory(input BuildServiceInput, repositories *builtRepositories) sheinPromotionBridgeRuntimeFactory {
	return sheinPromotionBridgeRuntimeFactory{
		storeCatalog:         sheinListingStoreCatalog{repo: repositories.storeRepository},
		storeAccessValidator: listingAdminStoreAccessValidator{repo: repositories.storeRepository},
		apiFactory:           input.Hooks.SheinAPIClientFactoryBuilder(repositories.storeRepository),
		storeRepository:      repositories.storeRepository,
		mappingRepo:          repositories.productImportMappingRepository,
		productDataRepo:      repositories.productDataRepository,
	}
}

func buildSheinEnrollmentAdapter(input BuildServiceInput, repositories *builtRepositories, strategyProvider localRuntimePromotionStrategyProvider) listingkit.SheinActivityAdapter {
	bridgeFactory := buildSheinPromotionBridgeRuntimeFactory(input, repositories)
	return listingkit.NewSheinActivityAdapterWithFactory(strategyProvider, bridgeFactory)
}

func (f sheinPromotionBridgeRuntimeFactory) BuildPromotionBridge(ctx context.Context, storeID int64) (activity.PromotionRegistrationBridge, error) {
	if f.storeCatalog == nil {
		return nil, fmt.Errorf("SHEIN store catalog is not configured")
	}
	if f.apiFactory == nil {
		return nil, fmt.Errorf("SHEIN API client factory is not configured")
	}
	if f.storeAccessValidator == nil {
		return nil, listingkit.NewStoreAccessError(listingkit.StoreAccessUnavailable, "store is unavailable")
	}

	tenantID, err := sheinRuntimeTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := f.storeAccessValidator.ValidateStoreAccess(ctx, tenantID, storeID, "SHEIN"); err != nil {
		return nil, err
	}
	storeInfo, err := f.storeCatalog.GetStoreInfo(ctx, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	if storeInfo == nil || storeInfo.ID != storeID || storeInfo.TenantID != tenantID || !strings.EqualFold(strings.TrimSpace(storeInfo.Platform), "SHEIN") {
		return nil, listingkit.NewStoreAccessError(listingkit.StoreAccessUnavailable, "store is unavailable")
	}
	apiClient := f.apiFactory.NewSheinAPIClient(storeID, storeInfo)
	if apiClient == nil {
		return nil, fmt.Errorf("SHEIN API client is unavailable")
	}

	return buildListingKitPromotionRegistrationBridgeWithDependencies(
		apiClient,
		sheinListingStoreService{repo: f.storeRepository, tenantID: tenantID},
		f.mappingRepo,
		f.productDataRepo,
	), nil
}

func sheinRuntimeTenantID(ctx context.Context) (int64, error) {
	value := strings.TrimSpace(listingkit.TenantIDFromContext(ctx))
	if value == "" {
		return 0, fmt.Errorf("tenant id is required")
	}
	tenantID, err := tenantbridge.ResolveLegacyTenantID(ctx, value)
	if err != nil || tenantID <= 0 {
		return 0, listingkit.NewStoreAccessError(listingkit.StoreAccessUnavailable, "store is unavailable")
	}
	return tenantID, nil
}

type sheinListingStoreService struct {
	repo     listingadmin.StoreRepository
	tenantID int64
}

func (s sheinListingStoreService) GetStore(storeID int64) (*listingruntime.StoreInfo, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("store repository is not configured")
	}
	store, err := s.repo.GetStore(context.Background(), s.tenantID, storeID)
	if err != nil || store == nil {
		return nil, err
	}
	return &listingruntime.StoreInfo{
		ID:                       store.ID,
		TenantID:                 store.TenantID,
		StoreID:                  store.StoreID,
		Username:                 store.Username,
		Platform:                 store.Platform,
		Name:                     store.Name,
		Region:                   store.Region,
		ShopType:                 store.ShopType,
		LoginURL:                 store.LoginURL,
		Proxy:                    store.Proxy,
		PriceType:                store.PriceType,
		DailyLimit:               store.DailyLimit,
		DailyLimitType:           store.DailyLimitType,
		EnableDraft:              store.EnableDraft,
		EnableAutoListing:        store.EnableAutoListing,
		FixedStockCount:          store.FixedStockCount,
		SkuGenerateStrategy:      store.SKUGenerateStrategy,
		Prefix:                   store.Prefix,
		Suffix:                   store.Suffix,
		EnableBrandAuthorization: store.EnableBrandAuthorization,
		AuthorizedBrandCode:      store.AuthorizedBrandCode,
		AuthorizedBrandName:      store.AuthorizedBrandName,
	}, nil
}

func (s sheinListingStoreService) GetStorePauseStatus(storeID int64) (bool, error) {
	return false, fmt.Errorf("store pause status is not supported for store %d", storeID)
}

func (s sheinListingStoreService) GetStorePauseStatusDetail(storeID int64) (*listingruntime.StorePauseStatusDetail, error) {
	return nil, fmt.Errorf("store pause status detail is not supported for store %d", storeID)
}

func (s sheinListingStoreService) SetStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error) {
	_ = pause
	_ = pauseType
	return false, fmt.Errorf("store pause status update is not supported for store %d", storeID)
}
