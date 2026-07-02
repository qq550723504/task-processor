package lock

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/core/config"

	goredis "github.com/redis/go-redis/v9"
)

const (
	redisUnlockScript = `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`
	redisExtendScript = `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("pexpire", KEYS[1], ARGV[2]) else return 0 end`
)

// RedisLock is a Redis-backed DistributedLock implementation.
type RedisLock struct {
	client *goredis.Client
	owner  string
	logger Logger
}

func NewRedisLock(cfg *config.RedisConfig, owner string, logger Logger) (*RedisLock, error) {
	if cfg == nil {
		return nil, errors.New("redis config is nil")
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, errors.New("redis host is required")
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
		return nil, fmt.Errorf("redis lock connection failed (%s:%d): %w", cfg.Host, cfg.Port, err)
	}
	return NewRedisLockWithClient(client, owner, logger), nil
}

func NewRedisLockWithClient(client *goredis.Client, owner string, logger Logger) *RedisLock {
	if strings.TrimSpace(owner) == "" {
		owner = fmt.Sprintf("owner-%d", time.Now().UnixNano())
	}
	return &RedisLock{
		client: client,
		owner:  owner,
		logger: logger,
	}
}

func (l *RedisLock) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if l == nil || l.client == nil {
		return false, errors.New("redis lock client is not configured")
	}
	if ttl <= 0 {
		return false, errors.New("lock ttl must be positive")
	}
	ok, err := l.client.SetNX(ctx, key, l.owner, ttl).Result()
	if err != nil {
		return false, err
	}
	if ok && l.logger != nil {
		l.logger.Debugf("[RedisLock] acquired lock %s ttl=%s", key, ttl)
	}
	return ok, nil
}

func (l *RedisLock) Unlock(ctx context.Context, key string) error {
	if l == nil || l.client == nil {
		return errors.New("redis lock client is not configured")
	}
	if err := l.client.Eval(ctx, redisUnlockScript, []string{key}, l.owner).Err(); err != nil {
		return err
	}
	if l.logger != nil {
		l.logger.Debugf("[RedisLock] released lock %s", key)
	}
	return nil
}

func (l *RedisLock) Extend(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if l == nil || l.client == nil {
		return false, errors.New("redis lock client is not configured")
	}
	if ttl <= 0 {
		return false, errors.New("lock ttl must be positive")
	}
	result, err := l.client.Eval(ctx, redisExtendScript, []string{key}, l.owner, ttl.Milliseconds()).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (l *RedisLock) IsLocked(ctx context.Context, key string) (bool, error) {
	if l == nil || l.client == nil {
		return false, errors.New("redis lock client is not configured")
	}
	count, err := l.client.Exists(ctx, key).Result()
	return count > 0, err
}

func (l *RedisLock) Close() error {
	if l == nil || l.client == nil {
		return nil
	}
	return l.client.Close()
}

var _ DistributedLock = (*RedisLock)(nil)
