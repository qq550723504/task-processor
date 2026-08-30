package local

import (
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingruntime"
	"task-processor/internal/pkg/types"
)

const (
	localStoreStatusEnabled = 0
)

type LocalDataProvider struct {
	*RuntimeResources
}

// NewLocalDataProvider is a compatibility constructor. New production code
// should construct RuntimeResources at the composition boundary instead.
func NewLocalDataProvider(dbCfg *config.DatabaseConfig, redisCfg *config.RedisConfig) (*LocalDataProvider, error) {
	resources, err := NewRuntimeResourcesFromConfig(dbCfg, redisCfg)
	if err != nil || resources == nil {
		return nil, err
	}
	return NewLocalDataProviderFromResources(resources), nil
}

// NewLocalDataProviderFromResources preserves legacy API implementations while
// RuntimeResources owns the underlying infrastructure lifecycle.
func NewLocalDataProviderFromResources(resources *RuntimeResources) *LocalDataProvider {
	if resources == nil {
		return nil
	}
	return &LocalDataProvider{RuntimeResources: resources}
}

func (p *LocalDataProvider) Close() error {
	if p == nil {
		return nil
	}
	return p.RuntimeResources.Close()
}

func (p *LocalDataProvider) HasDB() bool {
	return p != nil && p.RuntimeResources.HasDB()
}

func (p *LocalDataProvider) HasRedis() bool {
	return p != nil && p.RuntimeResources.HasRedis()
}

func (p *LocalDataProvider) initRepositories() {
	if p != nil {
		p.RuntimeResources.initRepositories()
	}
}

func (p *LocalDataProvider) storeRepository() *listingadmin.GormStoreRepository {
	if p == nil {
		return nil
	}
	p.initRepositories()
	return p.storeRepo
}

func (p *LocalDataProvider) StoreRepository() *listingadmin.GormStoreRepository {
	return p.storeRepository()
}

func (p *LocalDataProvider) filterRuleRepository() *listingadmin.GormFilterRuleRepository {
	if p == nil {
		return nil
	}
	p.initRepositories()
	return p.filterRuleRepo
}

func (p *LocalDataProvider) FilterRuleRepository() *listingadmin.GormFilterRuleRepository {
	return p.filterRuleRepository()
}

func (p *LocalDataProvider) profitRuleRepository() *listingadmin.GormProfitRuleRepository {
	if p == nil {
		return nil
	}
	p.initRepositories()
	return p.profitRuleRepo
}

func (p *LocalDataProvider) ProfitRuleRepository() *listingadmin.GormProfitRuleRepository {
	return p.profitRuleRepository()
}

func (p *LocalDataProvider) operationStrategyRepository() *listingadmin.GormOperationStrategyRepository {
	if p == nil {
		return nil
	}
	p.initRepositories()
	return p.operationStrategyRepo
}

func (p *LocalDataProvider) OperationStrategyRepository() *listingadmin.GormOperationStrategyRepository {
	return p.operationStrategyRepository()
}

func (p *LocalDataProvider) scheduledTaskConfigRepository() *listingadmin.GormScheduledTaskConfigRepository {
	if p == nil {
		return nil
	}
	p.initRepositories()
	return p.scheduledTaskConfigRepo
}

func (p *LocalDataProvider) ScheduledTaskConfigRepository() *listingadmin.GormScheduledTaskConfigRepository {
	return p.scheduledTaskConfigRepository()
}

func (p *LocalDataProvider) pricingRuleRepository() *listingadmin.GormPricingRuleRepository {
	if p == nil {
		return nil
	}
	p.initRepositories()
	return p.pricingRuleRepo
}

func (p *LocalDataProvider) PricingRuleRepository() *listingadmin.GormPricingRuleRepository {
	return p.pricingRuleRepository()
}

func (p *LocalDataProvider) productImportMappingRepository() *listingadmin.GormProductImportMappingRepository {
	if p == nil {
		return nil
	}
	p.initRepositories()
	return p.productImportMappingRepo
}

func (p *LocalDataProvider) ProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository {
	return p.productImportMappingRepository()
}

func (p *LocalDataProvider) productDataRepository() *listingadmin.GormProductDataRepository {
	if p == nil {
		return nil
	}
	p.initRepositories()
	return p.productDataRepo
}

