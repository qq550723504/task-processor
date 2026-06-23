package listingcontrol

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"
	controllib "task-processor/internal/listingcontrol"

	amqp "github.com/rabbitmq/amqp091-go"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type directStoreSource struct {
	db *gorm.DB
}

type listingStoreSnapshotRow struct {
	ID                int64 `gorm:"column:id"`
	TenantID          int64 `gorm:"column:tenant_id"`
	Platform          string
	Status            int
	EnableAutoListing *bool  `gorm:"column:enable_auto_listing"`
	Name              string `gorm:"column:name"`
}

func NewDirectStoreSource(db *gorm.DB) controllib.StoreSource {
	return directStoreSource{db: db}
}

func (s directStoreSource) ListEnabledAutoListingStores(ctx context.Context, platform string) ([]controllib.StoreSnapshot, error) {
	if s.db == nil {
		return nil, errors.New("store source database is not configured")
	}

	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" {
		return nil, errors.New("platform is required")
	}

	var rows []listingStoreSnapshotRow
	if err := s.db.WithContext(ctx).
		Table("listing_store").
		Select("id, tenant_id, platform, status, enable_auto_listing, name").
		Where("deleted = ?", 0).
		Where("LOWER(platform) = ?", platform).
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	stores := make([]controllib.StoreSnapshot, 0, len(rows))
	for _, row := range rows {
		stores = append(stores, controllib.StoreSnapshot{
			TenantID:          row.TenantID,
			StoreID:           row.ID,
			Platform:          row.Platform,
			Status:            row.Status,
			EnableAutoListing: row.EnableAutoListing,
			Name:              row.Name,
		})
	}
	return stores, nil
}

type redisStringRuntime struct {
	client *goredis.Client
}

func newRedisStringRuntime(cfg *config.RedisConfig) (*redisStringRuntime, error) {
	if cfg == nil {
		return nil, errors.New("redis config is nil")
	}
	client := goredis.NewClient(&goredis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis connection failed (%s:%d): %w", cfg.Host, cfg.Port, err)
	}
	return &redisStringRuntime{client: client}, nil
}

func (r *redisStringRuntime) Get(ctx context.Context, key string) (string, error) {
	value, err := r.client.Get(ctx, key).Result()
	if err == goredis.Nil {
		return "", controllib.ErrRuntimeKeyNotFound
	}
	return value, err
}

func (r *redisStringRuntime) Exists(ctx context.Context, key string) (bool, error) {
	count, err := r.client.Exists(ctx, key).Result()
	return count > 0, err
}

func (r *redisStringRuntime) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		if err == goredis.Nil {
			return 0, controllib.ErrRuntimeKeyNotFound
		}
		return 0, err
	}
	return ttl, nil
}

func (r *redisStringRuntime) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Close()
}

type rabbitQueueDepthSource struct {
	channel  queueInspector
	platform string
}

type queueInspector interface {
	QueueInspect(name string) (amqp.Queue, error)
}

func newRabbitQueueDepthSource(channel queueInspector, platform string) rabbitQueueDepthSource {
	return rabbitQueueDepthSource{
		channel:  channel,
		platform: platform,
	}
}

func (s rabbitQueueDepthSource) QueueDepth(ctx context.Context, tenantID, storeID int64) (int64, error) {
	if s.channel == nil {
		return 0, errors.New("RabbitMQ queue inspector is not configured")
	}
	queue, err := s.channel.QueueInspect(rabbitmq.GetStoreQueueName(s.platform, storeID))
	if err != nil {
		return 0, err
	}
	return int64(queue.Messages), nil
}
