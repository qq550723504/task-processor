package local

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/app/taskstatus"
	"task-processor/internal/core/config"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingruntime"
	"task-processor/internal/platformtask"
	"task-processor/internal/product"
)

type LocalRuntime struct {
	provider        *LocalDataProvider
	cookieProvider  SheinCookieProvider
	imageDownloader *ImageDownloader
	insecureImages  bool
}

type LocalRuntimeOptions struct {
	SheinCookieProvider      SheinCookieProvider
	ImageDownloadInsecureTLS bool
}

func NewLocalRuntime(provider *LocalDataProvider, options LocalRuntimeOptions) *LocalRuntime {
	if provider == nil {
		return nil
	}
	return &LocalRuntime{
		provider:       provider,
		cookieProvider: options.SheinCookieProvider,
		insecureImages: options.ImageDownloadInsecureTLS,
	}
}

func NewRedisSheinCookieProvider(cfg *config.RedisConfig) (SheinCookieProvider, error) {
	return newRedisSheinCookieProvider(cfg)
}

func NewLocalStoreAPI(provider *LocalDataProvider, cookieProvider SheinCookieProvider) listingadmin.StoreAPI {
	if provider == nil {
		return nil
	}
	return localStoreAPI{provider: provider, cookieProvider: cookieProvider}
}

func NewLocalProductDataAPI(provider *LocalDataProvider, storeID int64) listingadmin.ProductDataAPI {
	if provider == nil {
		return nil
	}
	return localProductDataAPI{provider: provider, storeID: storeID}
}

func NewLocalProductImportMappingAPI(provider *LocalDataProvider) listingadmin.ProductImportMappingAPI {
	if provider == nil {
		return nil
	}
	return localProductImportMappingAPI{provider: provider}
}

func NewLocalInventoryRecordAPI(provider *LocalDataProvider) listingadmin.InventoryRecordAPI {
	if provider == nil {
		return nil
	}
	return localInventoryRecordAPI{provider: provider}
}

func (r *LocalRuntime) GetRuntimeStoreService() listingruntime.StoreService {
	if r == nil || r.provider == nil {
		return nil
	}
	return localRuntimeStoreService{runtime: r}
}

func (r *LocalRuntime) ListRuntimeAutoPricingStoreIDs(ctx context.Context, platform string) ([]int64, error) {
	if r == nil || r.provider == nil {
		return nil, fmt.Errorf("local listing runtime is not initialized")
	}
	repo := r.provider.StoreRepository()
	if repo == nil {
		return nil, fmt.Errorf("store repository is not configured")
	}

	enableAutoPrice := true
	pageNo := 1
	storeIDs := make([]int64, 0, runtimeStoreDiscoveryPageSize)
	for {
		page, err := repo.ListStores(ctx, listingadmin.StoreQuery{
			Page:            pageNo,
			PageSize:        runtimeStoreDiscoveryPageSize,
			Platform:        platform,
			EnableAutoPrice: &enableAutoPrice,
		})
		if err != nil {
			return nil, err
		}
		if page == nil || len(page.Items) == 0 {
			break
		}

		for _, store := range page.Items {
			if store.ID == 0 || !strings.EqualFold(store.Platform, platform) {
				continue
			}
			storeIDs = append(storeIDs, store.ID)
		}

		if page.Total > 0 && int64(page.Page*page.PageSize) >= page.Total {
			break
		}
		if len(page.Items) < runtimeStoreDiscoveryPageSize {
			break
		}
		pageNo++
	}
	return dedupeInt64s(storeIDs), nil
}

