package local

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	listingkitstore "task-processor/internal/listingkit/store"
	platformdatabase "task-processor/internal/platform/database"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// RuntimeResources owns the infrastructure used by the local listing runtime.
// It is constructed at the composition boundary and closed by that owner.
type RuntimeResources struct {
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

	closeOnce sync.Once
	closeErr  error
}

func NewRuntimeResources(db *gorm.DB, redis *goredis.Client) *RuntimeResources {
	if db == nil && redis == nil {
		return nil
	}
	resources := &RuntimeResources{db: db, redis: redis}
	resources.initRepositories()
	return resources
}

func NewRuntimeResourcesFromConfig(dbCfg *config.DatabaseConfig, redisCfg *config.RedisConfig) (*RuntimeResources, error) {
	var (
		db  *gorm.DB
		rdb *goredis.Client
		err error
	)
	if dbCfg != nil && strings.TrimSpace(dbCfg.Host) != "" {
		db, err = platformdatabase.Open(platformDatabaseConfig(dbCfg))
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
			if db != nil {
				_ = platformdatabase.Close(db)
			}
			return nil, fmt.Errorf("connect local redis (%s:%d/%d): %w", redisCfg.Host, redisCfg.Port, redisCfg.DB, err)
		}
	}
	return NewRuntimeResources(db, rdb), nil
}

func (r *RuntimeResources) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		var errs []error
		if r.db != nil {
			if err := platformdatabase.Close(r.db); err != nil {
				errs = append(errs, err)
			}
		}
		if r.redis != nil {
			if err := r.redis.Close(); err != nil {
				errs = append(errs, err)
			}
		}
		r.closeErr = errors.Join(errs...)
	})
	return r.closeErr
}

func platformDatabaseConfig(cfg *config.DatabaseConfig) *platformdatabase.Config {
	if cfg == nil {
		return nil
	}
	return &platformdatabase.Config{
		Host:                  cfg.Host,
		Port:                  cfg.Port,
		User:                  cfg.User,
		Password:              cfg.Password,
		Database:              cfg.Database,
		MaxConnections:        cfg.MaxConnections,
		MaxIdleConnections:    cfg.MaxIdleConnections,
		ConnectionMaxLifetime: cfg.ConnectionMaxLifetime,
	}
}

func (r *RuntimeResources) HasDB() bool { return r != nil && r.db != nil }

func (r *RuntimeResources) HasRedis() bool { return r != nil && r.redis != nil }

func (r *RuntimeResources) DailyListingCountAPI() listingadmin.DailyListingCountAPI {
	if r == nil {
		return nil
	}
	return NewLocalDailyListingCountAPI(r.redis)
}

func (r *RuntimeResources) Database() *gorm.DB {
	if r == nil {
		return nil
	}
	return r.db
}

func (r *RuntimeResources) initRepositories() {
	if r == nil || r.db == nil {
		return
	}
	if r.storeRepo == nil {
		r.storeRepo = listingadmin.NewGormStoreRepository(r.db)
	}
	if r.filterRuleRepo == nil {
		r.filterRuleRepo = listingadmin.NewGormFilterRuleRepository(r.db)
	}
	if r.profitRuleRepo == nil {
		r.profitRuleRepo = listingadmin.NewGormProfitRuleRepository(r.db)
	}
	if r.operationStrategyRepo == nil {
		r.operationStrategyRepo = listingadmin.NewGormOperationStrategyRepository(r.db)
	}
	if r.scheduledTaskConfigRepo == nil {
		r.scheduledTaskConfigRepo = listingadmin.NewGormScheduledTaskConfigRepository(r.db)
	}
	if r.pricingRuleRepo == nil {
		r.pricingRuleRepo = listingadmin.NewGormPricingRuleRepository(r.db)
	}
	if r.productImportMappingRepo == nil {
		r.productImportMappingRepo = listingadmin.NewGormProductImportMappingRepository(r.db)
	}
	if r.productDataRepo == nil {
		r.productDataRepo = listingadmin.NewGormProductDataRepository(r.db)
	}
	if r.inventoryRecordRepo == nil {
		r.inventoryRecordRepo = listingadmin.NewGormInventoryRecordRepository(r.db)
	}
	if r.sheinSyncRepo == nil {
		r.sheinSyncRepo = listingkitstore.NewSheinSyncRepository(r.db)
	}
	if r.rawJSONDataRepo == nil {
		r.rawJSONDataRepo = listingadmin.NewGormRawJSONDataRepository(r.db)
	}
	if r.importTaskRepo == nil {
		r.importTaskRepo = listingadmin.NewGormImportTaskRepository(r.db)
	}
}

func (r *RuntimeResources) StoreRepository() *listingadmin.GormStoreRepository {
	if r == nil {
		return nil
	}
	r.initRepositories()
	return r.storeRepo
}

func (r *RuntimeResources) FilterRuleRepository() *listingadmin.GormFilterRuleRepository {
	if r == nil {
		return nil
	}
	r.initRepositories()
	return r.filterRuleRepo
}

func (r *RuntimeResources) ProfitRuleRepository() *listingadmin.GormProfitRuleRepository {
	if r == nil {
		return nil
	}
	r.initRepositories()
	return r.profitRuleRepo
}

func (r *RuntimeResources) OperationStrategyRepository() *listingadmin.GormOperationStrategyRepository {
	if r == nil {
		return nil
	}
	r.initRepositories()
	return r.operationStrategyRepo
}

func (r *RuntimeResources) ScheduledTaskConfigRepository() *listingadmin.GormScheduledTaskConfigRepository {
	if r == nil {
		return nil
	}
	r.initRepositories()
	return r.scheduledTaskConfigRepo
}

func (r *RuntimeResources) PricingRuleRepository() *listingadmin.GormPricingRuleRepository {
	if r == nil {
		return nil
	}
	r.initRepositories()
	return r.pricingRuleRepo
}

func (r *RuntimeResources) ProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository {
	if r == nil {
		return nil
	}
	r.initRepositories()
	return r.productImportMappingRepo
}

func (r *RuntimeResources) ProductDataRepository() listingadmin.ProductDataRepository {
	if r == nil {
		return nil
	}
	r.initRepositories()
	return r.productDataRepo
}

func (r *RuntimeResources) SheinSyncRepository() listingkit.SheinSyncRepository {
	if r == nil {
		return nil
	}
	r.initRepositories()
	return r.sheinSyncRepo
}

func (r *RuntimeResources) InventoryRecordRepository() *listingadmin.GormInventoryRecordRepository {
	if r == nil {
		return nil
	}
	r.initRepositories()
	return r.inventoryRecordRepo
}

func (r *RuntimeResources) RawJSONDataRepository() *listingadmin.GormRawJSONDataRepository {
	if r == nil {
		return nil
	}
	r.initRepositories()
	return r.rawJSONDataRepo
}

func (r *RuntimeResources) ImportTaskRepository() *listingadmin.GormImportTaskRepository {
	if r == nil {
		return nil
	}
	r.initRepositories()
	return r.importTaskRepo
}