func (p *LocalDataProvider) ProductDataRepository() listingadmin.ProductDataRepository {
	return p.productDataRepository()
}

func (p *LocalDataProvider) SheinSyncRepository() listingkit.SheinSyncRepository {
	if p == nil {
		return nil
	}
	p.initRepositories()
	return p.sheinSyncRepo
}

func (p *LocalDataProvider) inventoryRecordRepository() *listingadmin.GormInventoryRecordRepository {
	if p == nil {
		return nil
	}
	p.initRepositories()
	return p.inventoryRecordRepo
}

func (p *LocalDataProvider) InventoryRecordRepository() *listingadmin.GormInventoryRecordRepository {
	return p.inventoryRecordRepository()
}

func (p *LocalDataProvider) rawJSONDataRepository() *listingadmin.GormRawJSONDataRepository {
	if p == nil {
		return nil
	}
	p.initRepositories()
	return p.rawJSONDataRepo
}

func (p *LocalDataProvider) RawJSONDataRepository() *listingadmin.GormRawJSONDataRepository {
	return p.rawJSONDataRepository()
}

func (p *LocalDataProvider) importTaskRepository() *listingadmin.GormImportTaskRepository {
	if p == nil {
		return nil
	}
	p.initRepositories()
	return p.importTaskRepo
}

func (p *LocalDataProvider) ImportTaskRepository() *listingadmin.GormImportTaskRepository {
	return p.importTaskRepository()
}

type localListingStore struct {
	ID                       int64      `gorm:"column:id"`
	TenantID                 int64      `gorm:"column:tenant_id"`
	OwnerUserID              string     `gorm:"column:owner_user_id"`
	StoreID                  string     `gorm:"column:store_id"`
	Name                     string     `gorm:"column:name"`
	Username                 string     `gorm:"column:username"`
	Password                 string     `gorm:"column:password"`
	LoginURL                 string     `gorm:"column:login_url"`
	ShopType                 string     `gorm:"column:shop_type"`
	Region                   string     `gorm:"column:region"`
	Platform                 string     `gorm:"column:platform"`
	DailyLimit               *int       `gorm:"column:daily_limit"`
	DailyLimitType           string     `gorm:"column:daily_limit_type"`
	FixedStockCount          *int       `gorm:"column:fixed_stock_count"`
	SKUGenerateStrategy      string     `gorm:"column:sku_generate_strategy"`
	Prefix                   string     `gorm:"column:prefix"`
	Suffix                   string     `gorm:"column:suffix"`
	Proxy                    string     `gorm:"column:proxy"`
	EnableAutoListing        *bool      `gorm:"column:enable_auto_listing"`
	EnableAutoLogin          *bool      `gorm:"column:enable_auto_login"`
	EnableDraft              *bool      `gorm:"column:enable_draft"`
	EnableAutoPrice          *bool      `gorm:"column:enable_auto_price"`
	DedicatedQueueEnabled    *bool      `gorm:"column:dedicated_queue_enabled"`
	EnableRebargain          *bool      `gorm:"column:enable_rebargain"`
	EnableBrandAuthorization *bool      `gorm:"column:enable_brand_authorization"`
	AuthorizedBrandCode      string     `gorm:"column:authorized_brand_code"`
	AuthorizedBrandName      string     `gorm:"column:authorized_brand_name"`
	TemuPriceRejectStrategy  string     `gorm:"column:temu_price_reject_strategy"`
	PriceType                string     `gorm:"column:price_type"`
	Remark                   string     `gorm:"column:remark"`
	Status                   int16      `gorm:"column:status"`
	Deleted                  int16      `gorm:"column:deleted"`
	ValidFrom                *time.Time `gorm:"column:valid_from"`
	ValidUntil               *time.Time `gorm:"column:valid_until"`
	CreateTime               *time.Time `gorm:"column:create_time"`
	Creator                  string     `gorm:"column:creator"`
}

