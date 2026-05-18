// Package database 提供数据访问层实现
package database

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"task-processor/internal/core/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type sharedDatabaseEntry struct {
	db   *gorm.DB
	refs int
}

var sharedDatabases = struct {
	mu      sync.Mutex
	entries map[string]*sharedDatabaseEntry
}{
	entries: make(map[string]*sharedDatabaseEntry),
}

func sharedDatabaseKey(cfg *config.DatabaseConfig) string {
	if cfg == nil {
		return ""
	}
	return fmt.Sprintf("%s:%d:%s:%s", cfg.Host, cfg.Port, cfg.User, cfg.Database)
}

func quoteIdentifier(name string) string {
	name = strings.ReplaceAll(name, `"`, `""`)
	return fmt.Sprintf(`"%s"`, name)
}

func createDatabaseIfNotExists(cfg *config.DatabaseConfig) error {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=postgres sslmode=disable TimeZone=Asia/Shanghai",
		cfg.Host, cfg.Port, cfg.User, cfg.Password,
	)

	adminDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("连接管理员数据库失败: %w", err)
	}

	sqlDB, err := adminDB.DB()
	if err != nil {
		return fmt.Errorf("获取管理员 sql.DB 失败: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("管理员数据库连通性检查失败: %w", err)
	}

	var count int64
	if err := adminDB.Raw("SELECT count(*) FROM pg_database WHERE datname = ?", cfg.Database).Scan(&count).Error; err != nil {
		return fmt.Errorf("检查数据库是否存在失败: %w", err)
	}

	if count > 0 {
		return nil
	}

	if err := adminDB.Exec(fmt.Sprintf("CREATE DATABASE %s", quoteIdentifier(cfg.Database))).Error; err != nil {
		return fmt.Errorf("创建数据库失败: %w", err)
	}
	return nil
}

// NewDatabaseFromConfig 从 config.DatabaseConfig 创建数据库连接，cfg 为 nil 时返回 (nil, nil)。
func NewDatabaseFromConfig(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	if cfg == nil {
		return nil, nil
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:  logger.Default.LogMode(logger.Silent),
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "database \""+cfg.Database+"\" does not exist") {
			if err2 := createDatabaseIfNotExists(cfg); err2 != nil {
				return nil, err2
			}
			db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
				Logger:  logger.Default.LogMode(logger.Silent),
				NowFunc: func() time.Time { return time.Now().UTC() },
			})
			if err != nil {
				return nil, fmt.Errorf("连接数据库失败: %w", err)
			}
		} else {
			return nil, fmt.Errorf("连接数据库失败: %w", err)
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取 sql.DB 失败: %w", err)
	}

	maxConn := cfg.MaxConnections
	if maxConn <= 0 {
		maxConn = 10
	}
	maxIdle := cfg.MaxIdleConnections
	if maxIdle <= 0 {
		maxIdle = 5
	}
	lifetime := cfg.ConnectionMaxLifetime
	if lifetime <= 0 {
		lifetime = time.Hour
	}

	sqlDB.SetMaxOpenConns(maxConn)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(lifetime)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连通性检查失败: %w", err)
	}

	return db, nil
}

// NewSharedDatabaseFromConfig returns a process-local shared *gorm.DB for the
// given config. Repeated calls with the same config reuse one underlying sql.DB.
func NewSharedDatabaseFromConfig(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	if cfg == nil {
		return nil, nil
	}

	key := sharedDatabaseKey(cfg)
	sharedDatabases.mu.Lock()
	if entry := sharedDatabases.entries[key]; entry != nil {
		entry.refs++
		db := entry.db
		sharedDatabases.mu.Unlock()
		return db, nil
	}
	sharedDatabases.mu.Unlock()

	db, err := NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	sharedDatabases.mu.Lock()
	if entry := sharedDatabases.entries[key]; entry != nil {
		entry.refs++
		sharedDatabases.mu.Unlock()
		_ = CloseDatabase(db)
		return entry.db, nil
	}
	sharedDatabases.entries[key] = &sharedDatabaseEntry{
		db:   db,
		refs: 1,
	}
	sharedDatabases.mu.Unlock()
	return db, nil
}

// CloseDatabase 关闭数据库连接
func CloseDatabase(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取 sql.DB 失败: %w", err)
	}
	return sqlDB.Close()
}

// CloseSharedDatabase releases one reference to a shared database handle and
// closes the underlying sql.DB only when the last caller releases it.
func CloseSharedDatabase(cfg *config.DatabaseConfig, db *gorm.DB) error {
	if cfg == nil || db == nil {
		return nil
	}

	key := sharedDatabaseKey(cfg)
	sharedDatabases.mu.Lock()
	entry := sharedDatabases.entries[key]
	if entry == nil || entry.db != db {
		sharedDatabases.mu.Unlock()
		return CloseDatabase(db)
	}

	entry.refs--
	if entry.refs > 0 {
		sharedDatabases.mu.Unlock()
		return nil
	}

	delete(sharedDatabases.entries, key)
	sharedDatabases.mu.Unlock()
	return CloseDatabase(db)
}