func (r *LocalRuntime) ListRuntimeScheduledTaskConfigs(ctx context.Context, platform string, taskType scheduler.TaskType) ([]listingruntime.ScheduledTaskConfig, error) {
	if r == nil || r.provider == nil {
		return nil, fmt.Errorf("local listing runtime is not initialized")
	}
	repo := r.provider.ScheduledTaskConfigRepository()
	if repo == nil {
		return nil, fmt.Errorf("scheduled task config repository is not configured")
	}
	items, err := repo.ListEnabledScheduledTaskConfigs(ctx, platform, string(taskType))
	if err != nil {
		return nil, err
	}
	result := make([]listingruntime.ScheduledTaskConfig, 0, len(items))
	for _, item := range items {
		result = append(result, listingruntime.ScheduledTaskConfig{
			TenantID:        item.TenantID,
			StoreID:         item.StoreID,
			Platform:        item.Platform,
			TaskType:        item.TaskType,
			Enabled:         item.Enabled,
			IntervalSeconds: item.IntervalSeconds,
		})
	}
	return result, nil
}

func (r *LocalRuntime) ListRuntimeScheduledTaskConfigStates(ctx context.Context, platform string, taskType scheduler.TaskType) ([]listingruntime.ScheduledTaskConfig, error) {
	if r == nil || r.provider == nil {
		return nil, fmt.Errorf("local listing runtime is not initialized")
	}
	repo := r.provider.ScheduledTaskConfigRepository()
	if repo == nil {
		return nil, fmt.Errorf("scheduled task config repository is not configured")
	}

	pageNo := 1
	result := make([]listingruntime.ScheduledTaskConfig, 0)
	for {
		page, err := repo.ListScheduledTaskConfigs(ctx, listingadmin.ScheduledTaskConfigQuery{
			Page:     pageNo,
			PageSize: 500,
			Platform: platform,
			TaskType: string(taskType),
		})
		if err != nil {
			return nil, err
		}
		if page == nil || len(page.Items) == 0 {
			break
		}
		for _, item := range page.Items {
			result = append(result, listingruntime.ScheduledTaskConfig{
				TenantID:        item.TenantID,
				StoreID:         item.StoreID,
				Platform:        item.Platform,
				TaskType:        item.TaskType,
				Enabled:         item.Enabled,
				IntervalSeconds: item.IntervalSeconds,
			})
		}
		if page.Total > 0 && int64(page.Page*page.PageSize) >= page.Total {
			break
		}
		if len(page.Items) < page.PageSize {
			break
		}
		pageNo++
	}
	return result, nil
}

func (r *LocalRuntime) GetAutoPricingStoreConfig(ctx context.Context, storeID int64) (*platformtask.AutoPricingStoreConfig, error) {
	if r == nil || r.provider == nil {
		return nil, fmt.Errorf("local listing runtime is not initialized")
	}
	if storeID <= 0 {
		return nil, errors.New("store id is required")
	}
	repo := r.provider.StoreRepository()
	if repo == nil {
		return nil, fmt.Errorf("store repository is not configured")
	}
	store, err := repo.FindStoreByID(ctx, storeID)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, errors.New("store not found")
	}
	return &platformtask.AutoPricingStoreConfig{
		Name:            store.Name,
		EnableAutoPrice: store.EnableAutoPrice,
		EnableRebargain: store.EnableRebargain,
	}, nil
}

