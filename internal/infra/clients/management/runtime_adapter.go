package management

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	"task-processor/internal/taskstatus"
)

const runtimeStoreDiscoveryPageSize = 200

type runtimeStoreService struct {
	client *StoreAPIClient
}

func (cm *ClientManager) GetRuntimeStoreService() listingruntime.StoreService {
	if cm == nil {
		return nil
	}
	storeClient := cm.GetStoreClient()
	if storeClient == nil {
		return nil
	}
	return runtimeStoreService{client: storeClient}
}

func (cm *ClientManager) ListRuntimeAutoPricingStoreIDs(ctx context.Context, platform string) ([]int64, error) {
	if cm == nil {
		return nil, fmt.Errorf("management client is not initialized")
	}

	enableAutoPrice := true
	if repo := cm.GetLocalStoreRepository(); repo != nil {
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

	storeClient := cm.GetStoreClient()
	if storeClient == nil {
		return nil, fmt.Errorf("store client is not initialized")
	}

	pageNo := 1
	storeIDs := make([]int64, 0, runtimeStoreDiscoveryPageSize)
	for {
		page, err := storeClient.PageStores(&managementapi.StorePageReqDTO{
			PageNo:          pageNo,
			PageSize:        runtimeStoreDiscoveryPageSize,
			Platform:        platform,
			EnableAutoPrice: &enableAutoPrice,
		})
		if err != nil {
			return nil, err
		}
		if page == nil || len(page.List) == 0 {
			break
		}

		for _, store := range page.List {
			if store == nil || store.ID == 0 || !strings.EqualFold(store.Platform, platform) {
				continue
			}
			storeIDs = append(storeIDs, store.ID)
		}

		if page.Total > 0 && int64(page.PageNo*page.PageSize) >= page.Total {
			break
		}
		if len(page.List) < runtimeStoreDiscoveryPageSize {
			break
		}
		pageNo++
	}

	return dedupeInt64s(storeIDs), nil
}

func (cm *ClientManager) GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error) {
	if cm == nil || storeID == 0 {
		return nil, nil
	}
	if repo := cm.GetLocalOperationStrategyRepository(); repo != nil {
		strategy, err := repo.GetLatestByStoreID(context.Background(), storeID)
		if err == nil && strategy != nil {
			return runtimeOperationStrategyFromListing(strategy), nil
		}
		if err != nil && !errors.Is(err, listingadmin.ErrOperationStrategyNotFound) {
			return nil, err
		}
	}

	strategyClient := cm.GetOperationStrategyClient()
	if strategyClient == nil {
		return nil, nil
	}
	strategy, err := strategyClient.GetOperationStrategyByStoreId(storeID)
	if err != nil {
		return nil, err
	}
	return runtimeOperationStrategyFromManagement(strategy), nil
}

func (cm *ClientManager) GetRuntimeStorePauseStatusDetail(storeID int64) (*listingruntime.StorePauseStatusDetail, error) {
	storeService := cm.GetRuntimeStoreService()
	if storeService == nil {
		return nil, nil
	}
	return storeService.GetStorePauseStatusDetail(storeID)
}

func (cm *ClientManager) SetRuntimeStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error) {
	storeService := cm.GetRuntimeStoreService()
	if storeService == nil {
		return false, nil
	}
	return storeService.SetStorePauseStatus(storeID, pause, pauseType)
}

func (cm *ClientManager) GetPendingRuntimeTasks(maxTasks int, userID int64, storeIDs []int64) ([]listingruntime.ImportTask, error) {
	if cm == nil {
		return nil, fmt.Errorf("management client is not initialized")
	}

	importTaskClient := cm.GetImportTaskClient()
	if importTaskClient == nil {
		return nil, fmt.Errorf("import task client is not initialized")
	}

	items, err := importTaskClient.GetPendingAndRetryTasks(maxTasks, userID, storeIDs)
	if err != nil {
		return nil, err
	}

	tasks := make([]listingruntime.ImportTask, 0, len(items))
	for _, item := range items {
		tasks = append(tasks, runtimeImportTaskFromManagement(item))
	}
	return tasks, nil
}