func (s localListingStore) toDTO() *listingadmin.StoreRespDTO {
	return &listingadmin.StoreRespDTO{
		ID:                       s.ID,
		TenantID:                 s.TenantID,
		OwnerUserID:              s.OwnerUserID,
		StoreID:                  s.StoreID,
		Name:                     s.Name,
		Username:                 s.Username,
		Password:                 s.Password,
		LoginUrl:                 s.LoginURL,
		ShopType:                 s.ShopType,
		Region:                   s.Region,
		Platform:                 s.Platform,
		DailyLimit:               s.DailyLimit,
		DailyLimitType:           s.DailyLimitType,
		FixedStockCount:          s.FixedStockCount,
		SkuGenerateStrategy:      s.SKUGenerateStrategy,
		Prefix:                   s.Prefix,
		Suffix:                   s.Suffix,
		Proxy:                    s.Proxy,
		EnableAutoListing:        s.EnableAutoListing,
		EnableAutoLogin:          s.EnableAutoLogin,
		EnableDraft:              s.EnableDraft,
		EnableAutoPrice:          s.EnableAutoPrice,
		DedicatedQueueEnabled:    s.DedicatedQueueEnabled,
		EnableRebargain:          s.EnableRebargain,
		EnableBrandAuthorization: s.EnableBrandAuthorization,
		AuthorizedBrandCode:      s.AuthorizedBrandCode,
		AuthorizedBrandName:      s.AuthorizedBrandName,
		TemuPriceRejectStrategy:  s.TemuPriceRejectStrategy,
		PriceType:                s.PriceType,
		Remark:                   s.Remark,
		Status:                   s.Status,
		CreateTime:               types.ToFlexibleTime(s.CreateTime),
		Creator:                  s.Creator,
	}
}

func productImportMappingToDTO(mapping *listingadmin.ProductImportMapping) *listingadmin.ProductImportMappingRespDTO {
	return listingadmin.ProductImportMappingToRespDTO(mapping)
}

func ptrFloat64(value float64) *float64 {
	v := value
	return &v
}

func ptrInt(value int) *int {
	v := value
	return &v
}

func ptrString(value string) *string {
	v := value
	return &v
}

