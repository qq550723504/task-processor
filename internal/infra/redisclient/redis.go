// Package redisclient 提供基于 go-redis 的 Redis 客户端实现。
package redisclient

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"task-processor/internal/core/config"
)

// Client 实现 productenrich.RedisClient 接口（及其他需要相同接口的包）。
type Client struct {
	rdb *goredis.Client
}

// New 根据配置创建并验证 Redis 连接。cfg 为 nil 时返回错误。
func New(cfg *config.RedisConfig) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("redis config is nil")
	}
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis 连接失败 (%s:%d): %w", cfg.Host, cfg.Port, err)
	}
	return &Client{rdb: rdb}, nil
}

// Push 将 value 追加到 key 对应的列表末尾（RPUSH）。
func (c *Client) Push(ctx context.Context, key string, value string) error {
	return c.rdb.RPush(ctx, key, value).Err()
}

// Get 获取 key 对应的字符串值。key 不存在时返回 "key not found" 错误。
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if err == goredis.Nil {
		return "", fmt.Errorf("key not found: %s", key)
	}
	return val, err
}

// Set 设置 key 的字符串值，ttl=0 表示永不过期。
func (c *Client) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// SetNX sets key only when it does not already exist.
func (c *Client) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, key, value, ttl).Result()
}

// Delete 删除指定 key。
func (c *Client) Delete(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

// Scan iterates keys with a match pattern.
func (c *Client) Scan(ctx context.Context, cursor uint64, match string, count int64) (uint64, []string, error) {
	keys, nextCursor, err := c.rdb.Scan(ctx, cursor, match, count).Result()
	return nextCursor, keys, err
}

// SMembers returns all set members for the given key.
func (c *Client) SMembers(ctx context.Context, key string) ([]string, error) {
	return c.rdb.SMembers(ctx, key).Result()
}

// SAdd adds members into the target set.
func (c *Client) SAdd(ctx context.Context, key string, members ...string) error {
	values := make([]any, 0, len(members))
	for _, member := range members {
		values = append(values, member)
	}
	return c.rdb.SAdd(ctx, key, values...).Err()
}

// Close closes the underlying Redis client.
func (c *Client) Close() error {
	return c.rdb.Close()
}