func (r *LocalRuntime) GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error) {
	if r == nil || r.provider == nil || storeID == 0 {
		return nil, nil
	}
	repo := r.provider.OperationStrategyRepository()
	if repo == nil {
		return nil, nil
	}
	strategy, err := repo.GetLatestByStoreID(context.Background(), storeID)
	if err != nil {
		if errors.Is(err, listingadmin.ErrOperationStrategyNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return runtimeOperationStrategyFromListing(strategy), nil
}

func (r *LocalRuntime) GetRuntimeStorePauseStatusDetail(storeID int64) (*listingruntime.StorePauseStatusDetail, error) {
	service := r.GetRuntimeStoreService()
	if service == nil {
		return nil, nil
	}
	return service.GetStorePauseStatusDetail(storeID)
}

func (r *LocalRuntime) SetRuntimeStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error) {
	service := r.GetRuntimeStoreService()
	if service == nil {
		return false, nil
	}
	return service.SetStorePauseStatus(storeID, pause, pauseType)
}

func (r *LocalRuntime) GetRuntimeImportTask(taskID int64) (*listingruntime.ImportTask, error) {
	if r == nil || r.provider == nil {
		return nil, fmt.Errorf("local listing runtime is not initialized")
	}
	item, found, err := r.provider.GetImportTaskByID(taskID)
	if err != nil || !found || item == nil {
		return nil, err
	}
	return item, nil
}

func (r *LocalRuntime) UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error {
	if r == nil || r.provider == nil {
		return fmt.Errorf("local listing runtime is not initialized")
	}
	if req == nil {
		return fmt.Errorf("runtime task status update request is nil")
	}
	_, err := r.provider.UpdateImportTaskStatus(&listingadmin.ImportTaskStatusUpdate{
		ID:                    req.ID,
		Status:                req.Status,
		ErrorMessage:          req.ErrorMessage,
		ReasonCode:            req.ReasonCode,
		Stage:                 req.Stage,
		ExpectedCurrentStatus: req.ExpectedCurrentStatus,
		RetryCount:            req.RetryCount,
		Priority:              req.Priority,
	})
	return err
}

func (r *LocalRuntime) GetTaskStatus(taskID int64) (*taskstatus.TaskStatusSnapshot, error) {
	taskRPC := NewLocalTaskRPCProvider(r.provider)
	if taskRPC == nil {
		return nil, fmt.Errorf("local task rpc provider is not configured")
	}
	status, found, err := taskRPC.GetTaskStatus(taskID)
	if err != nil || !found || status == nil {
		return nil, err
	}
	return taskStatusSnapshotFromDTO(status), nil
}

func (r *LocalRuntime) RuntimePublishedProductExists(ctx context.Context, storeID int64, platform, region, productID string) (bool, error) {
	repo := r.provider.ProductImportMappingRepository()
	if repo == nil {
		return false, nil
	}
	return repo.ExistsPublishedProduct(ctx, storeID, platform, region, productID)
}

func (r *LocalRuntime) FindRuntimeProductImportMappingByTaskAndSKU(ctx context.Context, importTaskID int64, sku string) (*listingruntime.ProductImportMapping, error) {
	repo := r.provider.ProductImportMappingRepository()
	if repo == nil {
		return nil, nil
	}
	mapping, err := repo.FindLatest(ctx, listingadmin.ProductImportMappingQuery{ImportTaskID: &importTaskID, SKU: sku})
	if err != nil {
		return nil, err
	}
	return runtimeProductImportMappingFromListing(mapping), nil
}

func (r *LocalRuntime) CreateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) (int64, error) {
	if req == nil {
		return 0, fmt.Errorf("runtime product import mapping request is nil")
	}
	repo := r.provider.ProductImportMappingRepository()
	if repo == nil {
		return 0, nil
	}
	mapping, err := repo.CreateProductImportMapping(ctx, listingProductImportMappingFromRuntime(req))
	if err != nil || mapping == nil {
		return 0, err
	}
	return mapping.ID, nil
}

func (r *LocalRuntime) UpdateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) error {
	if req == nil {
		return fmt.Errorf("runtime product import mapping request is nil")
	}
	repo := r.provider.ProductImportMappingRepository()
	if repo == nil {
		return nil
	}
	_, err := repo.UpdateProductImportMapping(ctx, listingProductImportMappingFromRuntime(req))
	return err
}

func (r *LocalRuntime) GetSheinCookie(storeID int64) (string, int64, error) {
	if r == nil || r.cookieProvider == nil {
		return "", 0, nil
	}
	result, err := r.cookieProvider.GetCookie(context.Background(), storeID)
	if err != nil || result == nil {
		return "", 0, err
	}
	return result.CookieJSON, result.TenantID, nil
}

func (r *LocalRuntime) GetSheinStoreCookie(storeID int64) (string, error) {
	cookie, _, err := r.GetSheinCookie(storeID)
	return cookie, err
}

func (r *LocalRuntime) DeleteSheinStoreCookie(storeID int64) (bool, error) {
	if r == nil || r.cookieProvider == nil {
		return false, nil
	}
	return r.cookieProvider.DeleteCookie(context.Background(), storeID)
}