func int64FromPtr(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func ptrInt64(value int64) *int64 {
	v := value
	return &v
}

func intFromPtr(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func float64FromPtr(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func timeToUnixMillis(value *time.Time) int64 {
	if value == nil || value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}

func flexibleTimeValue(value *time.Time) types.FlexibleTime {
	if value == nil {
		return types.FlexibleTime{}
	}
	return types.FlexibleTime{Time: *value}
}

func flexibleStringValue(value *time.Time) types.FlexibleString {
	if value == nil {
		return ""
	}
	return types.FlexibleString(value.Format(time.RFC3339))
}

func (p *LocalDataProvider) GetStore(id int64) (*listingadmin.StoreRespDTO, error) {
	api := p.storeAPI()
	if api == nil {
		return nil, nil
	}
	return api.GetStore(id)
}

func (p *LocalDataProvider) PageStores(req *listingadmin.StorePageReqDTO) (*listingadmin.PageResult[*listingadmin.StoreRespDTO], error) {
	api := p.storeAPI()
	if api == nil {
		return nil, nil
	}
	return api.PageStores(req)
}

func (p *LocalDataProvider) UpdateStoreID(id int64, storeID string) (bool, error) {
	api := p.storeAPI()
	if api == nil {
		return false, nil
	}
	return api.UpdateStoreId(&listingadmin.StoreIdUpdateReqDTO{ID: id, StoreID: storeID})
}

func (p *LocalDataProvider) UpdateStoreStatus(id int64, status int16, remark string) (bool, error) {
	api := p.storeAPI()
	if api == nil {
		return false, nil
	}
	return api.UpdateStoreStatus(&listingadmin.StoreStatusUpdateReqDTO{ID: id, Status: status, Remark: remark})
}

func (p *LocalDataProvider) storeAPI() listingadmin.StoreAPI {
	if p == nil {
		return nil
	}
	return listingadmin.NewGormStoreAPI(p.StoreRepository())
}

func (p *LocalDataProvider) DeleteStoreCookie(id int64) (bool, error) {
	state := p.storeRuntimeState()
	if state == nil {
		return false, nil
	}
	return state.DeleteStoreCookie(id)
}

func (p *LocalDataProvider) GetStorePauseStatus(id int64) (bool, error) {
	state := p.storeRuntimeState()
	if state == nil {
		return false, nil
	}
	return state.GetStorePauseStatus(id)
}

func (p *LocalDataProvider) GetStorePauseStatusDetail(id int64) (*listingadmin.StorePauseStatusRespDTO, error) {
	state := p.storeRuntimeState()
	if state == nil {
		return nil, nil
	}
	return state.GetStorePauseStatusDetail(id)
}

func (p *LocalDataProvider) SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error) {
	state := p.storeRuntimeState()
	if state == nil {
		return false, nil
	}
	return state.SetStorePauseStatus(id, pause, pauseType)
}

func (p *LocalDataProvider) storeRuntimeState() *localStoreRuntimeState {
	if p == nil {
		return nil
	}
	return newLocalStoreRuntimeState(p.RuntimeResources, p.storeAPI())
}

type localOperationStrategy struct {
	ID                           int64      `gorm:"column:id"`
	TenantID                     int64      `gorm:"column:tenant_id"`
	StoreID                      int64      `gorm:"column:store_id"`
	Name                         string     `gorm:"column:name"`
	Platform                     string     `gorm:"column:platform"`
	Status                       int16      `gorm:"column:status"`
	StockChangeThreshold         int        `gorm:"column:stock_change_threshold"`
	StockChangeAction            string     `gorm:"column:stock_change_action"`
	OutOfStockAction             string     `gorm:"column:out_of_stock_action"`
	MinProfitRate                float64    `gorm:"column:min_profit_rate"`
	LowProfitAction              string     `gorm:"column:low_profit_action"`
	PriceUpdateMultiplier        float64    `gorm:"column:price_update_multiplier"`
	StockUpdateRatio             float64    `gorm:"column:stock_update_ratio"`
	ActivityEnabled              bool       `gorm:"column:activity_enabled"`
	ActivityType                 string     `gorm:"column:activity_type"`
	ActivityDiscountRate         float64    `gorm:"column:activity_discount_rate"`
	ActivityLimitedDiscountRate  float64    `gorm:"column:activity_limited_discount_rate"`
	ActivityStockRatio           float64    `gorm:"column:activity_stock_ratio"`
	PromotionRatio               float64    `gorm:"column:promotion_ratio"`
	ActivityMinProfitRate        float64    `gorm:"column:activity_min_profit_rate"`
	ActivityLimitedMinProfitRate float64    `gorm:"column:activity_limited_min_profit_rate"`
	ActivityPriceMode            string     `gorm:"column:activity_price_mode"`
	ActivityPartakeType          string     `gorm:"column:activity_partake_type"`
	TimeLimitedDiscountRate      float64    `gorm:"column:time_limited_discount_rate"`
	TimeLimitedMinProfitRate     float64    `gorm:"column:time_limited_min_profit_rate"`
	TimeLimitedPriceMode         string     `gorm:"column:time_limited_price_mode"`
	TimeLimitedUserLimit         bool       `gorm:"column:time_limited_user_limit"`
	TimeLimitedUserLimitNum      int        `gorm:"column:time_limited_user_limit_num"`
	TimeLimitedStockLimit        bool       `gorm:"column:time_limited_stock_limit"`
	TimeLimitedStockLimitPercent int        `gorm:"column:time_limited_stock_limit_percent"`
	FixedPriceAdjustment         float64    `gorm:"column:fixed_price_adjustment"`
	PriceIncreaseThreshold       float64    `gorm:"column:price_increase_threshold"`
	PriceDecreaseThreshold       float64    `gorm:"column:price_decrease_threshold"`
	PriceIncreaseAction          string     `gorm:"column:price_increase_action"`
	PriceDecreaseAction          string     `gorm:"column:price_decrease_action"`
	RestoreStockAmount           int        `gorm:"column:restore_stock_amount"`
	Remark                       string     `gorm:"column:remark"`
	CreateTime                   *time.Time `gorm:"column:create_time"`
}

func (p *LocalDataProvider) GetOperationStrategyByStoreID(storeID int64) (*listingadmin.OperationStrategyDTO, error) {
	api := p.operationStrategyAPI()
	if api == nil {
		return nil, nil
	}
	return api.GetOperationStrategyByStoreId(storeID)
}

func (p *LocalDataProvider) operationStrategyAPI() listingadmin.OperationStrategyAPI {
	if p == nil {
		return nil
	}
	return listingadmin.NewGormOperationStrategyAPI(p.OperationStrategyRepository())
}

func (p *LocalDataProvider) GetFilterRule(req *listingadmin.FilterRuleReqDTO) (*[]listingadmin.FilterRuleRespDTO, error) {
	api := p.filterRuleAPI()
	if api == nil {
		return nil, nil
	}
	return api.GetFilterRule(req)
}

func (p *LocalDataProvider) GetProfitRule(req *listingadmin.ProfitRuleReqDTO) (*listingadmin.ProfitRuleRespDTO, error) {
	api := p.profitRuleAPI()
	if api == nil {
		return nil, nil
	}
	return api.GetProfitRule(req)
}

func (p *LocalDataProvider) GetPricingRule(req *listingadmin.PricingRuleReqDTO) ([]listingadmin.PricingRuleRespDTO, error) {
	api := p.pricingRuleAPI()
	if api == nil {
		return nil, nil
	}
	return api.GetPricingRule(req)
}

func (p *LocalDataProvider) filterRuleAPI() listingadmin.FilterRuleAPI {
	if p == nil {
		return nil
	}
	return listingadmin.NewGormFilterRuleAPI(p.FilterRuleRepository())
}

func (p *LocalDataProvider) profitRuleAPI() listingadmin.ProfitRuleAPI {
	if p == nil {
		return nil
	}
	return listingadmin.NewGormProfitRuleAPI(p.ProfitRuleRepository())
}

func (p *LocalDataProvider) pricingRuleAPI() listingadmin.PricingRuleAPI {
	if p == nil {
		return nil
	}
	return listingadmin.NewGormPricingRuleAPI(p.PricingRuleRepository())
}

func (p *LocalDataProvider) GetRawJSONData(req *listingadmin.RawJsonDataReqDTO) (*listingadmin.RawJsonDataRespDTO, error) {
	api := p.rawJSONDataAPI()
	if api == nil {
		return nil, nil
	}
	return api.GetRawJsonData(req)
}

func (p *LocalDataProvider) GetRawJsonData(req *listingadmin.RawJsonDataReqDTO) (*listingadmin.RawJsonDataRespDTO, error) {
	return p.GetRawJSONData(req)
}

func (p *LocalDataProvider) CreateRawJSONData(req *listingadmin.RawJsonDataCreateReqDTO) (int64, error) {
	api := p.rawJSONDataAPI()
	if api == nil {
		return 0, nil
	}
	return api.CreateRawJsonData(req)
}

func (p *LocalDataProvider) CreateRawJsonData(req *listingadmin.RawJsonDataCreateReqDTO) (int64, error) {
	return p.CreateRawJSONData(req)
}

func (p *LocalDataProvider) rawJSONDataAPI() listingadmin.RawJsonDataAPI {
	if p == nil {
		return nil
	}
	return listingadmin.NewGormRawJsonDataAPI(p.RawJSONDataRepository())
}

func (p *LocalDataProvider) GetDailyListingCount(tenantID, storeID, userID int64, date string) (*listingadmin.DailyListingCountRespDTO, error) {
	api := p.dailyListingCountAPI()
	if api == nil {
		return nil, nil
	}
	return api.GetDailyListingCount(tenantID, storeID, userID, date)
}

func (p *LocalDataProvider) SetDailyListingCount(req *listingadmin.DailyListingCountSetReqDTO) error {
	api := p.dailyListingCountAPI()
	if api == nil {
		return nil
	}
	return api.SetDailyListingCount(req)
}

func (p *LocalDataProvider) TryConsumeDailyQuota(req *listingadmin.TryConsumeDailyQuotaReqDTO) (*listingadmin.TryConsumeDailyQuotaRespDTO, error) {
	api := p.dailyListingCountAPI()
	if api == nil {
		return nil, nil
	}
	return api.TryConsumeDailyQuota(req)
}

func (p *LocalDataProvider) RollbackDailyQuota(req *listingadmin.RollbackDailyQuotaReqDTO) (int64, error) {
	api := p.dailyListingCountAPI()
	if api == nil {
		return 0, nil
	}
	return api.RollbackDailyQuota(req)
}

func (p *LocalDataProvider) dailyListingCountAPI() listingadmin.DailyListingCountAPI {
	if p == nil {
		return nil
	}
	return NewLocalDailyListingCountAPI(p.redis)
}

func (p *LocalDataProvider) SetRemainingListingQuota(tenantID, storeID int64, quota int) (bool, error) {
	api := p.dailyListingCountAPI()
	if api == nil {
		return false, nil
	}
	return api.SetRemainingListingQuota(tenantID, storeID, quota)
}

func (p *LocalDataProvider) ListProductDataByStore(platform string, tenantID, storeID int64, shelfStatus *int) ([]*listingadmin.ProductDataDTO, error) {
	api := p.productDataAPI()
	if api == nil {
		return nil, nil
	}
	return api.ListByStore(platform, tenantID, storeID, shelfStatus)
}

func (p *LocalDataProvider) PageProductDataByStore(req *listingadmin.ProductDataListByStorePageReqDTO) (*listingadmin.PageResult[*listingadmin.ProductDataRespDTO], error) {
	api := p.productDataAPI()
	if api == nil {
		return nil, nil
	}
	return api.PageProductDataByStore(req)
}

func (p *LocalDataProvider) BatchCreateOrUpdateProductData(req *listingadmin.ProductDataBatchSaveReqDTO) (int, error) {
	api := p.productDataAPI()
	if api == nil {
		return 0, nil
	}
	return api.BatchCreateOrUpdate(req)
}

func (p *LocalDataProvider) BatchUpdateProductAttributes(req *listingadmin.ProductDataBatchUpdateAttributesReqDTO) (int, error) {
	api := p.productDataAPI()
	if api == nil {
		return 0, nil
	}
	return api.BatchUpdateAttributes(req)
}

func (p *LocalDataProvider) productDataAPI() listingadmin.ProductDataAPI {
	if p == nil {
		return nil
	}
	return listingadmin.NewGormProductDataAPI(p.ProductDataRepository(), 0)
}

func (p *LocalDataProvider) CreateProductImportMapping(req *listingadmin.ProductImportMappingCreateReqDTO) (int64, error) {
	api := p.productImportMappingAPI()
	if api == nil {
		return 0, nil
	}
	return api.CreateProductImportMapping(req)
}

func (p *LocalDataProvider) UpdateProductImportMapping(req *listingadmin.ProductImportMappingCreateReqDTO) (bool, error) {
	api := p.productImportMappingAPI()
	if api == nil || req == nil || req.ID == nil || *req.ID == 0 {
		return false, nil
	}
	err := api.UpdateProductImportMapping(req)
	return err == nil, err
}

func (p *LocalDataProvider) GetProductImportMappingByPlatformProductID(platformProductID string) (*listingadmin.ProductImportMappingRespDTO, bool, error) {
	api := p.productImportMappingAPI()
	if api == nil {
		return nil, false, nil
	}
	mapping, err := api.GetProductImportMappingByPlatformProductId(&listingadmin.ProductImportMappingGetReqDTO{PlatformProductId: platformProductID})
	return mapping, mapping != nil, err
}

func (p *LocalDataProvider) GetProductImportMappingByTaskAndSKU(importTaskID int64, sku string) (*listingadmin.ProductImportMappingRespDTO, bool, error) {
	api := p.productImportMappingAPI()
	if api == nil {
		return nil, false, nil
	}
	mapping, err := api.GetProductImportMappingByTaskAndSku(importTaskID, sku)
	return mapping, mapping != nil, err
}

func (p *LocalDataProvider) GetProductImportMappingBySKU(sku string, storeID int64) (*listingadmin.ProductImportMappingRespDTO, bool, error) {
	api := p.productImportMappingAPI()
	if api == nil {
		return nil, false, nil
	}
	mapping, err := api.GetProductImportMappingBySku(&listingadmin.ProductImportMappingGetBySkuReqDTO{Sku: sku, StoreId: storeID})
	return mapping, mapping != nil, err
}

func (p *LocalDataProvider) GetProductImportMappingByPlatformProductIDAndStore(platformProductID string, storeID int64) (*listingadmin.ProductImportMappingRespDTO, bool, error) {
	api := p.productImportMappingAPI()
	if api == nil {
		return nil, false, nil
	}
	mapping, err := api.GetProductImportMappingByPlatformProductIdAndStore(&listingadmin.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO{PlatformProductId: platformProductID, StoreId: storeID})
	return mapping, mapping != nil, err
}

func (p *LocalDataProvider) CheckProductExists(req *listingadmin.ProductImportMappingCheckReqDTO) (bool, bool, error) {
	api := p.productImportMappingAPI()
	if api == nil || req == nil {
		return false, false, nil
	}
	exists, err := api.CheckProductExists(req)
	return exists, true, err
}

func (p *LocalDataProvider) productImportMappingAPI() listingadmin.ProductImportMappingAPI {
	if p == nil {
		return nil
	}
	return listingadmin.NewGormProductImportMappingAPI(p.ProductImportMappingRepository())
}

func (p *LocalDataProvider) CreateInventoryRecord(req *listingadmin.InventoryRecordCreateReqDTO) (int64, error) {
	api := p.inventoryRecordAPI()
	if api == nil {
		return 0, nil
	}
	return api.CreateInventoryRecord(req)
}

func (p *LocalDataProvider) GetLatestInventoryRecord(platform, productID, region string) (*listingadmin.InventoryRecordRespDTO, bool, error) {
	api := p.inventoryRecordAPI()
	if api == nil {
		return nil, false, nil
	}
	record, err := api.GetLatestInventoryRecord(platform, productID, region)
	if err != nil {
		return nil, true, err
	}
	if record == nil {
		return nil, false, nil
	}
	return record, true, nil
}

func (p *LocalDataProvider) inventoryRecordAPI() listingadmin.InventoryRecordAPI {
	if p == nil {
		return nil
	}
	return listingadmin.NewGormInventoryRecordAPI(p.InventoryRecordRepository())
}

type localImportTaskRow struct {
	ID             int64      `gorm:"column:id"`
	TenantID       int64      `gorm:"column:tenant_id"`
	OwnerUserID    string     `gorm:"column:owner_user_id"`
	StoreID        int64      `gorm:"column:store_id"`
	Platform       string     `gorm:"column:platform"`
	SourcePlatform string     `gorm:"column:source_platform"`
	TargetPlatform string     `gorm:"column:target_platform"`
	Region         string     `gorm:"column:region"`
	CategoryID     *int64     `gorm:"column:category_id"`
	ProductID      string     `gorm:"column:product_id"`
	Status         int16      `gorm:"column:status"`
	ErrorMessage   string     `gorm:"column:error_message"`
	ReasonCode     string     `gorm:"column:reason_code"`
	Stage          string     `gorm:"column:stage"`
	RetryCount     int        `gorm:"column:retry_count"`
	MaxRetryCount  int        `gorm:"column:max_retry_count"`
	Deleted        int16      `gorm:"column:deleted"`
	Remark         string     `gorm:"column:remark"`
	Priority       int        `gorm:"column:priority"`
	CreateTime     time.Time  `gorm:"column:create_time"`
	UpdateTime     time.Time  `gorm:"column:update_time"`
	PublishedTime  *time.Time `gorm:"column:published_time"`
	Creator        string     `gorm:"column:creator"`
	Updater        string     `gorm:"column:updater"`
}

func (p *LocalDataProvider) GetPendingAndRetryTasks(limit int, userID int64, storeIDs []int64) ([]listingruntime.ImportTask, bool, error) {
	state := p.importTaskRuntimeState()
	if state == nil {
		return nil, false, nil
	}
	return state.GetPendingAndRetryTasks(limit, userID, storeIDs)
}

func (p *LocalDataProvider) GetImportTaskByID(taskID int64) (*listingruntime.ImportTask, bool, error) {
	state := p.importTaskRuntimeState()
	if state == nil {
		return nil, false, nil
	}
	return state.GetImportTaskByID(taskID)
}

func (p *LocalDataProvider) UpdateImportTaskStatus(req *listingadmin.ImportTaskStatusUpdate) (bool, error) {
	state := p.importTaskRuntimeState()
	if state == nil {
		return false, nil
	}
	return state.UpdateImportTaskStatus(req)
}

func (p *LocalDataProvider) importTaskRuntimeState() *localImportTaskRuntimeState {
	if p == nil {
		return nil
	}
	return newLocalImportTaskRuntimeState(p.RuntimeResources)
}

func ptrTime(ts time.Time) *time.Time {
	return &ts
}