func (cm *ClientManager) GetRuntimeImportTask(taskID int64) (*listingruntime.ImportTask, error) {
	if cm == nil {
		return nil, fmt.Errorf("management client is not initialized")
	}

	importTaskClient := cm.GetImportTaskClient()
	if importTaskClient == nil {
		return nil, fmt.Errorf("import task client is not initialized")
	}

	item, err := importTaskClient.GetTaskByID(taskID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("import task %d not found", taskID)
	}

	task := runtimeImportTaskFromManagement(*item)
	return &task, nil
}

func (cm *ClientManager) UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error {
	if cm == nil {
		return fmt.Errorf("management client is not initialized")
	}
	if req == nil {
		return fmt.Errorf("runtime task status update request is nil")
	}

	importTaskClient := cm.GetImportTaskClient()
	if importTaskClient == nil {
		return fmt.Errorf("import task client is not initialized")
	}

	updateReq := &managementapi.ProductImportTaskUpdateReqDTO{
		ID:                    req.ID,
		Status:                req.Status,
		ErrorMessage:          req.ErrorMessage,
		ReasonCode:            req.ReasonCode,
		Stage:                 req.Stage,
		ExpectedCurrentStatus: req.ExpectedCurrentStatus,
		RetryCount:            req.RetryCount,
		Priority:              req.Priority,
	}
	return importTaskClient.UpdateTaskStatus(updateReq)
}

func (cm *ClientManager) GetTaskStatus(taskID int64) (*taskstatus.TaskStatusSnapshot, error) {
	if cm == nil {
		return nil, fmt.Errorf("management client is not initialized")
	}
	taskRPCClient := cm.GetTaskRPCClient()
	if taskRPCClient == nil {
		return nil, fmt.Errorf("task rpc client is not initialized")
	}
	status, err := taskRPCClient.GetTaskStatus(taskID)
	if err != nil || status == nil {
		return nil, err
	}
	return taskStatusSnapshotFromDTO(status), nil
}

func (cm *ClientManager) RuntimePublishedProductExists(ctx context.Context, storeID int64, platform, region, productID string) (bool, error) {
	if cm == nil {
		return false, fmt.Errorf("management client is not initialized")
	}
	if repo := cm.GetLocalProductImportMappingRepository(); repo != nil {
		return repo.ExistsPublishedProduct(ctx, storeID, platform, region, productID)
	}

	mappingClient := cm.GetProductImportMappingClient()
	if mappingClient == nil {
		return false, fmt.Errorf("product import mapping client is not initialized")
	}
	return mappingClient.CheckProductExists(&managementapi.ProductImportMappingCheckReqDTO{
		StoreId:   storeID,
		Platform:  platform,
		Region:    region,
		ProductId: productID,
	})
}

func (cm *ClientManager) FindRuntimeProductImportMappingByTaskAndSKU(ctx context.Context, importTaskID int64, sku string) (*listingruntime.ProductImportMapping, error) {
	if cm == nil {
		return nil, fmt.Errorf("management client is not initialized")
	}
	if repo := cm.GetLocalProductImportMappingRepository(); repo != nil {
		mapping, err := repo.FindLatest(ctx, listingadmin.ProductImportMappingQuery{
			ImportTaskID: &importTaskID,
			SKU:          sku,
		})
		if err != nil {
			return nil, err
		}
		return runtimeProductImportMappingFromListing(mapping), nil
	}

	mappingClient := cm.GetProductImportMappingClient()
	if mappingClient == nil {
		return nil, fmt.Errorf("product import mapping client is not initialized")
	}
	mapping, err := mappingClient.GetProductImportMappingByTaskAndSku(importTaskID, sku)
	if err != nil {
		return nil, err
	}
	return runtimeProductImportMappingFromManagement(mapping), nil
}

func (cm *ClientManager) FindRuntimeProductImportMappingByPlatformProductID(ctx context.Context, platformProductID string, storeID int64) (*listingruntime.ProductImportMapping, error) {
	if cm == nil {
		return nil, fmt.Errorf("management client is not initialized")
	}
	if repo := cm.GetLocalProductImportMappingRepository(); repo != nil {
		mapping, err := repo.FindLatest(ctx, listingadmin.ProductImportMappingQuery{
			PlatformProductID: platformProductID,
			StoreID:           &storeID,
		})
		if err != nil {
			return nil, err
		}
		return runtimeProductImportMappingFromListing(mapping), nil
	}

	mappingClient := cm.GetProductImportMappingClient()
	if mappingClient == nil {
		return nil, fmt.Errorf("product import mapping client is not initialized")
	}
	mapping, err := mappingClient.GetProductImportMappingByPlatformProductIdAndStore(&managementapi.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO{
		PlatformProductId: platformProductID,
		StoreId:           storeID,
	})
	if err != nil {
		return nil, err
	}
	return runtimeProductImportMappingFromManagement(mapping), nil
}

