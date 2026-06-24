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

func (r *redisStringRuntime) AcquireLeaderLock(ctx context.Context, key, owner string, ttl time.Duration) (string, bool, error) {
	result, err := r.client.Eval(ctx, acquireLeaderLockScript, []string{key}, owner, leaderLockTTLMilliseconds(ttl)).Result()
	if err != nil {
		return "", false, err
	}
	values, ok := result.([]any)
	if !ok || len(values) != 2 {
		return "", false, fmt.Errorf("unexpected Redis leader lock response: %v", result)
	}
	acquired, err := redisScriptBool(values[0])
	if err != nil {
		return "", false, err
	}
	currentOwner, ok := values[1].(string)
	if !ok {
		return "", false, fmt.Errorf("unexpected Redis leader lock owner response: %v", values[1])
	}
	return currentOwner, acquired, nil
}

func (r *redisStringRuntime) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Close()
}

const acquireLeaderLockScript = `
local current = redis.call("GET", KEYS[1])
if not current then
	redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2])
	return {1, ARGV[1]}
end
if current == ARGV[1] then
	redis.call("PEXPIRE", KEYS[1], ARGV[2])
	return {1, current}
end
return {0, current}
`

func redisScriptBool(value any) (bool, error) {
	switch v := value.(type) {
	case int64:
		return v == 1, nil
	case int:
		return v == 1, nil
	default:
		return false, fmt.Errorf("unexpected Redis leader lock acquired response: %v", value)
	}
}

type rabbitQueueDepthSource struct {
	inspectQueue queueInspectFunc
	declarer     storeQueueDeclarer
	platform     string
}

type queueInspectFunc func(name string) (amqp.Queue, error)

type storeQueueDeclarer interface {
	DeclareQueue(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) error
	BindQueue(queueName, routingKey, exchangeName string, noWait bool, args amqp.Table) error
}

func newRabbitQueueDepthSource(inspectQueue queueInspectFunc, declarer storeQueueDeclarer, platform string) rabbitQueueDepthSource {
	return rabbitQueueDepthSource{
		inspectQueue: inspectQueue,
		declarer:     declarer,
		platform:     platform,
	}
}

func (s rabbitQueueDepthSource) QueueDepth(ctx context.Context, tenantID, storeID int64) (int64, error) {
	if s.inspectQueue == nil {
		return 0, errors.New("RabbitMQ queue inspector is not configured")
	}
	queue, err := s.inspectQueue(rabbitmq.GetStoreQueueName(s.platform, storeID))
	if err != nil {
		if isAMQPNotFound(err) {
			if err := s.declareStoreQueue(storeID); err != nil {
				return 0, err
			}
			return 0, nil
		}
		return 0, err
	}
	return int64(queue.Messages), nil
}

func (s rabbitQueueDepthSource) declareStoreQueue(storeID int64) error {
	if s.declarer == nil {
		return errors.New("RabbitMQ queue declarer is not configured")
	}
	for _, q := range rabbitmq.GetStoreQueueDeclareConfigs(s.platform, storeID) {
		if err := s.declarer.DeclareQueue(q.Name, q.Durable, q.AutoDelete, q.Exclusive, q.NoWait, q.Args); err != nil {
			return fmt.Errorf("declare store queue %s: %w", q.Name, err)
		}
	}
	for _, b := range rabbitmq.GetStoreQueueBindingConfigs(s.platform, storeID) {
		if err := s.declarer.BindQueue(b.QueueName, b.RoutingKey, b.ExchangeName, b.NoWait, b.Args); err != nil {
			return fmt.Errorf("bind store queue %s: %w", b.QueueName, err)
		}
	}
	return nil
}

func isAMQPNotFound(err error) bool {
	var amqpErr *amqp.Error
	return errors.As(err, &amqpErr) && amqpErr.Code == 404
}
