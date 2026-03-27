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

// Delete 删除指定 key。
func (c *Client) Delete(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}