func (cm *ClientManager) CreateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) (int64, error) {
	if cm == nil {
		return 0, fmt.Errorf("management client is not initialized")
	}
	if req == nil {
		return 0, fmt.Errorf("runtime product import mapping request is nil")
	}
	if repo := cm.GetLocalProductImportMappingRepository(); repo != nil {
		mapping, err := repo.CreateProductImportMapping(ctx, listingProductImportMappingFromRuntime(req))
		if err != nil {
			return 0, err
		}
		if mapping == nil {
			return 0, nil
		}
		return mapping.ID, nil
	}

	mappingClient := cm.GetProductImportMappingClient()
	if mappingClient == nil {
		return 0, fmt.Errorf("product import mapping client is not initialized")
	}
	return mappingClient.CreateProductImportMapping(managementProductImportMappingFromRuntime(req))
}

func (cm *ClientManager) UpdateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) error {
	if cm == nil {
		return fmt.Errorf("management client is not initialized")
	}
	if req == nil {
		return fmt.Errorf("runtime product import mapping request is nil")
	}
	if repo := cm.GetLocalProductImportMappingRepository(); repo != nil {
		_, err := repo.UpdateProductImportMapping(ctx, listingProductImportMappingFromRuntime(req))
		return err
	}

	mappingClient := cm.GetProductImportMappingClient()
	if mappingClient == nil {
		return fmt.Errorf("product import mapping client is not initialized")
	}
	return mappingClient.UpdateProductImportMapping(managementProductImportMappingFromRuntime(req))
}

func (s runtimeStoreService) GetStore(storeID int64) (*listingruntime.StoreInfo, error) {
	if s.client == nil {
		return nil, fmt.Errorf("store client is not initialized")
	}
	store, err := s.client.GetStore(storeID)
	if err != nil {
		return nil, err
	}
	return runtimeStoreFromManagement(store), nil
}

func (s runtimeStoreService) GetStorePauseStatus(storeID int64) (bool, error) {
	if s.client == nil {
		return false, fmt.Errorf("store client is not initialized")
	}
	return s.client.GetStorePauseStatus(storeID)
}

func (s runtimeStoreService) GetStorePauseStatusDetail(storeID int64) (*listingruntime.StorePauseStatusDetail, error) {
	if s.client == nil {
		return nil, fmt.Errorf("store client is not initialized")
	}
	detail, err := s.client.GetStorePauseStatusDetail(storeID)
	if err != nil {
		return nil, err
	}
	return runtimePauseDetailFromManagement(detail), nil
}

func (s runtimeStoreService) SetStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error) {
	if s.client == nil {
		return false, fmt.Errorf("store client is not initialized")
	}
	return s.client.SetStorePauseStatus(storeID, pause, pauseType)
}

func runtimeImportTaskFromManagement(task managementapi.ProductImportTaskRespDTO) listingruntime.ImportTask {
	return listingruntime.ImportTask{
		ID:              task.ID,
		TenantID:        task.TenantID,
		StoreID:         task.StoreID,
		Platform:        task.Platform,
		Region:          task.Region,
		CategoryID:      task.CategoryID,
		ProductID:       task.ProductID,
		Status:          task.Status,
		ErrorMessage:    task.ErrorMessage,
		RetryCount:      task.RetryCount,
		MaxRetryCount:   task.MaxRetryCount,
		Priority:        task.Priority,
		CreateTime:      task.CreateTime,
		PublishedTime:   task.PublishedTime,
		Creator:         task.Creator,
		StatusKey:       task.StatusKey,
		CanonicalStatus: task.CanonicalStatus,
	}
}

func runtimeStoreFromListing(store *listingadmin.Store) *listingruntime.StoreInfo {
	if store == nil {
		return nil
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
	}
}

