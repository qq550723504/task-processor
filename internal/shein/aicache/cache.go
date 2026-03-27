// Package aicache 提供 SHEIN 上架流程中 AI 调用结果的持久化缓存。
//
// 架构：PostgreSQL 持久层（永久存储）+ 进程内内存二级缓存（30分钟）。
// 同一产品（相同 cacheKey）的 AI 结果跨租户、跨店铺共享，
// 避免重复调用 AI 接口。
package aicache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ---- 数据库模型 ----

// AIResultCache AI 结果缓存表（永久存储，无过期时间）
type AIResultCache struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	CacheKey  string    `gorm:"column:cache_key;uniqueIndex;not null;size:64"`
	CacheType string    `gorm:"column:cache_type;not null;size:32;index"`
	Value     string    `gorm:"column:value;not null;type:text"`
	HitCount  int64     `gorm:"column:hit_count;default:0"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 指定表名
func (AIResultCache) TableName() string { return "shein_ai_result_cache" }

// ---- 缓存类型常量 ----

const (
	TypeCategory     = "category"  // AI 分类选择
	TypeContent      = "content"   // 标题/描述内容优化
	TypeAttribute    = "attribute" // AI 属性选择
	TypeSaleAttr     = "sale_attr" // AI 销售规格生成
	TypeSKCTranslate = "skc_trans" // SKC 翻译优化
)

const memCacheTTL = 30 * time.Minute // 内存二级缓存 30 分钟

// ---- 内存二级缓存条目 ----

type memEntry struct {
	value     string
	expiresAt time.Time
}

// ---- Cache ----

// Cache AI 结果持久化缓存，线程安全。
// 读写顺序：内存 → PostgreSQL。数据库记录永久保存，不设过期时间。
type Cache struct {
	db     *gorm.DB
	mem    sync.Map // key -> memEntry
	logger *logrus.Entry
}

// New 创建缓存实例。db 为 nil 时退化为纯内存缓存（仅用于测试/无数据库场景）。
func New(db *gorm.DB) *Cache {
	c := &Cache{
		db:     db,
		logger: logger.GetGlobalLogger("aicache"),
	}
	if db != nil {
		if err := db.AutoMigrate(&AIResultCache{}); err != nil {
			c.logger.Warnf("自动迁移 AI 缓存表失败: %v", err)
		}
	}
	go c.cleanupMemLoop()
	return c
}

// HashKey 根据多个字符串生成 16 位十六进制哈希，用于构造缓存键。
func HashKey(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// Get 获取缓存，将 JSON 反序列化到 dest。未命中返回 false。
func (c *Cache) Get(cacheType, key string, dest any) bool {
	fullKey := cacheType + ":" + key

	// 1. 查内存
	if raw, ok := c.getFromMem(fullKey); ok {
		if err := json.Unmarshal([]byte(raw), dest); err == nil {
			return true
		}
	}

	// 2. 查数据库
	if c.db == nil {
		c.logger.Warnf("db is nill")
		return false
	}
	raw, ok := c.getFromDB(fullKey)
	if !ok {
		return false
	}
	if err := json.Unmarshal([]byte(raw), dest); err != nil {
		c.logger.Warnf("反序列化缓存失败 key=%s: %v", fullKey, err)
		return false
	}

	// 回填内存
	c.setToMem(fullKey, raw)
	return true
}

// Set 将 value 序列化后写入内存和数据库（永久保存）。
func (c *Cache) Set(cacheType, key string, value any) {
	fullKey := cacheType + ":" + key

	raw, err := json.Marshal(value)
	if err != nil {
		c.logger.Warnf("序列化缓存失败 key=%s: %v", fullKey, err)
		return
	}
	rawStr := string(raw)

	c.setToMem(fullKey, rawStr)

	if c.db != nil {
		c.setToDB(fullKey, cacheType, rawStr)
	}
}

// ---- 内存操作 ----

func (c *Cache) getFromMem(key string) (string, bool) {
	v, ok := c.mem.Load(key)
	if !ok {
		return "", false
	}
	e := v.(memEntry)
	if time.Now().After(e.expiresAt) {
		c.mem.Delete(key)
		return "", false
	}
	return e.value, true
}

func (c *Cache) setToMem(key, raw string) {
	c.mem.Store(key, memEntry{value: raw, expiresAt: time.Now().Add(memCacheTTL)})
}

// ---- 数据库操作 ----

func (c *Cache) getFromDB(key string) (string, bool) {
	var record AIResultCache
	if err := c.db.Where("cache_key = ?", key).First(&record).Error; err != nil {
		return "", false
	}
	// 异步更新命中次数
	go c.db.Model(&record).UpdateColumn("hit_count", gorm.Expr("hit_count + 1"))
	return record.Value, true
}

func (c *Cache) setToDB(key, cacheType, raw string) {
	record := AIResultCache{
		CacheKey:  key,
		CacheType: cacheType,
		Value:     raw,
	}
	// Upsert：key 冲突时更新 value
	err := c.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "cache_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&record).Error
	if err != nil {
		c.logger.Warnf("写入 AI 缓存失败 key=%s: %v", key, err)
	}
}

// ---- 内存清理（仅清理内存过期条目，数据库永久保留）----

func (c *Cache) cleanupMemLoop() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		c.mem.Range(func(k, v any) bool {
			if now.After(v.(memEntry).expiresAt) {
				c.mem.Delete(k)
			}
			return true
		})
	}
}