func (r *LocalRuntime) GetImageDownloader() interface {
	DownloadImage(url string) ([]byte, error)
} {
	if r == nil {
		return nil
	}
	if r.imageDownloader == nil {
		r.imageDownloader = NewImageDownloader(120*time.Second, r.insecureImages)
	}
	return r.imageDownloader
}

func (r *LocalRuntime) ValidateLocalListingRuntimeFields() (map[string]bool, error) {
	report, err := ValidateLocalListingRuntime(r.provider)
	return report.Fields(), err
}

func (r *LocalRuntime) GetStoreAPI() listingadmin.StoreAPI {
	return NewLocalStoreAPI(r.provider, r.cookieProvider)
}

func (r *LocalRuntime) GetRawJsonDataAdapter() product.RawJsonDataClient {
	if r == nil || r.provider == nil {
		return nil
	}
	return NewRawJsonDataAdapter(r.provider)
}

func (r *LocalRuntime) GetPricingRuleClient() listingadmin.PricingRuleAPI {
	return r.provider
}

func (r *LocalRuntime) GetProductImportMappingAPI() listingadmin.ProductImportMappingAPI {
	return NewLocalProductImportMappingAPI(r.provider)
}

func (r *LocalRuntime) GetInventoryRecordAPI() listingadmin.InventoryRecordAPI {
	return NewLocalInventoryRecordAPI(r.provider)
}

func (r *LocalRuntime) GetProductDataClient(storeID int64) listingadmin.ProductDataAPI {
	return NewLocalProductDataAPI(r.provider, storeID)
}

func (r *LocalRuntime) GetFilterRuleClient() listingadmin.FilterRuleAPI {
	return r.provider
}

func (r *LocalRuntime) GetDailyListingCountClient() listingadmin.DailyListingCountAPI {
	return r.provider
}

func (r *LocalRuntime) GetProfitRuleClient() listingadmin.ProfitRuleAPI {
	return r.provider
}

func (r *LocalRuntime) GetLocalProductDataRepository() listingadmin.ProductDataRepository {
	return r.provider.ProductDataRepository()
}

func (r *LocalRuntime) GetLocalSheinSyncRepository() listingkit.SheinSyncRepository {
	return r.provider.SheinSyncRepository()
}

func (r *LocalRuntime) GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository {
	return r.provider.PricingRuleRepository()
}

func (r *LocalRuntime) GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository {
	return r.provider.ProductImportMappingRepository()
}

func (r *LocalRuntime) GetLocalStoreRepository() *listingadmin.GormStoreRepository {
	return r.provider.StoreRepository()
}

func (r *LocalRuntime) GetLocalInventoryRecordRepository() *listingadmin.GormInventoryRecordRepository {
	return r.provider.InventoryRecordRepository()
}

func (r *LocalRuntime) GetLocalFilterRuleRepository() *listingadmin.GormFilterRuleRepository {
	return r.provider.FilterRuleRepository()
}

func (r *LocalRuntime) GetLocalProfitRuleRepository() *listingadmin.GormProfitRuleRepository {
	return r.provider.ProfitRuleRepository()
}

type localRuntimeStoreService struct {
	runtime *LocalRuntime
}

func (s localRuntimeStoreService) GetStore(storeID int64) (*listingruntime.StoreInfo, error) {
	store, err := s.runtime.provider.StoreRepository().FindStoreByID(context.Background(), storeID)
	if err != nil {
		return nil, err
	}
	return runtimeStoreFromListing(store), nil
}

func (s localRuntimeStoreService) GetStorePauseStatus(storeID int64) (bool, error) {
	return s.runtime.provider.GetStorePauseStatus(storeID)
}

func (s localRuntimeStoreService) GetStorePauseStatusDetail(storeID int64) (*listingruntime.StorePauseStatusDetail, error) {
	detail, err := s.runtime.provider.GetStorePauseStatusDetail(storeID)
	if err != nil {
		return nil, err
	}
	return runtimePauseDetailFromListingAdminDTO(detail), nil
}

func (s localRuntimeStoreService) SetStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error) {
	return s.runtime.provider.SetStorePauseStatus(storeID, pause, pauseType)
}