func runtimeStoreFromManagement(store *managementapi.StoreRespDTO) *listingruntime.StoreInfo {
	if store == nil {
		return nil
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
		LoginURL:                 store.LoginUrl,
		Proxy:                    store.Proxy,
		PriceType:                store.PriceType,
		DailyLimit:               store.DailyLimit,
		DailyLimitType:           store.DailyLimitType,
		EnableDraft:              store.EnableDraft,
		EnableAutoListing:        store.EnableAutoListing,
		FixedStockCount:          store.FixedStockCount,
		SkuGenerateStrategy:      store.SkuGenerateStrategy,
		Prefix:                   store.Prefix,
		Suffix:                   store.Suffix,
		EnableBrandAuthorization: store.EnableBrandAuthorization,
		AuthorizedBrandCode:      store.AuthorizedBrandCode,
		AuthorizedBrandName:      store.AuthorizedBrandName,
	}
}

func runtimePauseDetailFromManagement(detail *managementapi.StorePauseStatusRespDTO) *listingruntime.StorePauseStatusDetail {
	if detail == nil {
		return nil
	}
	return &listingruntime.StorePauseStatusDetail{
		Paused:     detail.Paused,
		PauseType:  detail.PauseType,
		Reason:     detail.Reason,
		TTLSeconds: detail.TTLSeconds,
	}
}

func runtimeOperationStrategyFromListing(strategy *listingadmin.OperationStrategy) *listingruntime.OperationStrategy {
	if strategy == nil {
		return nil
	}
	return &listingruntime.OperationStrategy{
		ID:                           strategy.ID,
		TenantID:                     strategy.TenantID,
		StoreID:                      strategy.StoreID,
		Name:                         strategy.Name,
		Platform:                     strategy.Platform,
		Status:                       strategy.Status,
		StockChangeThreshold:         runtimeIntValue(strategy.StockChangeThreshold),
		StockChangeAction:            strategy.StockChangeAction,
		OutOfStockAction:             strategy.OutOfStockAction,
		MinProfitRate:                runtimeFloat64Value(strategy.MinProfitRate),
		LowProfitAction:              strategy.LowProfitAction,
		PriceUpdateMultiplier:        runtimeFloat64Value(strategy.PriceUpdateMultiplier),
		StockUpdateRatio:             runtimeFloat64Value(strategy.StockUpdateRatio),
		ActivityEnabled:              strategy.ActivityEnabled,
		ActivityType:                 strategy.ActivityType,
		ActivityDiscountRate:         runtimeFloat64Value(strategy.ActivityDiscountRate),
		ActivityStockRatio:           runtimeFloat64Value(strategy.ActivityStockRatio),
		PromotionRatio:               runtimeFloat64Value(strategy.PromotionRatio),
		ActivityMinProfitRate:        runtimeFloat64Value(strategy.ActivityMinProfitRate),
		ActivityPriceMode:            strategy.ActivityPriceMode,
		TimeLimitedDiscountRate:      runtimeFloat64Value(strategy.TimeLimitedDiscountRate),
		TimeLimitedMinProfitRate:     runtimeFloat64Value(strategy.TimeLimitedMinProfitRate),
		TimeLimitedPriceMode:         strategy.TimeLimitedPriceMode,
		TimeLimitedUserLimit:         strategy.TimeLimitedUserLimit,
		TimeLimitedUserLimitNum:      runtimeIntValue(strategy.TimeLimitedUserLimitNum),
		TimeLimitedStockLimit:        strategy.TimeLimitedStockLimit,
		TimeLimitedStockLimitPercent: runtimeIntValue(strategy.TimeLimitedStockLimitPercent),
		FixedPriceAdjustment:         runtimeFloat64Value(strategy.FixedPriceAdjustment),
		PriceIncreaseThreshold:       runtimeFloat64Value(strategy.PriceIncreaseThreshold),
		PriceDecreaseThreshold:       runtimeFloat64Value(strategy.PriceDecreaseThreshold),
		PriceIncreaseAction:          strategy.PriceIncreaseAction,
		PriceDecreaseAction:          strategy.PriceDecreaseAction,
		RestoreStockAmount:           runtimeIntValue(strategy.RestoreStockAmount),
		Remark:                       strategy.Remark,
	}
}

