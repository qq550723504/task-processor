package management

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/infra/database"
	"task-processor/internal/model"
	"task-processor/internal/pkg/types"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	localDailyCountTTL      = 30 * 24 * time.Hour
	localStoreStatusEnabled = 0
)

type LocalDataProvider struct {
	db    *gorm.DB
	redis *goredis.Client
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
	return &LocalDataProvider{db: db, redis: rdb}, nil
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

func (s localListingStore) toDTO() *api.StoreRespDTO {
	return &api.StoreRespDTO{
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

func (p *LocalDataProvider) GetStore(id int64) (*api.StoreRespDTO, error) {
	if p == nil || p.db == nil {
		return nil, nil
	}
	var row localListingStore
	err := p.db.Table("listing_store").Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if row.ValidFrom != nil && now.Before(*row.ValidFrom) {
		return nil, nil
	}
	if row.ValidUntil != nil && now.After(*row.ValidUntil) {
		return nil, nil
	}
	return row.toDTO(), nil
}

func (p *LocalDataProvider) PageStores(req *api.StorePageReqDTO) (*api.PageResult[*api.StoreRespDTO], error) {
	if p == nil || p.db == nil {
		return nil, nil
	}
	query := p.db.Table("listing_store")
	if req != nil {
		if req.TenantID > 0 {
			query = query.Where("tenant_id = ?", req.TenantID)
		}
		if req.Platform != "" {
			query = query.Where("platform = ?", req.Platform)
		}
		if req.EnableAutoPrice != nil {
			query = query.Where("enable_auto_price = ?", *req.EnableAutoPrice)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	pageNo, pageSize := 1, 20
	if req != nil {
		if req.PageNo > 0 {
			pageNo = req.PageNo
		}
		if req.PageSize > 0 {
			pageSize = req.PageSize
		}
	}
	var rows []localListingStore
	if err := query.Order("id desc").Offset((pageNo - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]*api.StoreRespDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toDTO())
	}
	return &api.PageResult[*api.StoreRespDTO]{List: items, Total: total, PageNo: pageNo, PageSize: pageSize}, nil
}

func (p *LocalDataProvider) UpdateStoreID(id int64, storeID string) (bool, error) {
	if p == nil || p.db == nil {
		return false, nil
	}
	res := p.db.Table("listing_store").Where("id = ?", id).Update("store_id", storeID)
	return res.Error == nil && res.RowsAffected > 0, res.Error
}

func (p *LocalDataProvider) UpdateStoreStatus(id int64, status int16, remark string) (bool, error) {
	if p == nil || p.db == nil {
		return false, nil
	}
	updates := map[string]any{"status": status}
	if remark != "" {
		updates["remark"] = remark
	}
	res := p.db.Table("listing_store").Where("id = ?", id).Updates(updates)
	return res.Error == nil, res.Error
}

func (p *LocalDataProvider) GetStorePauseStatus(id int64) (bool, error) {
	detail, err := p.GetStorePauseStatusDetail(id)
	if err != nil || detail == nil {
		return false, err
	}
	return detail.Paused, nil
}

func (p *LocalDataProvider) GetStorePauseStatusDetail(id int64) (*api.StorePauseStatusRespDTO, error) {
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
		return &api.StorePauseStatusRespDTO{}, nil
	}
	if err != nil {
		return nil, err
	}
	ttl, _ := p.redis.TTL(context.Background(), key).Result()
	return &api.StorePauseStatusRespDTO{
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
	ActivityStockRatio           float64    `gorm:"column:activity_stock_ratio"`
	PromotionRatio               float64    `gorm:"column:promotion_ratio"`
	ActivityMinProfitRate        float64    `gorm:"column:activity_min_profit_rate"`
	ActivityPriceMode            string     `gorm:"column:activity_price_mode"`
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

func (p *LocalDataProvider) GetOperationStrategyByStoreID(storeID int64) (*api.OperationStrategyDTO, error) {
	if p == nil || p.db == nil {
		return nil, nil
	}
	var row localOperationStrategy
	err := p.db.Table("listing_operation_strategy").Where("store_id = ?", storeID).Order("id desc").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var createTime types.FlexibleString
	if row.CreateTime != nil {
		createTime = types.FlexibleString(row.CreateTime.Format(time.RFC3339))
	}
	return &api.OperationStrategyDTO{
		ID:                           row.ID,
		TenantID:                     row.TenantID,
		StoreID:                      row.StoreID,
		Name:                         row.Name,
		Platform:                     row.Platform,
		Status:                       row.Status,
		StockChangeThreshold:         row.StockChangeThreshold,
		StockChangeAction:            row.StockChangeAction,
		OutOfStockAction:             row.OutOfStockAction,
		MinProfitRate:                row.MinProfitRate,
		LowProfitAction:              row.LowProfitAction,
		PriceUpdateMultiplier:        row.PriceUpdateMultiplier,
		StockUpdateRatio:             row.StockUpdateRatio,
		ActivityEnabled:              row.ActivityEnabled,
		ActivityType:                 row.ActivityType,
		ActivityDiscountRate:         row.ActivityDiscountRate,
		ActivityStockRatio:           row.ActivityStockRatio,
		PromotionRatio:               row.PromotionRatio,
		ActivityMinProfitRate:        row.ActivityMinProfitRate,
		ActivityPriceMode:            row.ActivityPriceMode,
		TimeLimitedDiscountRate:      row.TimeLimitedDiscountRate,
		TimeLimitedMinProfitRate:     row.TimeLimitedMinProfitRate,
		TimeLimitedPriceMode:         row.TimeLimitedPriceMode,
		TimeLimitedUserLimit:         row.TimeLimitedUserLimit,
		TimeLimitedUserLimitNum:      row.TimeLimitedUserLimitNum,
		TimeLimitedStockLimit:        row.TimeLimitedStockLimit,
		TimeLimitedStockLimitPercent: row.TimeLimitedStockLimitPercent,
		FixedPriceAdjustment:         row.FixedPriceAdjustment,
		PriceIncreaseThreshold:       row.PriceIncreaseThreshold,
		PriceDecreaseThreshold:       row.PriceDecreaseThreshold,
		PriceIncreaseAction:          row.PriceIncreaseAction,
		PriceDecreaseAction:          row.PriceDecreaseAction,
		RestoreStockAmount:           row.RestoreStockAmount,
		Remark:                       row.Remark,
		CreateTime:                   createTime,
	}, nil
}

func (p *LocalDataProvider) GetFilterRule(req *api.FilterRuleReqDTO) (*[]api.FilterRuleRespDTO, error) {
	if p == nil || p.db == nil || req == nil {
		return nil, nil
	}
	var rows []api.FilterRuleRespDTO
	load := func(q *gorm.DB) error { return q.Order("id desc").Find(&rows).Error }
	queryBase := p.db.Table("listing_filter_rule").Where("tenant_id = ?", req.TenantID)
	if req.CategoryID != 0 {
		if err := load(queryBase.Where("store_id = ? AND category_id = ?", req.StoreID, req.CategoryID)); err != nil {
			return nil, err
		}
	}
	if len(rows) == 0 {
		if err := load(queryBase.Where("store_id = ?", req.StoreID)); err != nil {
			return nil, err
		}
	}
	if len(rows) == 0 {
		if err := load(queryBase.Where("store_id IS NULL")); err != nil {
			return nil, err
		}
	}
	return &rows, nil
}

func (p *LocalDataProvider) GetProfitRule(req *api.ProfitRuleReqDTO) (*api.ProfitRuleRespDTO, error) {
	if p == nil || p.db == nil || req == nil {
		return nil, nil
	}
	var row api.ProfitRuleRespDTO
	queryBase := p.db.Table("listing_profit_rule").Where("tenant_id = ?", req.TenantID)
	try := []func() *gorm.DB{
		func() *gorm.DB { return queryBase.Where("store_id = ?", req.StoreID) },
		func() *gorm.DB { return queryBase.Where("store_id = ?", req.StoreID) },
		func() *gorm.DB { return queryBase.Where("store_id IS NULL") },
	}
	for _, build := range try {
		err := build().Order("id desc").Take(&row).Error
		if err == nil {
			return &row, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	return nil, nil
}

func (p *LocalDataProvider) GetPricingRule(req *api.PricingRuleReqDTO) ([]api.PricingRuleRespDTO, error) {
	if p == nil || p.db == nil || req == nil || req.StoreID == nil {
		return nil, nil
	}
	var rows []api.PricingRuleRespDTO
	if err := p.db.Table("listing_pricing_rule").Where("store_id = ?", *req.StoreID).Order("id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (p *LocalDataProvider) GetRawJSONData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error) {
	if p == nil || p.db == nil || req == nil {
		return nil, nil
	}
	var row api.RawJsonDataRespDTO
	err := p.db.Table("listing_raw_json_data").
		Where("deleted = ? AND platform = ? AND product_id = ? AND region = ?", false, req.Platform, req.ProductID, req.Region).
		Order("id desc").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}

func (p *LocalDataProvider) CreateRawJSONData(req *api.RawJsonDataCreateReqDTO) (int64, error) {
	if p == nil || p.db == nil || req == nil {
		return 0, nil
	}
	if existing, err := p.GetRawJSONData(&api.RawJsonDataReqDTO{Platform: req.Platform, ProductID: req.ProductID, Region: req.Region}); err == nil && existing != nil {
		err = p.db.Table("listing_raw_json_data").Where("id = ?", existing.ID).Updates(map[string]any{"raw_json_data": req.RawJsonData, "update_time": time.Now()}).Error
		return existing.ID, err
	}
	record := map[string]any{
		"platform":      req.Platform,
		"product_id":    req.ProductID,
		"region":        req.Region,
		"raw_json_data": req.RawJsonData,
		"create_time":   time.Now(),
		"update_time":   time.Now(),
		"creator":       req.Creator,
		"updater":       req.Creator,
	}
	if err := p.db.Table("listing_raw_json_data").Create(record).Error; err != nil {
		return 0, err
	}
	created, err := p.GetRawJSONData(&api.RawJsonDataReqDTO{Platform: req.Platform, ProductID: req.ProductID, Region: req.Region})
	if err != nil || created == nil {
		return 0, err
	}
	return created.ID, nil
}

func (p *LocalDataProvider) GetDailyListingCount(tenantID, storeID, userID int64, date string) (*api.DailyListingCountRespDTO, error) {
	if p == nil || p.redis == nil {
		return nil, nil
	}
	key := fmt.Sprintf("listing:daily:count:%d:%d:%s", tenantID, storeID, date)
	val, err := p.redis.Get(context.Background(), key).Result()
	if err == goredis.Nil {
		return &api.DailyListingCountRespDTO{TenantID: tenantID, StoreID: storeID, UserID: userID, Date: date, Count: 0}, nil
	}
	if err != nil {
		return nil, err
	}
	count, _ := strconv.ParseInt(val, 10, 64)
	return &api.DailyListingCountRespDTO{TenantID: tenantID, StoreID: storeID, UserID: userID, Date: date, Count: count}, nil
}

func (p *LocalDataProvider) SetDailyListingCount(req *api.DailyListingCountSetReqDTO) error {
	if p == nil || p.redis == nil || req == nil {
		return nil
	}
	key := fmt.Sprintf("listing:daily:count:%d:%d:%s", req.TenantID, req.StoreID, req.Date)
	return p.redis.Set(context.Background(), key, strconv.FormatInt(req.Count, 10), localDailyCountTTL).Err()
}

func (p *LocalDataProvider) TryConsumeDailyQuota(req *api.TryConsumeDailyQuotaReqDTO) (*api.TryConsumeDailyQuotaRespDTO, error) {
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
		return &api.TryConsumeDailyQuotaRespDTO{Allowed: false, NewCount: currentResp.Count, Remaining: remaining, ReachedLimit: currentResp.Count >= req.Limit}, nil
	}
	if err := p.SetDailyListingCount(&api.DailyListingCountSetReqDTO{TenantID: req.TenantID, StoreID: req.StoreID, UserID: req.UserID, Date: req.Date, Count: next}); err != nil {
		return nil, err
	}
	remaining := req.Limit - next
	if remaining < 0 {
		remaining = 0
	}
	return &api.TryConsumeDailyQuotaRespDTO{Allowed: true, NewCount: next, Remaining: remaining, ReachedLimit: next >= req.Limit}, nil
}

func (p *LocalDataProvider) RollbackDailyQuota(req *api.RollbackDailyQuotaReqDTO) (int64, error) {
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
	return next, p.SetDailyListingCount(&api.DailyListingCountSetReqDTO{TenantID: req.TenantID, StoreID: req.StoreID, UserID: req.UserID, Date: req.Date, Count: next})
}

func (p *LocalDataProvider) SetRemainingListingQuota(tenantID, storeID int64, quota int) (bool, error) {
	if p == nil || p.redis == nil {
		return false, nil
	}
	key := fmt.Sprintf("listing:remaining:quota:%d:%d", tenantID, storeID)
	err := p.redis.Set(context.Background(), key, strconv.Itoa(quota), 0).Err()
	return err == nil, err
}

type localProductData struct {
	ID                int64      `gorm:"column:id"`
	Source            string     `gorm:"column:source"`
	ImportTaskID      int64      `gorm:"column:import_task_id"`
	StoreID           int64      `gorm:"column:store_id"`
	Platform          string     `gorm:"column:platform"`
	CategoryID        int64      `gorm:"column:category_id"`
	Region            string     `gorm:"column:region"`
	ParentProductID   string     `gorm:"column:parent_product_id"`
	ProductID         string     `gorm:"column:product_id"`
	Title             string     `gorm:"column:title"`
	Description       string     `gorm:"column:description"`
	OriginalPrice     string     `gorm:"column:original_price"`
	SpecialPrice      string     `gorm:"column:special_price"`
	PriceCurrency     string     `gorm:"column:price_currency"`
	Stock             string     `gorm:"column:stock"`
	Brand             string     `gorm:"column:brand"`
	Category          string     `gorm:"column:category"`
	MainImageURL      string     `gorm:"column:main_image_url"`
	ImageURLs         string     `gorm:"column:image_urls"`
	Attributes        string     `gorm:"column:attributes"`
	SourceURL         string     `gorm:"column:source_url"`
	Status            int16      `gorm:"column:status"`
	RawJSONDataID     int64      `gorm:"column:raw_json_data_id"`
	PlatformProductID string     `gorm:"column:platform_product_id"`
	PlatformStatus    string     `gorm:"column:platform_status"`
	ShelfStatus       int        `gorm:"column:shelf_status"`
	PublishTime       *time.Time `gorm:"column:publish_time"`
	ShelfTime         *time.Time `gorm:"column:shelf_time"`
	LastSyncTime      *time.Time `gorm:"column:last_sync_time"`
	PlatformData      string     `gorm:"column:platform_data"`
	TenantID          int64      `gorm:"column:tenant_id"`
	CreateTime        *time.Time `gorm:"column:create_time"`
	UpdateTime        *time.Time `gorm:"column:update_time"`
	Creator           string     `gorm:"column:creator"`
	Updater           string     `gorm:"column:updater"`
	Deleted           bool       `gorm:"column:deleted"`
}

func toFlexibleTimePtr(ts *time.Time) *types.FlexibleTime {
	if ts == nil || ts.IsZero() {
		return nil
	}
	ft := types.FlexibleTime{Time: *ts}
	return &ft
}

func (p localProductData) toDTO() *api.ProductDataDTO {
	return &api.ProductDataDTO{
		ID:                p.ID,
		Source:            p.Source,
		ImportTaskID:      p.ImportTaskID,
		StoreID:           p.StoreID,
		Platform:          p.Platform,
		CategoryID:        p.CategoryID,
		Region:            p.Region,
		ParentProductID:   p.ParentProductID,
		ProductID:         p.ProductID,
		Title:             p.Title,
		Description:       p.Description,
		OriginalPrice:     types.FlexibleString(p.OriginalPrice),
		SpecialPrice:      types.FlexibleString(p.SpecialPrice),
		PriceCurrency:     p.PriceCurrency,
		Stock:             types.FlexibleString(p.Stock),
		Brand:             p.Brand,
		Category:          p.Category,
		MainImageURL:      p.MainImageURL,
		ImageURLs:         p.ImageURLs,
		Attributes:        p.Attributes,
		SourceURL:         p.SourceURL,
		Status:            p.Status,
		RawJSONDataID:     p.RawJSONDataID,
		PlatformProductID: p.PlatformProductID,
		PlatformStatus:    p.PlatformStatus,
		ShelfStatus:       p.ShelfStatus,
		PublishTime:       toFlexibleTimePtr(p.PublishTime),
		ShelfTime:         toFlexibleTimePtr(p.ShelfTime),
		LastSyncTime:      toFlexibleTimePtr(p.LastSyncTime),
		PlatformData:      p.PlatformData,
		TenantID:          p.TenantID,
		CreateTime:        toFlexibleTimePtr(p.CreateTime),
		UpdateTime:        toFlexibleTimePtr(p.UpdateTime),
		Creator:           p.Creator,
		Updater:           p.Updater,
		Deleted:           p.Deleted,
	}
}

func (p *LocalDataProvider) productDataBaseQuery(platform string, tenantID, storeID int64) *gorm.DB {
	return p.db.Table("listing_product_data").
		Where("platform = ? AND tenant_id = ? AND store_id = ?", platform, tenantID, storeID)
}

func (p *LocalDataProvider) ListProductDataByStore(platform string, tenantID, storeID int64, shelfStatus *int) ([]*api.ProductDataDTO, error) {
	if p == nil || p.db == nil {
		return nil, nil
	}
	query := p.productDataBaseQuery(platform, tenantID, storeID)
	if shelfStatus != nil {
		query = query.Where("shelf_status = ?", *shelfStatus)
	}
	var rows []localProductData
	if err := query.Order("id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]*api.ProductDataDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toDTO())
	}
	return items, nil
}

func (p *LocalDataProvider) PageProductDataByStore(req *api.ProductDataListByStorePageReqDTO) (*api.PageResult[*api.ProductDataRespDTO], error) {
	if p == nil || p.db == nil || req == nil {
		return nil, nil
	}
	query := p.productDataBaseQuery(req.Platform, req.TenantID, req.StoreID)
	if req.Region != "" {
		query = query.Where("region = ?", req.Region)
	}
	if req.ShelfStatus != nil {
		query = query.Where("shelf_status = ?", *req.ShelfStatus)
	}
	if req.Title != "" {
		query = query.Where("title LIKE ?", "%"+req.Title+"%")
	}
	if req.Brand != "" {
		query = query.Where("brand LIKE ?", "%"+req.Brand+"%")
	}
	if req.Category != "" {
		query = query.Where("category LIKE ?", "%"+req.Category+"%")
	}
	if req.PlatformProductID != "" {
		query = query.Where("platform_product_id = ?", req.PlatformProductID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	pageNo, pageSize := req.PageNo, req.PageSize
	if pageNo <= 0 {
		pageNo = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	var rows []localProductData
	if err := query.Order("id desc").Offset((pageNo - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]*api.ProductDataRespDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, &api.ProductDataRespDTO{ProductDataDTO: row.toDTO()})
	}
	return &api.PageResult[*api.ProductDataRespDTO]{List: items, Total: total, PageNo: pageNo, PageSize: pageSize}, nil
}

func (p *LocalDataProvider) BatchCreateOrUpdateProductData(req *api.ProductDataBatchSaveReqDTO) (int, error) {
	if p == nil || p.db == nil || req == nil {
		return 0, nil
	}
	updated := 0
	for _, product := range req.Products {
		now := time.Now()
		updates := map[string]any{
			"title":             product.ProductName,
			"product_id":        product.ProductSku,
			"original_price":    product.ProductPrice.String(),
			"stock":             product.ProductStock.String(),
			"category":          product.ProductCategory,
			"main_image_url":    product.ProductImage,
			"description":       product.ProductDescription,
			"brand":             product.Brand,
			"price_currency":    product.PriceCurrency,
			"image_urls":        product.ImageUrls,
			"attributes":        product.Attributes,
			"platform_status":   product.PlatformStatus,
			"platform_data":     product.PlatformData,
			"parent_product_id": product.ParentProductID,
			"region":            req.Region,
			"tenant_id":         req.TenantID,
			"store_id":          req.StoreID,
			"platform":          req.Platform,
			"update_time":       now,
		}
		if product.ShelfStatus != nil {
			updates["shelf_status"] = *product.ShelfStatus
		}
		if product.CategoryID != nil {
			updates["category_id"] = *product.CategoryID
		}
		if product.SpecialPrice.String() != "" {
			updates["special_price"] = product.SpecialPrice.String()
		}
		if product.PublishTime != nil && !product.PublishTime.IsZero() {
			updates["publish_time"] = product.PublishTime.Time
		}
		if product.ShelfTime != nil && !product.ShelfTime.IsZero() {
			updates["shelf_time"] = product.ShelfTime.Time
		}
		if product.CreateTime != nil && !product.CreateTime.IsZero() {
			updates["create_time"] = product.CreateTime.Time
		}
		if product.UpdateTime != nil && !product.UpdateTime.IsZero() {
			updates["update_time"] = product.UpdateTime.Time
		}
		res := p.db.Table("listing_product_data").
			Where("platform = ? AND tenant_id = ? AND store_id = ? AND platform_product_id = ?", req.Platform, req.TenantID, req.StoreID, product.PlatformProductID).
			Updates(updates)
		if res.Error != nil {
			return updated, res.Error
		}
		if res.RowsAffected == 0 {
			updates["platform_product_id"] = product.PlatformProductID
			updates["create_time"] = now
			if err := p.db.Table("listing_product_data").Create(updates).Error; err != nil {
				return updated, err
			}
		}
		updated++
	}
	return updated, nil
}

func (p *LocalDataProvider) BatchUpdateProductAttributes(req *api.ProductDataBatchUpdateAttributesReqDTO) (int, error) {
	if p == nil || p.db == nil || req == nil {
		return 0, nil
	}
	updated := 0
	for _, product := range req.Products {
		res := p.db.Table("listing_product_data").
			Where("platform = ? AND tenant_id = ? AND store_id = ? AND platform_product_id = ?", req.Platform, req.TenantID, req.StoreID, product.PlatformProductID).
			Updates(map[string]any{
				"attributes":  product.Attributes,
				"update_time": time.Now(),
			})
		if res.Error != nil {
			return updated, res.Error
		}
		updated += int(res.RowsAffected)
	}
	return updated, nil
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

func (r localProductImportMappingRow) toDTO() *api.ProductImportMappingRespDTO {
	return &api.ProductImportMappingRespDTO{
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

func (p *LocalDataProvider) CreateProductImportMapping(req *api.ProductImportMappingCreateReqDTO) (int64, error) {
	if p == nil || p.db == nil || req == nil {
		return 0, nil
	}
	row := localProductImportMappingRow{
		TenantID:                req.TenantID,
		ImportTaskID:            req.ImportTaskId,
		StoreID:                 req.StoreId,
		Platform:                req.Platform,
		Region:                  req.Region,
		ProductID:               req.ProductId,
		SKU:                     req.Sku,
		CostPrice:               req.CostPrice,
		PlatformProductID:       req.PlatformProductId,
		ProfitRuleID:            req.ProfitRuleId,
		SalePriceMultiplierRaw:  req.SalePriceMultiplier,
		DiscountPriceMultRaw:    req.DiscountPriceMultiplier,
		ParentProductID:         req.ParentProductId,
		PlatformParentProductID: req.PlatformParentProductId,
		FilterRuleID:            req.FilterRuleId,
		FilterRuleRange:         req.FilterRuleRange,
		CreateTime:              ptrTime(time.Now()),
		UpdateTime:              ptrTime(time.Now()),
	}
	if req.ID != nil {
		row.ID = *req.ID
	}
	if req.Status != nil {
		row.Status = *req.Status
	}
	if req.Remark != nil {
		row.Remark = req.Remark
	}
	if err := p.db.Table("listing_product_import_mapping").Create(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

func (p *LocalDataProvider) UpdateProductImportMapping(req *api.ProductImportMappingCreateReqDTO) (bool, error) {
	if p == nil || p.db == nil || req == nil {
		return false, nil
	}
	var id int64
	if req.ID != nil {
		id = *req.ID
	}
	if id == 0 {
		return false, nil
	}
	updates := map[string]any{
		"tenant_id":                  req.TenantID,
		"import_task_id":             req.ImportTaskId,
		"store_id":                   req.StoreId,
		"platform":                   req.Platform,
		"region":                     req.Region,
		"product_id":                 req.ProductId,
		"sku":                        req.Sku,
		"cost_price":                 req.CostPrice,
		"platform_product_id":        req.PlatformProductId,
		"profit_rule_id":             req.ProfitRuleId,
		"sale_price_multiplier":      req.SalePriceMultiplier,
		"discount_price_multiplier":  req.DiscountPriceMultiplier,
		"parent_product_id":          req.ParentProductId,
		"platform_parent_product_id": req.PlatformParentProductId,
		"filter_rule_id":             req.FilterRuleId,
		"filter_rule_range":          req.FilterRuleRange,
		"remark":                     req.Remark,
		"update_time":                time.Now(),
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	res := p.db.Table("listing_product_import_mapping").Where("id = ?", id).Updates(updates)
	return res.RowsAffected > 0, res.Error
}

func (p *LocalDataProvider) GetProductImportMappingByPlatformProductID(platformProductID string) (*api.ProductImportMappingRespDTO, bool, error) {
	return p.findProductImportMapping("platform_product_id = ?", platformProductID)
}

func (p *LocalDataProvider) GetProductImportMappingByTaskAndSKU(importTaskID int64, sku string) (*api.ProductImportMappingRespDTO, bool, error) {
	return p.findProductImportMapping("import_task_id = ? AND sku = ?", importTaskID, sku)
}

func (p *LocalDataProvider) GetProductImportMappingBySKU(sku string, storeID int64) (*api.ProductImportMappingRespDTO, bool, error) {
	return p.findProductImportMapping("sku = ? AND store_id = ?", sku, storeID)
}

func (p *LocalDataProvider) GetProductImportMappingByPlatformProductIDAndStore(platformProductID string, storeID int64) (*api.ProductImportMappingRespDTO, bool, error) {
	return p.findProductImportMapping("platform_product_id = ? AND store_id = ?", platformProductID, storeID)
}

func (p *LocalDataProvider) findProductImportMapping(query string, args ...any) (*api.ProductImportMappingRespDTO, bool, error) {
	if p == nil || p.db == nil {
		return nil, false, nil
	}
	var row localProductImportMappingRow
	err := p.db.Table("listing_product_import_mapping").Where(query, args...).Order("id desc").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return row.toDTO(), true, nil
}

func (p *LocalDataProvider) CheckProductExists(req *api.ProductImportMappingCheckReqDTO) (bool, bool, error) {
	if p == nil || p.db == nil || req == nil {
		return false, false, nil
	}
	var count int64
	err := p.db.Table("listing_product_import_mapping").
		Where("store_id = ? AND platform = ? AND region = ? AND product_id = ?", req.StoreId, req.Platform, req.Region, req.ProductId).
		Where("platform_product_id IS NOT NULL AND platform_product_id <> ''").
		Count(&count).Error
	return count > 0, true, err
}

type localInventoryRecordRow struct {
	ID                 int64     `gorm:"column:id"`
	Platform           string    `gorm:"column:platform"`
	ProductID          string    `gorm:"column:product_id"`
	Region             string    `gorm:"column:region"`
	Stock              *int      `gorm:"column:stock"`
	StockStatus        string    `gorm:"column:stock_status"`
	IsAvailable        bool      `gorm:"column:is_available"`
	OriginalPrice      *float64  `gorm:"column:original_price"`
	CurrentPrice       *float64  `gorm:"column:current_price"`
	Currency           string    `gorm:"column:currency"`
	PriceChangePercent *float64  `gorm:"column:price_change_percent"`
	SyncSource         string    `gorm:"column:sync_source"`
	Remark             string    `gorm:"column:remark"`
	CreateTime         time.Time `gorm:"column:create_time"`
}

func (r localInventoryRecordRow) toDTO() *api.InventoryRecordRespDTO {
	return &api.InventoryRecordRespDTO{
		ID:                 r.ID,
		Platform:           r.Platform,
		ProductId:          r.ProductID,
		Region:             r.Region,
		Stock:              r.Stock,
		StockStatus:        r.StockStatus,
		IsAvailable:        r.IsAvailable,
		OriginalPrice:      r.OriginalPrice,
		CurrentPrice:       r.CurrentPrice,
		Currency:           r.Currency,
		PriceChangePercent: r.PriceChangePercent,
		SyncSource:         r.SyncSource,
		Remark:             r.Remark,
		CreateTime:         types.FlexibleTime{Time: r.CreateTime},
	}
}

func (p *LocalDataProvider) CreateInventoryRecord(req *api.InventoryRecordCreateReqDTO) (int64, error) {
	if p == nil || p.db == nil || req == nil {
		return 0, nil
	}
	row := localInventoryRecordRow{
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
		CreateTime:         time.Now(),
	}
	if err := p.db.Table("listing_inventory_record").Create(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

func (p *LocalDataProvider) GetLatestInventoryRecord(platform, productID, region string) (*api.InventoryRecordRespDTO, bool, error) {
	if p == nil || p.db == nil {
		return nil, false, nil
	}
	var row localInventoryRecordRow
	err := p.db.Table("listing_inventory_record").
		Where("platform = ? AND product_id = ? AND region = ?", platform, productID, region).
		Order("create_time desc, id desc").
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return row.toDTO(), true, nil
}

type localImportTaskRow struct {
	ID            int64     `gorm:"column:id"`
	TenantID      int64     `gorm:"column:tenant_id"`
	StoreID       int64     `gorm:"column:store_id"`
	Platform      string    `gorm:"column:platform"`
	Region        string    `gorm:"column:region"`
	CategoryID    int64     `gorm:"column:category_id"`
	ProductID     string    `gorm:"column:product_id"`
	Status        int16     `gorm:"column:status"`
	ErrorMessage  string    `gorm:"column:error_message"`
	ReasonCode    string    `gorm:"column:reason_code"`
	Stage         string    `gorm:"column:stage"`
	RetryCount    int       `gorm:"column:retry_count"`
	MaxRetryCount int       `gorm:"column:max_retry_count"`
	Remark        string    `gorm:"column:remark"`
	Priority      int       `gorm:"column:priority"`
	CreateTime    time.Time `gorm:"column:create_time"`
	UpdateTime    time.Time `gorm:"column:update_time"`
	Creator       string    `gorm:"column:creator"`
	Updater       string    `gorm:"column:updater"`
}

func (r localImportTaskRow) toDTO() api.ProductImportTaskRespDTO {
	meta := localTaskStatusMetadata(r.Status)
	return api.ProductImportTaskRespDTO{
		ID:              r.ID,
		TenantID:        r.TenantID,
		StoreID:         r.StoreID,
		Platform:        r.Platform,
		Region:          r.Region,
		CategoryID:      r.CategoryID,
		ProductID:       r.ProductID,
		Status:          r.Status,
		ErrorMessage:    r.ErrorMessage,
		ReasonCode:      r.ReasonCode,
		Stage:           r.Stage,
		RetryCount:      r.RetryCount,
		MaxRetryCount:   r.MaxRetryCount,
		Remark:          r.Remark,
		Priority:        r.Priority,
		CreateTime:      r.CreateTime.UnixMilli(),
		UpdateTime:      r.UpdateTime.UnixMilli(),
		Creator:         r.Creator,
		Updater:         r.Updater,
		StatusKey:       meta.Key,
		StatusName:      meta.Name,
		CanonicalStatus: meta.Canonical,
	}
}

func (p *LocalDataProvider) GetPendingAndRetryTasks(limit int, userID int64, storeIDs []int64) ([]api.ProductImportTaskRespDTO, bool, error) {
	if p == nil || p.db == nil {
		return nil, false, nil
	}
	if limit <= 0 {
		limit = 20
	}
	statuses := []int16{
		model.TaskStatusPending.Int16(),
		model.TaskStatusPendingRetry.Int16(),
		model.TaskStatusCrawled.Int16(),
	}
	query := p.db.Table("listing_product_import_task").Where("status IN ?", statuses)
	if userID > 0 {
		query = query.Where("tenant_id = ?", userID)
	}
	if len(storeIDs) > 0 {
		query = query.Where("store_id IN ?", storeIDs)
	}
	var rows []localImportTaskRow
	if err := query.Order("priority asc, update_time asc, id asc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, true, err
	}
	result := make([]api.ProductImportTaskRespDTO, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDTO())
	}
	return result, true, nil
}

func (p *LocalDataProvider) UpdateImportTaskStatus(req *api.ProductImportTaskUpdateReqDTO) (bool, error) {
	if p == nil || p.db == nil || req == nil {
		return false, nil
	}
	var row localImportTaskRow
	err := p.db.Table("listing_product_import_task").Where("id = ?", req.ID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	if req.ExpectedCurrentStatus != nil && row.Status != *req.ExpectedCurrentStatus {
		return true, fmt.Errorf("管理端拒绝更新任务状态: taskId=%d, currentStatus=%d, expectedCurrentStatus=%d", req.ID, row.Status, *req.ExpectedCurrentStatus)
	}
	current, parseErr := model.ParseTaskStatus(row.Status)
	if parseErr == nil {
		target := model.TaskStatus(req.Status)
		if current != target {
			if err := model.ValidateTaskStatusTransition(current, target); err != nil {
				return true, fmt.Errorf("管理端拒绝更新任务状态: taskId=%d, invalid transition %d -> %d", req.ID, row.Status, req.Status)
			}
		}
	}
	updates := map[string]any{
		"status":        req.Status,
		"error_message": req.ErrorMessage,
		"reason_code":   req.ReasonCode,
		"stage":         req.Stage,
		"remark":        req.Remark,
		"update_time":   time.Now(),
	}
	if req.RetryCount != nil {
		updates["retry_count"] = *req.RetryCount
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	res := p.db.Table("listing_product_import_task").Where("id = ?", req.ID).Updates(updates)
	if res.Error != nil {
		return true, res.Error
	}
	return res.RowsAffected > 0, nil
}

func ptrTime(ts time.Time) *time.Time {
	return &ts
}
