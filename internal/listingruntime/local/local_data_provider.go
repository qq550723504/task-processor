package local

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/listingruntime"
	"task-processor/internal/pkg/types"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	localDailyCountTTL      = 30 * 24 * time.Hour
	localStoreStatusEnabled = 0
)

type LocalDataProvider struct {
	db                       *gorm.DB
	redis                    *goredis.Client
	storeRepo                *listingadmin.GormStoreRepository
	filterRuleRepo           *listingadmin.GormFilterRuleRepository
	profitRuleRepo           *listingadmin.GormProfitRuleRepository
	operationStrategyRepo    *listingadmin.GormOperationStrategyRepository
	scheduledTaskConfigRepo  *listingadmin.GormScheduledTaskConfigRepository
	pricingRuleRepo          *listingadmin.GormPricingRuleRepository
	productImportMappingRepo *listingadmin.GormProductImportMappingRepository
	productDataRepo          *listingadmin.GormProductDataRepository
	inventoryRecordRepo      *listingadmin.GormInventoryRecordRepository
	sheinSyncRepo            listingkit.SheinSyncRepository
	rawJSONDataRepo          *listingadmin.GormRawJSONDataRepository
	importTaskRepo           *listingadmin.GormImportTaskRepository
}

func NewLocalDataProvider(dbCfg *config.DatabaseConfig, redisCfg *config.RedisConfig) (*LocalDataProvider, error) {
	var (
		db  *gorm.DB
		rdb *goredis.Client
		err error
	)
	if dbCfg != nil && strings.TrimSpace(dbCfg.Host) != "" {
		db, err = database.NewDatabaseFromConfig(dbCfg)
		if err != nil {
			return nil, err
		}
	}
	if redisCfg != nil && strings.TrimSpace(redisCfg.Host) != "" {
		poolSize := redisCfg.PoolSize
		if poolSize <= 0 {
			poolSize = 10
		}
		rdb = goredis.NewClient(&goredis.Options{
			Addr:     fmt.Sprintf("%s:%d", redisCfg.Host, redisCfg.Port),
			Password: redisCfg.Password,
			DB:       redisCfg.DB,
			PoolSize: poolSize,
		})
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := rdb.Ping(ctx).Err(); err != nil {
			_ = rdb.Close()
			return nil, fmt.Errorf("connect local redis (%s:%d/%d): %w", redisCfg.Host, redisCfg.Port, redisCfg.DB, err)
		}
	}
	if db == nil && rdb == nil {
		return nil, nil
	}
	provider := &LocalDataProvider{db: db, redis: rdb}
	provider.initRepositories()
	return provider, nil
}