func runtimeOperationStrategyFromManagement(strategy *managementapi.OperationStrategyDTO) *listingruntime.OperationStrategy {
	if strategy == nil {
		return nil
	}
	return &listingruntime.OperationStrategy{
		ID:                           strategy.ID,
		TenantID:                     strategy.TenantID,
		StoreID:                      strategy.StoreID,
		Name:                         strategy.Name,
		Platform:                     strategy.Platform,
		Status:                       strategy.Status,
		StockChangeThreshold:         strategy.StockChangeThreshold,
		StockChangeAction:            strategy.StockChangeAction,
		OutOfStockAction:             strategy.OutOfStockAction,
		MinProfitRate:                strategy.MinProfitRate,
		LowProfitAction:              strategy.LowProfitAction,
		PriceUpdateMultiplier:        strategy.PriceUpdateMultiplier,
		StockUpdateRatio:             strategy.StockUpdateRatio,
		ActivityEnabled:              strategy.ActivityEnabled,
		ActivityType:                 strategy.ActivityType,
		ActivityDiscountRate:         strategy.ActivityDiscountRate,
		ActivityStockRatio:           strategy.ActivityStockRatio,
		PromotionRatio:               strategy.PromotionRatio,
		ActivityMinProfitRate:        strategy.ActivityMinProfitRate,
		ActivityPriceMode:            strategy.ActivityPriceMode,
		TimeLimitedDiscountRate:      strategy.TimeLimitedDiscountRate,
		TimeLimitedMinProfitRate:     strategy.TimeLimitedMinProfitRate,
		TimeLimitedPriceMode:         strategy.TimeLimitedPriceMode,
		TimeLimitedUserLimit:         strategy.TimeLimitedUserLimit,
		TimeLimitedUserLimitNum:      strategy.TimeLimitedUserLimitNum,
		TimeLimitedStockLimit:        strategy.TimeLimitedStockLimit,
		TimeLimitedStockLimitPercent: strategy.TimeLimitedStockLimitPercent,
		FixedPriceAdjustment:         strategy.FixedPriceAdjustment,
		PriceIncreaseThreshold:       strategy.PriceIncreaseThreshold,
		PriceDecreaseThreshold:       strategy.PriceDecreaseThreshold,
		PriceIncreaseAction:          strategy.PriceIncreaseAction,
		PriceDecreaseAction:          strategy.PriceDecreaseAction,
		RestoreStockAmount:           strategy.RestoreStockAmount,
		Remark:                       strategy.Remark,
	}
}

func runtimeProductImportMappingFromListing(mapping *listingadmin.ProductImportMapping) *listingruntime.ProductImportMapping {
	if mapping == nil {
		return nil
	}
	return &listingruntime.ProductImportMapping{
		ID:                      mapping.ID,
		ImportTaskID:            mapping.ImportTaskID,
		StoreID:                 mapping.StoreID,
		Platform:                mapping.Platform,
		Region:                  mapping.Region,
		ProductID:               mapping.ProductID,
		ParentProductID:         runtimeStringPtr(mapping.ParentProductID),
		SKU:                     runtimeStringPtr(mapping.SKU),
		PlatformProductID:       runtimeStringPtr(mapping.PlatformProductID),
		PlatformParentProductID: runtimeStringPtr(mapping.PlatformParentProductID),
		CostPrice:               runtimeFloat64Value(mapping.CostPrice),
		FilterRuleID:            runtimeInt64Value(mapping.FilterRuleID),
		FilterRuleRange:         runtimeStringPtr(mapping.FilterRuleRange),
		ProfitRuleID:            runtimeInt64Value(mapping.ProfitRuleID),
		SalePriceMultiplier:     runtimeFloat64Ptr(mapping.SalePriceMultiplier),
		DiscountPriceMultiplier: runtimeFloat64Ptr(mapping.DiscountPriceMultiplier),
		Status:                  mapping.Status,
		Remark:                  runtimeStringPtr(mapping.Remark),
		TenantID:                mapping.TenantID,
	}
}

func runtimeProductImportMappingFromManagement(mapping *managementapi.ProductImportMappingRespDTO) *listingruntime.ProductImportMapping {
	if mapping == nil {
		return nil
	}
	return &listingruntime.ProductImportMapping{
		ID:                      mapping.ID,
		ImportTaskID:            mapping.ImportTaskId,
		StoreID:                 mapping.StoreId,
		Platform:                mapping.Platform,
		Region:                  mapping.Region,
		ProductID:               mapping.ProductId,
		ParentProductID:         mapping.ParentProductId,
		SKU:                     mapping.Sku,
		PlatformProductID:       mapping.PlatformProductId,
		PlatformParentProductID: mapping.PlatformParentProductId,
		CostPrice:               runtimeFloat64Value(mapping.CostPrice),
		FilterRuleID:            runtimeInt64Value(mapping.FilterRuleId),
		FilterRuleRange:         mapping.FilterRuleRange,
		ProfitRuleID:            runtimeInt64Value(mapping.ProfitRuleId),
		SalePriceMultiplier:     mapping.SalePriceMultiplier,
		DiscountPriceMultiplier: mapping.DiscountPriceMultiplier,
		Status:                  mapping.Status,
		Remark:                  mapping.Remark,
		TenantID:                mapping.TenantId,
	}
}

func listingProductImportMappingFromRuntime(req *listingruntime.ProductImportMappingUpsert) *listingadmin.ProductImportMapping {
	if req == nil {
		return nil
	}
	return &listingadmin.ProductImportMapping{
		ID:                      runtimeInt64Value(req.ID),
		TenantID:                req.TenantID,
		ImportTaskID:            req.ImportTaskID,
		StoreID:                 req.StoreID,
		Platform:                req.Platform,
		Region:                  req.Region,
		ProductID:               req.ProductID,
		ParentProductID:         runtimeStringValue(req.ParentProductID),
		SKU:                     runtimeStringValue(req.SKU),
		CostPrice:               req.CostPrice,
		PlatformProductID:       runtimeStringValue(req.PlatformProductID),
		PlatformParentProductID: runtimeStringValue(req.PlatformParentProductID),
		FilterRuleID:            runtimePositiveInt64Ptr(req.FilterRuleID),
		FilterRuleRange:         runtimeStringValue(req.FilterRuleRange),
		ProfitRuleID:            runtimePositiveInt64Ptr(req.ProfitRuleID),
		SalePriceMultiplier:     runtimeFloat64Value(req.SalePriceMultiplier),
		DiscountPriceMultiplier: runtimeFloat64Value(req.DiscountPriceMultiplier),
		Status:                  runtimeInt16Value(req.Status),
		Remark:                  runtimeStringValue(req.Remark),
	}
}

func managementProductImportMappingFromRuntime(req *listingruntime.ProductImportMappingUpsert) *managementapi.ProductImportMappingCreateReqDTO {
	if req == nil {
		return nil
	}
	return &managementapi.ProductImportMappingCreateReqDTO{
		ID:                      req.ID,
		TenantID:                req.TenantID,
		ImportTaskId:            req.ImportTaskID,
		StoreId:                 req.StoreID,
		Platform:                req.Platform,
		Region:                  req.Region,
		ProductId:               req.ProductID,
		Sku:                     req.SKU,
		CostPrice:               req.CostPrice,
		PlatformProductId:       req.PlatformProductID,
		ProfitRuleId:            req.ProfitRuleID,
		SalePriceMultiplier:     runtimeFormatFloat(req.SalePriceMultiplier),
		DiscountPriceMultiplier: runtimeFormatFloat(req.DiscountPriceMultiplier),
		Status:                  req.Status,
		Remark:                  req.Remark,
		ParentProductId:         req.ParentProductID,
		PlatformParentProductId: req.PlatformParentProductID,
		FilterRuleId:            req.FilterRuleID,
		FilterRuleRange:         req.FilterRuleRange,
	}
}

func runtimeStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	out := value
	return &out
}

func runtimeStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func runtimeFloat64Ptr(value float64) *float64 {
	if value == 0 {
		return nil
	}
	out := value
	return &out
}

func runtimeFloat64Value(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func runtimePositiveInt64Ptr(value *int64) *int64 {
	if value == nil || *value <= 0 {
		return nil
	}
	out := *value
	return &out
}

func runtimeInt64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func runtimeIntValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func runtimeInt16Value(value *int16) int16 {
	if value == nil {
		return 0
	}
	return *value
}

func runtimeFormatFloat(value *float64) *string {
	if value == nil {
		return nil
	}
	out := strconv.FormatFloat(*value, 'f', -1, 64)
	return &out
}

func dedupeInt64s(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