func (p *LocalDataProvider) Close() error {
	var errs []error
	if p == nil {
		return nil
	}
	if p.db != nil {
		if err := database.CloseDatabase(p.db); err != nil {
			errs = append(errs, err)
		}
	}
	if p.redis != nil {
		if err := p.redis.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (p *LocalDataProvider) HasDB() bool {
	return p != nil && p.db != nil
}

func (p *LocalDataProvider) HasRedis() bool {
	return p != nil && p.redis != nil
}

func (p *LocalDataProvider) initRepositories() {
	if p == nil || p.db == nil {
		return
	}
	if p.storeRepo == nil {
		p.storeRepo = listingadmin.NewGormStoreRepository(p.db)
	}
	if p.filterRuleRepo == nil {
		p.filterRuleRepo = listingadmin.NewGormFilterRuleRepository(p.db)
	}
	if p.profitRuleRepo == nil {
		p.profitRuleRepo = listingadmin.NewGormProfitRuleRepository(p.db)
	}
	if p.operationStrategyRepo == nil {
		p.operationStrategyRepo = listingadmin.NewGormOperationStrategyRepository(p.db)
	}
	if p.scheduledTaskConfigRepo == nil {
		p.scheduledTaskConfigRepo = listingadmin.NewGormScheduledTaskConfigRepository(p.db)
	}
	if p.pricingRuleRepo == nil {
		p.pricingRuleRepo = listingadmin.NewGormPricingRuleRepository(p.db)
	}
	if p.productImportMappingRepo == nil {
		p.productImportMappingRepo = listingadmin.NewGormProductImportMappingRepository(p.db)
	}
	if p.productDataRepo == nil {
		p.productDataRepo = listingadmin.NewGormProductDataRepository(p.db)
	}
	if p.inventoryRecordRepo == nil {
		p.inventoryRecordRepo = listingadmin.NewGormInventoryRecordRepository(p.db)
	}
	if p.sheinSyncRepo == nil {
		p.sheinSyncRepo = listingkitstore.NewSheinSyncRepository(p.db)
	}
	if p.rawJSONDataRepo == nil {
		p.rawJSONDataRepo = listingadmin.NewGormRawJSONDataRepository(p.db)
	}
	if p.importTaskRepo == nil {
		p.importTaskRepo = listingadmin.NewGormImportTaskRepository(p.db)
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
	ValidFrom                *time.Time `gorm:"column:valid_from"`
	ValidUntil               *time.Time `gorm:"column:valid_until"`
	CreateTime               *time.Time `gorm:"column:create_time"`
	Creator                  string     `gorm:"column:creator"`
}

func (s localListingStore) toDTO() *listingadmin.StoreRespDTO {
	return &listingadmin.StoreRespDTO{
		ID:                       s.ID,
		TenantID:                 s.TenantID,
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

func storeToDTO(store *listingadmin.Store) *listingadmin.StoreRespDTO {
	if store == nil {
		return nil
	}
	return &listingadmin.StoreRespDTO{
		ID:                       store.ID,
		TenantID:                 store.TenantID,
		StoreID:                  store.StoreID,
		Name:                     store.Name,
		Username:                 store.Username,
		Password:                 store.Password,
		LoginUrl:                 store.LoginURL,
		ShopType:                 store.ShopType,
		Region:                   store.Region,
		Platform:                 store.Platform,
		DailyLimit:               store.DailyLimit,
		DailyLimitType:           store.DailyLimitType,
		FixedStockCount:          store.FixedStockCount,
		SkuGenerateStrategy:      store.SKUGenerateStrategy,
		Prefix:                   store.Prefix,
		Suffix:                   store.Suffix,
		Proxy:                    store.Proxy,
		EnableAutoListing:        store.EnableAutoListing,
		EnableAutoLogin:          store.EnableAutoLogin,
		EnableDraft:              store.EnableDraft,
		EnableAutoPrice:          store.EnableAutoPrice,
		DedicatedQueueEnabled:    store.DedicatedQueueEnabled,
		EnableRebargain:          store.EnableRebargain,
		EnableBrandAuthorization: store.EnableBrandAuthorization,
		AuthorizedBrandCode:      store.AuthorizedBrandCode,
		AuthorizedBrandName:      store.AuthorizedBrandName,
		TemuPriceRejectStrategy:  store.TemuPriceRejectStrategy,
		PriceType:                store.PriceType,
		Remark:                   store.Remark,
		Status:                   store.Status,
		CreateTime:               types.ToFlexibleTime(store.CreateTime),
		Creator:                  store.CreatedBy,
	}
}

func filterRuleToDTO(rule listingadmin.FilterRule) listingadmin.FilterRuleRespDTO {
	dto := listingadmin.FilterRuleRespDTO{
		ID:              rule.ID,
		Name:            rule.Name,
		RuleCode:        rule.RuleCode,
		Description:     rule.Description,
		TenantID:        rule.TenantID,
		PriceType:       rule.PriceType,
		FulfillmentType: rule.FulfillmentType,
		Status:          rule.Status,
		Remark:          rule.Remark,
		CreateTime:      flexibleTimeValue(rule.CreateTime),
	}
	if rule.StoreID != nil {
		dto.StoreID = *rule.StoreID
	}
	if rule.CategoryID != nil {
		dto.CategoryID = *rule.CategoryID
	}
	dto.PriceMin = ptrFloat64(rule.PriceMin)
	dto.PriceMax = ptrFloat64(rule.PriceMax)
	dto.StockMin = ptrInt(rule.StockMin)
	dto.RatingMin = ptrFloat64(rule.RatingMin)
	dto.ReviewCountMin = ptrInt(rule.ReviewCountMin)
	dto.DeliveryTimeMax = rule.DeliveryTimeMax
	return dto
}

func profitRuleToDTO(rule *listingadmin.ProfitRule) listingadmin.ProfitRuleRespDTO {
	if rule == nil {
		return listingadmin.ProfitRuleRespDTO{}
	}
	return listingadmin.ProfitRuleRespDTO{
		ID:                      rule.ID,
		Name:                    rule.Name,
		RuleCode:                rule.RuleCode,
		Description:             rule.Description,
		StoreID:                 rule.StoreID,
		CategoryID:              rule.CategoryID,
		SalePriceMultiplier:     rule.SalePriceMultiplier,
		DiscountPriceMultiplier: rule.DiscountPriceMultiplier,
		Status:                  rule.Status,
		Remark:                  rule.Remark,
		CreateTime:              flexibleTimeValue(rule.CreateTime),
		TenantID:                rule.TenantID,
	}
}

func operationStrategyToDTO(strategy *listingadmin.OperationStrategy) *listingadmin.OperationStrategyDTO {
	if strategy == nil {
		return nil
	}
	return &listingadmin.OperationStrategyDTO{
		ID:                           strategy.ID,
		TenantID:                     strategy.TenantID,
		StoreID:                      strategy.StoreID,
		Name:                         strategy.Name,
		Platform:                     strategy.Platform,
		Status:                       strategy.Status,
		StockChangeThreshold:         intFromPtr(strategy.StockChangeThreshold),
		StockChangeAction:            strategy.StockChangeAction,
		OutOfStockAction:             strategy.OutOfStockAction,
		MinProfitRate:                float64FromPtr(strategy.MinProfitRate),
		LowProfitAction:              strategy.LowProfitAction,
		PriceUpdateMultiplier:        float64FromPtr(strategy.PriceUpdateMultiplier),
		StockUpdateRatio:             float64FromPtr(strategy.StockUpdateRatio),
		ActivityEnabled:              strategy.ActivityEnabled,
		ActivityType:                 strategy.ActivityType,
		ActivityDiscountRate:         float64FromPtr(strategy.ActivityDiscountRate),
		ActivityLimitedDiscountRate:  float64FromPtr(strategy.ActivityLimitedDiscountRate),
		ActivityStockRatio:           float64FromPtr(strategy.ActivityStockRatio),
		PromotionRatio:               float64FromPtr(strategy.PromotionRatio),
		ActivityMinProfitRate:        float64FromPtr(strategy.ActivityMinProfitRate),
		ActivityLimitedMinProfitRate: float64FromPtr(strategy.ActivityLimitedMinProfitRate),
		ActivityPriceMode:            strategy.ActivityPriceMode,
		ActivityPartakeType:          strategy.ActivityPartakeType,
		TimeLimitedDiscountRate:      float64FromPtr(strategy.TimeLimitedDiscountRate),
		TimeLimitedMinProfitRate:     float64FromPtr(strategy.TimeLimitedMinProfitRate),
		TimeLimitedPriceMode:         strategy.TimeLimitedPriceMode,
		TimeLimitedUserLimit:         strategy.TimeLimitedUserLimit,
		TimeLimitedUserLimitNum:      intFromPtr(strategy.TimeLimitedUserLimitNum),
		TimeLimitedStockLimit:        strategy.TimeLimitedStockLimit,
		TimeLimitedStockLimitPercent: intFromPtr(strategy.TimeLimitedStockLimitPercent),
		FixedPriceAdjustment:         float64FromPtr(strategy.FixedPriceAdjustment),
		PriceIncreaseThreshold:       float64FromPtr(strategy.PriceIncreaseThreshold),
		PriceDecreaseThreshold:       float64FromPtr(strategy.PriceDecreaseThreshold),
		PriceIncreaseAction:          strategy.PriceIncreaseAction,
		PriceDecreaseAction:          strategy.PriceDecreaseAction,
		RestoreStockAmount:           intFromPtr(strategy.RestoreStockAmount),
		Remark:                       strategy.Remark,
		CreateTime:                   flexibleStringValue(strategy.CreateTime),
	}
}

func pricingRuleToDTO(rule listingadmin.PricingRule) listingadmin.PricingRuleRespDTO {
	dto := listingadmin.PricingRuleRespDTO{
		ID:         rule.ID,
		Name:       rule.Name,
		RuleCode:   rule.RuleCode,
		StoreID:    rule.StoreID,
		CategoryID: rule.CategoryID,
		PriceMin:   ptrFloat64(rule.PriceMin),
		PriceMax:   ptrFloat64(rule.PriceMax),
		RuleType:   rule.RuleType,
		RuleValue:  ptrFloat64(rule.RuleValue),
		FixedValue: rule.FixedValue,
		Status:     int(rule.Status),
		CreateTime: flexibleTimeValue(rule.CreateTime),
		TenantID:   rule.TenantID,
	}
	if rule.Description != "" {
		dto.Description = ptrString(rule.Description)
	}
	if rule.AcceptCondition != "" {
		dto.AcceptCondition = ptrString(rule.AcceptCondition)
	}
	if rule.RejectCondition != "" {
		dto.RejectCondition = ptrString(rule.RejectCondition)
	}
	if rule.Remark != "" {
		dto.Remark = ptrString(rule.Remark)
	}
	return dto
}

func productImportMappingToDTO(mapping *listingadmin.ProductImportMapping) *listingadmin.ProductImportMappingRespDTO {
	if mapping == nil {
		return nil
	}
	dto := &listingadmin.ProductImportMappingRespDTO{
		ID:                      mapping.ID,
		ImportTaskId:            mapping.ImportTaskID,
		StoreId:                 mapping.StoreID,
		Platform:                mapping.Platform,
		Region:                  mapping.Region,
		ProductId:               mapping.ProductID,
		CostPrice:               mapping.CostPrice,
		FilterRuleId:            mapping.FilterRuleID,
		ProfitRuleId:            mapping.ProfitRuleID,
		SalePriceMultiplier:     ptrFloat64(mapping.SalePriceMultiplier),
		DiscountPriceMultiplier: ptrFloat64(mapping.DiscountPriceMultiplier),
		Status:                  mapping.Status,
		CreateTime:              types.ToFlexibleTime(mapping.CreateTime),
		TenantId:                mapping.TenantID,
	}
	if mapping.ParentProductID != "" {
		dto.ParentProductId = ptrString(mapping.ParentProductID)
	}
	if mapping.PlatformProductID != "" {
		dto.PlatformProductId = ptrString(mapping.PlatformProductID)
	}
	if mapping.PlatformParentProductID != "" {
		dto.PlatformParentProductId = ptrString(mapping.PlatformParentProductID)
	}
	if mapping.SKU != "" {
		dto.Sku = ptrString(mapping.SKU)
	}
	if mapping.FilterRuleRange != "" {
		dto.FilterRuleRange = ptrString(mapping.FilterRuleRange)
	}
	if mapping.Remark != "" {
		dto.Remark = ptrString(mapping.Remark)
	}
	return dto
}

func productImportMappingFromCreateReq(req *listingadmin.ProductImportMappingCreateReqDTO) *listingadmin.ProductImportMapping {
	if req == nil {
		return nil
	}
	mapping := &listingadmin.ProductImportMapping{
		TenantID:                req.TenantID,
		ImportTaskID:            req.ImportTaskId,
		StoreID:                 req.StoreId,
		Platform:                req.Platform,
		Region:                  req.Region,
		ProductID:               req.ProductId,
		CostPrice:               req.CostPrice,
		FilterRuleID:            req.FilterRuleId,
		ProfitRuleID:            req.ProfitRuleId,
		Status:                  int16FromPtr(req.Status),
		SalePriceMultiplier:     parseStringFloatWithDefault(req.SalePriceMultiplier, 1),
		DiscountPriceMultiplier: parseStringFloatWithDefault(req.DiscountPriceMultiplier, 1),
	}
	if req.ID != nil {
		mapping.ID = *req.ID
	}
	if req.Sku != nil {
		mapping.SKU = *req.Sku
	}
	if req.PlatformProductId != nil {
		mapping.PlatformProductID = *req.PlatformProductId
	}
	if req.ParentProductId != nil {
		mapping.ParentProductID = *req.ParentProductId
	}
	if req.PlatformParentProductId != nil {
		mapping.PlatformParentProductID = *req.PlatformParentProductId
	}
	if req.FilterRuleRange != nil {
		mapping.FilterRuleRange = *req.FilterRuleRange
	}
	if req.Remark != nil {
		mapping.Remark = *req.Remark
	}
	return mapping
}

func productDataToDTO(product *listingadmin.ProductData) *listingadmin.ProductDataDTO {
	if product == nil {
		return nil
	}
	return &listingadmin.ProductDataDTO{
		ID:                product.ID,
		Source:            product.Source,
		ImportTaskID:      int64FromPtr(product.ImportTaskID),
		StoreID:           int64FromPtr(product.StoreID),
		Platform:          product.Platform,
		CategoryID:        int64FromPtr(product.CategoryID),
		Region:            product.Region,
		ParentProductID:   product.ParentProductID,
		ProductID:         product.ProductID,
		Title:             product.Title,
		Description:       product.Description,
		OriginalPrice:     types.FlexibleString(strconv.FormatFloat(product.OriginalPrice, 'f', -1, 64)),
		SpecialPrice:      types.FlexibleString(strconv.FormatFloat(product.SpecialPrice, 'f', -1, 64)),
		PriceCurrency:     product.PriceCurrency,
		Stock:             types.FlexibleString(product.Stock),
		Brand:             product.Brand,
		Category:          product.Category,
		MainImageURL:      product.MainImageURL,
		ImageURLs:         string(product.ImageURLs),
		Attributes:        string(product.Attributes),
		SourceURL:         product.SourceURL,
		Status:            product.Status,
		RawJSONDataID:     int64FromPtr(product.RawJSONDataID),
		PlatformProductID: product.PlatformProductID,
		PlatformStatus:    product.PlatformStatus,
		ShelfStatus:       intFromPtr(product.ShelfStatus),
		PublishTime:       types.ToFlexibleTime(product.PublishTime),
		ShelfTime:         types.ToFlexibleTime(product.ShelfTime),
		LastSyncTime:      types.ToFlexibleTime(product.LastSyncTime),
		PlatformData:      string(product.PlatformData),
		TenantID:          product.TenantID,
		CreateTime:        types.ToFlexibleTime(product.CreateTime),
		UpdateTime:        types.ToFlexibleTime(product.UpdateTime),
	}
}

func productDataFromBatchItem(req *listingadmin.ProductDataBatchSaveReqDTO, product listingadmin.ProductDataItemDTO) listingadmin.ProductData {
	item := listingadmin.ProductData{
		TenantID:          req.TenantID,
		StoreID:           ptrInt64(req.StoreID),
		Platform:          req.Platform,
		Region:            req.Region,
		ParentProductID:   product.ParentProductID,
		ProductID:         product.ProductSku,
		Title:             product.ProductName,
		Description:       product.ProductDescription,
		OriginalPrice:     flexibleStringToFloat64(product.ProductPrice),
		SpecialPrice:      flexibleStringToFloat64(product.SpecialPrice),
		PriceCurrency:     product.PriceCurrency,
		Stock:             product.ProductStock.String(),
		Brand:             product.Brand,
		Category:          product.ProductCategory,
		MainImageURL:      product.ProductImage,
		ImageURLs:         rawJSONString(product.ImageUrls),
		Attributes:        rawJSONString(product.Attributes),
		PlatformStatus:    product.PlatformStatus,
		PlatformData:      rawJSONString(product.PlatformData),
		PlatformProductID: product.PlatformProductID,
	}
	if product.ShelfStatus != nil {
		item.ShelfStatus = product.ShelfStatus
	}
	if product.CategoryID != nil {
		item.CategoryID = product.CategoryID
	}
	if product.PublishTime != nil {
		item.PublishTime = &product.PublishTime.Time
	}
	if product.ShelfTime != nil {
		item.ShelfTime = &product.ShelfTime.Time
	}
	if product.CreateTime != nil {
		item.CreateTime = &product.CreateTime.Time
	}
	if product.UpdateTime != nil {
		item.UpdateTime = &product.UpdateTime.Time
	}
	return item
}

func productDataFromAttributesItem(platform string, tenantID, storeID int64, product listingadmin.ProductAttributesItemDTO) listingadmin.ProductData {
	return listingadmin.ProductData{
		TenantID:          tenantID,
		StoreID:           ptrInt64(storeID),
		Platform:          platform,
		PlatformProductID: product.PlatformProductID,
		Attributes:        rawJSONString(product.Attributes),
	}
}

func inventoryRecordToDTO(record *listingadmin.InventoryRecord) *listingadmin.InventoryRecordRespDTO {
	if record == nil {
		return nil
	}
	return &listingadmin.InventoryRecordRespDTO{
		ID:                 record.ID,
		Platform:           record.Platform,
		ProductId:          record.ProductID,
		Region:             record.Region,
		Stock:              record.Stock,
		StockStatus:        record.StockStatus,
		IsAvailable:        record.IsAvailable,
		OriginalPrice:      record.OriginalPrice,
		CurrentPrice:       record.CurrentPrice,
		Currency:           record.Currency,
		PriceChangePercent: record.PriceChangePercent,
		SyncSource:         record.SyncSource,
		Remark:             record.Remark,
		CreateTime:         flexibleTimeValue(record.CreateTime),
	}
}

func rawJSONDataToDTO(record *listingadmin.RawJSONData) *listingadmin.RawJsonDataRespDTO {
	if record == nil {
		return nil
	}
	return &listingadmin.RawJsonDataRespDTO{
		ID:          record.ID,
		TaskID:      record.ImportTaskID,
		Platform:    record.Platform,
		ProductID:   record.ProductID,
		Region:      record.Region,
		RawJSONData: record.RawJSONData,
		CreateTime:  flexibleTimeValue(record.CreateTime),
		UpdateTime:  flexibleTimeValue(record.UpdateTime),
	}
}

func importTaskToRuntime(task *listingadmin.ImportTask) *listingruntime.ImportTask {
	if task == nil {
		return nil
	}
	meta := localTaskStatusMetadata(task.Status)
	return &listingruntime.ImportTask{
		ID:              task.ID,
		TenantID:        task.TenantID,
		StoreID:         int64FromPtr(task.StoreID),
		Platform:        task.Platform,
		Region:          task.Region,
		CategoryID:      int64FromPtr(task.CategoryID),
		ProductID:       task.ProductID,
		Status:          task.Status,
		ErrorMessage:    task.ErrorMessage,
		RetryCount:      task.RetryCount,
		MaxRetryCount:   task.MaxRetryCount,
		Priority:        task.Priority,
		CreateTime:      timeToUnixMillis(task.CreateTime),
		PublishedTime:   timeToUnixMillis(task.PublishedTime),
		StatusKey:       meta.Key,
		CanonicalStatus: meta.Canonical,
	}
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

func int16FromPtr(value *int16) int16 {
	if value == nil {
		return 0
	}
	return *value
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

func parseStringFloatWithDefault(value *string, fallback float64) float64 {
	if value == nil || strings.TrimSpace(*value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(*value), 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func flexibleStringToFloat64(value types.FlexibleString) float64 {
	if strings.TrimSpace(value.String()) == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value.String()), 64)
	if err != nil {
		return 0
	}
	return parsed
}

func rawJSONString(value string) []byte {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if json.Valid([]byte(trimmed)) {
		return []byte(trimmed)
	}
	encoded, _ := json.Marshal(trimmed)
	return encoded
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
	repo := p.storeRepository()
	if repo == nil {
		return nil, nil
	}
	store, err := repo.FindStoreByID(context.Background(), id)
	if errors.Is(err, listingadmin.ErrStoreNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, nil
	}
	now := time.Now()
	if store.ValidFrom != nil && now.Before(*store.ValidFrom) {
		return nil, nil
	}
	if store.ValidUntil != nil && now.After(*store.ValidUntil) {
		return nil, nil
	}
	return storeToDTO(store), nil
}

func (p *LocalDataProvider) PageStores(req *listingadmin.StorePageReqDTO) (*listingadmin.PageResult[*listingadmin.StoreRespDTO], error) {
	repo := p.storeRepository()
	if repo == nil {
		return nil, nil
	}
	query := listingadmin.StoreQuery{}
	if req != nil {
		query.TenantID = req.TenantID
		query.Platform = req.Platform
		query.Page = req.PageNo
		query.PageSize = req.PageSize
		query.EnableAutoPrice = req.EnableAutoPrice
	}
	page, err := repo.ListStores(context.Background(), query)
	if err != nil {
		return nil, err
	}
	if page == nil {
		return nil, nil
	}
	items := make([]*listingadmin.StoreRespDTO, 0, len(page.Items))
	for i := range page.Items {
		store := page.Items[i]
		items = append(items, storeToDTO(&store))
	}
	return &listingadmin.PageResult[*listingadmin.StoreRespDTO]{List: items, Total: page.Total, PageNo: page.Page, PageSize: page.PageSize}, nil
}

func (p *LocalDataProvider) UpdateStoreID(id int64, storeID string) (bool, error) {
	repo := p.storeRepository()
	if repo == nil {
		return false, nil
	}
	store, err := repo.UpdateStoreID(context.Background(), id, storeID)
	return err == nil && store != nil, err
}

func (p *LocalDataProvider) UpdateStoreStatus(id int64, status int16, remark string) (bool, error) {
	repo := p.storeRepository()
	if repo == nil {
		return false, nil
	}
	store, err := repo.FindStoreByID(context.Background(), id)
	if err != nil {
		return false, err
	}
	updated, err := repo.UpdateStoreStatus(context.Background(), store.TenantID, id, status, remark)
	return err == nil && updated != nil, err
}

func (p *LocalDataProvider) DeleteStoreCookie(id int64) (bool, error) {
	if p == nil || p.redis == nil || p.db == nil {
		return false, nil
	}
	store, err := p.GetStore(id)
	if err != nil || store == nil {
		return false, err
	}

	ctx := context.Background()
	lastLoginKey := fmt.Sprintf("%s:last_login_time:%d:%d", strings.ToLower(store.Platform), store.TenantID, store.ID)
	lastLoginTimeStr, getErr := p.redis.Get(ctx, lastLoginKey).Result()
	if getErr != nil && getErr != goredis.Nil {
		return false, getErr
	}
	if getErr == nil {
		lastLoginTime, parseErr := strconv.ParseFloat(strings.TrimSpace(lastLoginTimeStr), 64)
		if parseErr == nil {
			currentTime := float64(time.Now().Unix())
			if currentTime-lastLoginTime < 300 {
				return false, nil
			}
		}
	}

	key := fmt.Sprintf("%s:cookie:%d:%d", strings.ToLower(store.Platform), store.TenantID, store.ID)
	deleted, err := p.redis.Del(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return deleted > 0, nil
}

func (p *LocalDataProvider) GetStorePauseStatus(id int64) (bool, error) {
	detail, err := p.GetStorePauseStatusDetail(id)
	if err != nil || detail == nil {
		return false, err
	}
	return detail.Paused, nil
}

func (p *LocalDataProvider) GetStorePauseStatusDetail(id int64) (*listingadmin.StorePauseStatusRespDTO, error) {
	if p == nil || p.redis == nil || p.db == nil {
		return nil, nil
	}
	store, err := p.GetStore(id)
	if err != nil || store == nil {
		return nil, err
	}
	key := fmt.Sprintf("listing:task:pause:%s:%d:%d", strings.ToLower(store.Platform), store.TenantID, store.ID)
	val, err := p.redis.Get(context.Background(), key).Result()
	if err == goredis.Nil {
		return &listingadmin.StorePauseStatusRespDTO{}, nil
	}
	if err != nil {
		return nil, err
	}
	ttl, _ := p.redis.TTL(context.Background(), key).Result()
	return &listingadmin.StorePauseStatusRespDTO{
		Paused:     true,
		Reason:     val,
		TTLSeconds: int64(ttl.Seconds()),
	}, nil
}

func (p *LocalDataProvider) SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error) {
	if p == nil || p.redis == nil || p.db == nil {
		return false, nil
	}
	store, err := p.GetStore(id)
	if err != nil || store == nil {
		return false, err
	}
	key := fmt.Sprintf("listing:task:pause:%s:%d:%d", strings.ToLower(store.Platform), store.TenantID, store.ID)
	ctx := context.Background()
	if !pause {
		res := p.redis.Del(ctx, key)
		return res.Err() == nil, res.Err()
	}
	err = p.redis.Set(ctx, key, pauseType, 24*time.Hour).Err()
	return err == nil, err
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
	repo := p.operationStrategyRepository()
	if repo == nil {
		return nil, nil
	}
	strategy, err := repo.GetLatestByStoreID(context.Background(), storeID)
	if err != nil {
		return nil, err
	}
	return operationStrategyToDTO(strategy), nil
}

func (p *LocalDataProvider) GetFilterRule(req *listingadmin.FilterRuleReqDTO) (*[]listingadmin.FilterRuleRespDTO, error) {
	repo := p.filterRuleRepository()
	if repo == nil || req == nil {
		return nil, nil
	}
	items, err := repo.ResolveFilterRules(context.Background(), req.TenantID, req.StoreID, req.CategoryID)
	if err != nil {
		return nil, err
	}
	rows := make([]listingadmin.FilterRuleRespDTO, 0, len(items))
	for _, item := range items {
		rows = append(rows, filterRuleToDTO(item))
	}
	return &rows, nil
}

func (p *LocalDataProvider) GetProfitRule(req *listingadmin.ProfitRuleReqDTO) (*listingadmin.ProfitRuleRespDTO, error) {
	repo := p.profitRuleRepository()
	if repo == nil || req == nil {
		return nil, nil
	}
	rule, err := repo.ResolveProfitRule(context.Background(), req.TenantID, req.StoreID)
	if err != nil || rule == nil {
		return nil, err
	}
	dto := profitRuleToDTO(rule)
	return &dto, nil
}

func (p *LocalDataProvider) GetPricingRule(req *listingadmin.PricingRuleReqDTO) ([]listingadmin.PricingRuleRespDTO, error) {
	repo := p.pricingRuleRepository()
	if repo == nil || req == nil || req.StoreID == nil {
		return nil, nil
	}
	items, err := repo.ListByStoreID(context.Background(), *req.StoreID)
	if err != nil {
		return nil, err
	}
	rows := make([]listingadmin.PricingRuleRespDTO, 0, len(items))
	for _, item := range items {
		rows = append(rows, pricingRuleToDTO(item))
	}
	return rows, nil
}

func (p *LocalDataProvider) GetRawJSONData(req *listingadmin.RawJsonDataReqDTO) (*listingadmin.RawJsonDataRespDTO, error) {
	repo := p.rawJSONDataRepository()
	if repo == nil || req == nil {
		return nil, nil
	}
	record, err := repo.GetLatestRawJSONData(context.Background(), req.Platform, req.ProductID, req.Region)
	if err != nil || record == nil {
		return nil, err
	}
	return rawJSONDataToDTO(record), nil
}

func (p *LocalDataProvider) GetRawJsonData(req *listingadmin.RawJsonDataReqDTO) (*listingadmin.RawJsonDataRespDTO, error) {
	return p.GetRawJSONData(req)
}

func (p *LocalDataProvider) CreateRawJSONData(req *listingadmin.RawJsonDataCreateReqDTO) (int64, error) {
	repo := p.rawJSONDataRepository()
	if repo == nil || req == nil {
		return 0, nil
	}
	record, err := repo.UpsertRawJSONData(context.Background(), &listingadmin.RawJSONData{
		TenantID:     req.TenantID,
		StoreID:      req.StoreID,
		ImportTaskID: req.ImportTaskID,
		Platform:     req.Platform,
		ProductID:    req.ProductID,
		Region:       req.Region,
		CategoryID:   req.CategoryID,
		RawJSONData:  req.RawJsonData,
		Creator:      req.Creator,
		Updater:      req.Creator,
	})
	if err != nil || record == nil {
		return 0, err
	}
	return record.ID, nil
}

func (p *LocalDataProvider) CreateRawJsonData(req *listingadmin.RawJsonDataCreateReqDTO) (int64, error) {
	return p.CreateRawJSONData(req)
}

func (p *LocalDataProvider) GetDailyListingCount(tenantID, storeID, userID int64, date string) (*listingadmin.DailyListingCountRespDTO, error) {
	if p == nil || p.redis == nil {
		return nil, nil
	}
	key := fmt.Sprintf("listing:daily:count:%d:%d:%s", tenantID, storeID, date)
	val, err := p.redis.Get(context.Background(), key).Result()
	if err == goredis.Nil {
		return &listingadmin.DailyListingCountRespDTO{TenantID: tenantID, StoreID: storeID, UserID: userID, Date: date, Count: 0}, nil
	}
	if err != nil {
		return nil, err
	}
	count, _ := strconv.ParseInt(val, 10, 64)
	return &listingadmin.DailyListingCountRespDTO{TenantID: tenantID, StoreID: storeID, UserID: userID, Date: date, Count: count}, nil
}

func (p *LocalDataProvider) SetDailyListingCount(req *listingadmin.DailyListingCountSetReqDTO) error {
	if p == nil || p.redis == nil || req == nil {
		return nil
	}
	key := fmt.Sprintf("listing:daily:count:%d:%d:%s", req.TenantID, req.StoreID, req.Date)
	return p.redis.Set(context.Background(), key, strconv.FormatInt(req.Count, 10), localDailyCountTTL).Err()
}

func (p *LocalDataProvider) TryConsumeDailyQuota(req *listingadmin.TryConsumeDailyQuotaReqDTO) (*listingadmin.TryConsumeDailyQuotaRespDTO, error) {
	if p == nil || p.redis == nil || req == nil {
		return nil, nil
	}
	currentResp, err := p.GetDailyListingCount(req.TenantID, req.StoreID, req.UserID, req.Date)
	if err != nil {
		return nil, err
	}
	next := currentResp.Count + req.Increment
	if next > req.Limit {
		remaining := req.Limit - currentResp.Count
		if remaining < 0 {
			remaining = 0
		}
		return &listingadmin.TryConsumeDailyQuotaRespDTO{Allowed: false, NewCount: currentResp.Count, Remaining: remaining, ReachedLimit: currentResp.Count >= req.Limit}, nil
	}
	if err := p.SetDailyListingCount(&listingadmin.DailyListingCountSetReqDTO{TenantID: req.TenantID, StoreID: req.StoreID, UserID: req.UserID, Date: req.Date, Count: next}); err != nil {
		return nil, err
	}
	remaining := req.Limit - next
	if remaining < 0 {
		remaining = 0
	}
	return &listingadmin.TryConsumeDailyQuotaRespDTO{Allowed: true, NewCount: next, Remaining: remaining, ReachedLimit: next >= req.Limit}, nil
}

func (p *LocalDataProvider) RollbackDailyQuota(req *listingadmin.RollbackDailyQuotaReqDTO) (int64, error) {
	if p == nil || p.redis == nil || req == nil {
		return 0, nil
	}
	resp, err := p.GetDailyListingCount(req.TenantID, req.StoreID, req.UserID, req.Date)
	if err != nil {
		return 0, err
	}
	next := resp.Count - req.Decrement
	if next < 0 {
		next = 0
	}
	return next, p.SetDailyListingCount(&listingadmin.DailyListingCountSetReqDTO{TenantID: req.TenantID, StoreID: req.StoreID, UserID: req.UserID, Date: req.Date, Count: next})
}

func (p *LocalDataProvider) SetRemainingListingQuota(tenantID, storeID int64, quota int) (bool, error) {
	if p == nil || p.redis == nil {
		return false, nil
	}
	key := fmt.Sprintf("listing:remaining:quota:%d:%d", tenantID, storeID)
	err := p.redis.Set(context.Background(), key, strconv.Itoa(quota), 0).Err()
	return err == nil, err
}

func (p *LocalDataProvider) ListProductDataByStore(platform string, tenantID, storeID int64, shelfStatus *int) ([]*listingadmin.ProductDataDTO, error) {
	repo := p.productDataRepository()
	if repo == nil {
		return nil, nil
	}
	query := listingadmin.ProductDataQuery{
		TenantID: tenantID,
		StoreID:  ptrInt64(storeID),
		Platform: platform,
		Page:     1,
		PageSize: 2000,
	}
	if shelfStatus != nil {
		query.ShelfStatus = shelfStatus
	}
	page, err := repo.ListProductData(context.Background(), query)
	if err != nil {
		return nil, err
	}
	if page == nil {
		return nil, nil
	}
	items := make([]*listingadmin.ProductDataDTO, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, productDataToDTO(&page.Items[i]))
	}
	return items, nil
}

func (p *LocalDataProvider) PageProductDataByStore(req *listingadmin.ProductDataListByStorePageReqDTO) (*listingadmin.PageResult[*listingadmin.ProductDataRespDTO], error) {
	repo := p.productDataRepository()
	if repo == nil || req == nil {
		return nil, nil
	}
	query := listingadmin.ProductDataQuery{
		TenantID:          req.TenantID,
		StoreID:           ptrInt64(req.StoreID),
		Platform:          req.Platform,
		Region:            req.Region,
		Title:             req.Title,
		Brand:             req.Brand,
		PlatformProductID: req.PlatformProductID,
		Page:              req.PageNo,
		PageSize:          req.PageSize,
	}
	if req.ShelfStatus != nil {
		query.ShelfStatus = req.ShelfStatus
	}
	if req.Category != "" {
		query.Category = req.Category
	}
	page, err := repo.ListProductData(context.Background(), query)
	if err != nil {
		return nil, err
	}
	if page == nil {
		return nil, nil
	}
	items := make([]*listingadmin.ProductDataRespDTO, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, &listingadmin.ProductDataRespDTO{ProductDataDTO: productDataToDTO(&page.Items[i])})
	}
	return &listingadmin.PageResult[*listingadmin.ProductDataRespDTO]{List: items, Total: page.Total, PageNo: page.Page, PageSize: page.PageSize}, nil
}

func (p *LocalDataProvider) BatchCreateOrUpdateProductData(req *listingadmin.ProductDataBatchSaveReqDTO) (int, error) {
	repo := p.productDataRepository()
	if repo == nil || req == nil {
		return 0, nil
	}
	items := make([]listingadmin.ProductData, 0, len(req.Products))
	for _, product := range req.Products {
		items = append(items, productDataFromBatchItem(req, product))
	}
	return repo.UpsertProductDataBatch(context.Background(), items)
}

func (p *LocalDataProvider) BatchUpdateProductAttributes(req *listingadmin.ProductDataBatchUpdateAttributesReqDTO) (int, error) {
	repo := p.productDataRepository()
	if repo == nil || req == nil {
		return 0, nil
	}
	items := make([]listingadmin.ProductData, 0, len(req.Products))
	for _, product := range req.Products {
		items = append(items, productDataFromAttributesItem(req.Platform, req.TenantID, req.StoreID, product))
	}
	return repo.BatchUpdateAttributesByPlatformProductID(context.Background(), items)
}

type localProductImportMappingRow struct {
	ID                      int64      `gorm:"column:id"`
	TenantID                int64      `gorm:"column:tenant_id"`
	ImportTaskID            int64      `gorm:"column:import_task_id"`
	StoreID                 int64      `gorm:"column:store_id"`
	Platform                string     `gorm:"column:platform"`
	Region                  string     `gorm:"column:region"`
	ProductID               string     `gorm:"column:product_id"`
	SKU                     *string    `gorm:"column:sku"`
	CostPrice               *float64   `gorm:"column:cost_price"`
	PlatformProductID       *string    `gorm:"column:platform_product_id"`
	ProfitRuleID            *int64     `gorm:"column:profit_rule_id"`
	SalePriceMultiplierRaw  *string    `gorm:"column:sale_price_multiplier"`
	DiscountPriceMultRaw    *string    `gorm:"column:discount_price_multiplier"`
	Status                  int16      `gorm:"column:status"`
	Remark                  *string    `gorm:"column:remark"`
	ParentProductID         *string    `gorm:"column:parent_product_id"`
	PlatformParentProductID *string    `gorm:"column:platform_parent_product_id"`
	FilterRuleID            *int64     `gorm:"column:filter_rule_id"`
	FilterRuleRange         *string    `gorm:"column:filter_rule_range"`
	CreateTime              *time.Time `gorm:"column:create_time"`
	UpdateTime              *time.Time `gorm:"column:update_time"`
}

func parseOptionalFloat(raw *string) *float64 {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(*raw), 64)
	if err != nil {
		return nil
	}
	return &value
}

func formatOptionalFloat(raw *float64) *string {
	if raw == nil {
		return nil
	}
	value := strconv.FormatFloat(*raw, 'f', -1, 64)
	return &value
}

func (r localProductImportMappingRow) toDTO() *listingadmin.ProductImportMappingRespDTO {
	return &listingadmin.ProductImportMappingRespDTO{
		ID:                      r.ID,
		ImportTaskId:            r.ImportTaskID,
		StoreId:                 r.StoreID,
		Platform:                r.Platform,
		Region:                  r.Region,
		ProductId:               r.ProductID,
		ParentProductId:         r.ParentProductID,
		PlatformProductId:       r.PlatformProductID,
		PlatformParentProductId: r.PlatformParentProductID,
		Sku:                     r.SKU,
		CostPrice:               r.CostPrice,
		FilterRuleId:            r.FilterRuleID,
		FilterRuleRange:         r.FilterRuleRange,
		ProfitRuleId:            r.ProfitRuleID,
		SalePriceMultiplier:     parseOptionalFloat(r.SalePriceMultiplierRaw),
		DiscountPriceMultiplier: parseOptionalFloat(r.DiscountPriceMultRaw),
		Status:                  r.Status,
		Remark:                  r.Remark,
		CreateTime:              types.ToFlexibleTime(r.CreateTime),
		TenantId:                r.TenantID,
	}
}

func (p *LocalDataProvider) CreateProductImportMapping(req *listingadmin.ProductImportMappingCreateReqDTO) (int64, error) {
	repo := p.productImportMappingRepository()
	if repo == nil || req == nil {
		return 0, nil
	}
	created, err := repo.CreateProductImportMapping(context.Background(), productImportMappingFromCreateReq(req))
	if err != nil || created == nil {
		return 0, err
	}
	return created.ID, nil
}

func (p *LocalDataProvider) UpdateProductImportMapping(req *listingadmin.ProductImportMappingCreateReqDTO) (bool, error) {
	repo := p.productImportMappingRepository()
	if repo == nil || req == nil {
		return false, nil
	}
	mapping := productImportMappingFromCreateReq(req)
	if mapping == nil || mapping.ID == 0 {
		return false, nil
	}
	updated, err := repo.UpdateProductImportMapping(context.Background(), mapping)
	return updated != nil, err
}

func (p *LocalDataProvider) GetProductImportMappingByPlatformProductID(platformProductID string) (*listingadmin.ProductImportMappingRespDTO, bool, error) {
	return p.findProductImportMapping("platform_product_id = ?", platformProductID)
}

func (p *LocalDataProvider) GetProductImportMappingByTaskAndSKU(importTaskID int64, sku string) (*listingadmin.ProductImportMappingRespDTO, bool, error) {
	return p.findProductImportMapping("import_task_id = ? AND sku = ?", importTaskID, sku)
}

func (p *LocalDataProvider) GetProductImportMappingBySKU(sku string, storeID int64) (*listingadmin.ProductImportMappingRespDTO, bool, error) {
	return p.findProductImportMapping("sku = ? AND store_id = ?", sku, storeID)
}

func (p *LocalDataProvider) GetProductImportMappingByPlatformProductIDAndStore(platformProductID string, storeID int64) (*listingadmin.ProductImportMappingRespDTO, bool, error) {
	return p.findProductImportMapping("platform_product_id = ? AND store_id = ?", platformProductID, storeID)
}

func productImportMappingQueryFromLegacyCondition(query string, args ...any) *listingadmin.ProductImportMappingQuery {
	result := &listingadmin.ProductImportMappingQuery{}
	switch query {
	case "platform_product_id = ?":
		if len(args) != 1 {
			return nil
		}
		value, ok := args[0].(string)
		if !ok {
			return nil
		}
		result.PlatformProductID = value
	case "import_task_id = ? AND sku = ?":
		if len(args) != 2 {
			return nil
		}
		importTaskID, ok := args[0].(int64)
		if !ok {
			return nil
		}
		sku, ok := args[1].(string)
		if !ok {
			return nil
		}
		result.ImportTaskID = &importTaskID
		result.SKU = sku
	case "sku = ? AND store_id = ?":
		if len(args) != 2 {
			return nil
		}
		sku, ok := args[0].(string)
		if !ok {
			return nil
		}
		storeID, ok := args[1].(int64)
		if !ok {
			return nil
		}
		result.SKU = sku
		result.StoreID = &storeID
	case "platform_product_id = ? AND store_id = ?":
		if len(args) != 2 {
			return nil
		}
		platformProductID, ok := args[0].(string)
		if !ok {
			return nil
		}
		storeID, ok := args[1].(int64)
		if !ok {
			return nil
		}
		result.PlatformProductID = platformProductID
		result.StoreID = &storeID
	default:
		return nil
	}
	return result
}

func (p *LocalDataProvider) findProductImportMapping(query string, args ...any) (*listingadmin.ProductImportMappingRespDTO, bool, error) {
	repo := p.productImportMappingRepository()
	if repo == nil {
		return nil, false, nil
	}
	mappingQuery := productImportMappingQueryFromLegacyCondition(query, args...)
	if mappingQuery == nil {
		return nil, false, errors.New("unsupported product import mapping query")
	}
	mapping, err := repo.FindLatest(context.Background(), *mappingQuery)
	if err != nil {
		return nil, false, err
	}
	if mapping == nil {
		return nil, false, nil
	}
	return productImportMappingToDTO(mapping), true, nil
}

func (p *LocalDataProvider) CheckProductExists(req *listingadmin.ProductImportMappingCheckReqDTO) (bool, bool, error) {
	repo := p.productImportMappingRepository()
	if repo == nil || req == nil {
		return false, false, nil
	}
	exists, err := repo.ExistsPublishedProduct(context.Background(), req.StoreId, req.Platform, req.Region, req.ProductId)
	return exists, true, err
}

func (p *LocalDataProvider) CreateInventoryRecord(req *listingadmin.InventoryRecordCreateReqDTO) (int64, error) {
	repo := p.inventoryRecordRepository()
	if repo == nil || req == nil {
		return 0, nil
	}
	record, err := repo.CreateInventoryRecord(context.Background(), &listingadmin.InventoryRecord{
		Platform:           req.Platform,
		ProductID:          req.ProductId,
		Region:             req.Region,
		Stock:              req.Stock,
		StockStatus:        req.StockStatus,
		IsAvailable:        req.IsAvailable,
		OriginalPrice:      req.OriginalPrice,
		CurrentPrice:       req.CurrentPrice,
		Currency:           req.Currency,
		PriceChangePercent: req.PriceChangePercent,
		SyncSource:         req.SyncSource,
		Remark:             req.Remark,
	})
	if err != nil || record == nil {
		return 0, err
	}
	return record.ID, nil
}

func (p *LocalDataProvider) GetLatestInventoryRecord(platform, productID, region string) (*listingadmin.InventoryRecordRespDTO, bool, error) {
	repo := p.inventoryRecordRepository()
	if repo == nil {
		return nil, false, nil
	}
	record, err := repo.GetLatestInventoryRecord(context.Background(), platform, productID, region)
	if err != nil {
		return nil, true, err
	}
	if record == nil {
		return nil, false, nil
	}
	return inventoryRecordToDTO(record), true, nil
}

type localImportTaskRow struct {
	ID            int64      `gorm:"column:id"`
	TenantID      int64      `gorm:"column:tenant_id"`
	StoreID       int64      `gorm:"column:store_id"`
	Platform      string     `gorm:"column:platform"`
	Region        string     `gorm:"column:region"`
	CategoryID    int64      `gorm:"column:category_id"`
	ProductID     string     `gorm:"column:product_id"`
	Status        int16      `gorm:"column:status"`
	ErrorMessage  string     `gorm:"column:error_message"`
	ReasonCode    string     `gorm:"column:reason_code"`
	Stage         string     `gorm:"column:stage"`
	RetryCount    int        `gorm:"column:retry_count"`
	MaxRetryCount int        `gorm:"column:max_retry_count"`
	Remark        string     `gorm:"column:remark"`
	Priority      int        `gorm:"column:priority"`
	CreateTime    time.Time  `gorm:"column:create_time"`
	UpdateTime    time.Time  `gorm:"column:update_time"`
	PublishedTime *time.Time `gorm:"column:published_time"`
	Creator       string     `gorm:"column:creator"`
	Updater       string     `gorm:"column:updater"`
}

func (r localImportTaskRow) toRuntimeTask() listingruntime.ImportTask {
	meta := localTaskStatusMetadata(r.Status)
	return listingruntime.ImportTask{
		ID:              r.ID,
		TenantID:        r.TenantID,
		StoreID:         r.StoreID,
		Platform:        r.Platform,
		Region:          r.Region,
		CategoryID:      r.CategoryID,
		ProductID:       r.ProductID,
		Status:          r.Status,
		ErrorMessage:    r.ErrorMessage,
		RetryCount:      r.RetryCount,
		MaxRetryCount:   r.MaxRetryCount,
		Priority:        r.Priority,
		CreateTime:      r.CreateTime.UnixMilli(),
		PublishedTime:   timeToUnixMillis(r.PublishedTime),
		Creator:         r.Creator,
		StatusKey:       meta.Key,
		CanonicalStatus: meta.Canonical,
	}
}

func (p *LocalDataProvider) GetPendingAndRetryTasks(limit int, userID int64, storeIDs []int64) ([]listingruntime.ImportTask, bool, error) {
	repo := p.importTaskRepository()
	if repo == nil {
		return nil, false, nil
	}
	tasks, err := repo.ListPendingAndRetryTasks(context.Background(), limit, userID, storeIDs)
	if err != nil {
		return nil, true, err
	}
	result := make([]listingruntime.ImportTask, 0, len(tasks))
	for i := range tasks {
		if runtimeTask := importTaskToRuntime(&tasks[i]); runtimeTask != nil {
			result = append(result, *runtimeTask)
		}
	}
	return result, true, nil
}

func (p *LocalDataProvider) GetImportTaskByID(taskID int64) (*listingruntime.ImportTask, bool, error) {
	repo := p.importTaskRepository()
	if repo == nil || taskID <= 0 {
		return nil, false, nil
	}
	task, err := repo.GetImportTaskByID(context.Background(), taskID)
	if err != nil {
		return nil, true, err
	}
	return importTaskToRuntime(task), true, nil
}

func (p *LocalDataProvider) UpdateImportTaskStatus(req *listingadmin.ImportTaskStatusUpdate) (bool, error) {
	repo := p.importTaskRepository()
	if repo == nil || req == nil {
		return false, nil
	}
	return repo.UpdateImportTaskStatus(context.Background(), req)
}

func ptrTime(ts time.Time) *time.Time {
	return &ts
}
