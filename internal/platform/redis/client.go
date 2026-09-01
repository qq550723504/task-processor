// Package redis provides the application Redis runtime.
package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Client implements Redis operations consumed by application and legacy transition callers.
type Client struct {
	rdb *goredis.Client
}

// New creates and verifies a Redis connection. A nil config returns an error.
func New(cfg *Config) (*Client, error) {
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

func (c *Client) Push(ctx context.Context, key string, value string) error {
	return c.rdb.RPush(ctx, key, value).Err()
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if err == goredis.Nil {
		return "", fmt.Errorf("key not found: %s", key)
	}
	return val, err
}

func (c *Client) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

func (c *Client) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, key, value, ttl).Result()
}

func (c *Client) Delete(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

func (c *Client) Scan(ctx context.Context, cursor uint64, match string, count int64) (uint64, []string, error) {
	keys, nextCursor, err := c.rdb.Scan(ctx, cursor, match, count).Result()
	return nextCursor, keys, err
}

func (c *Client) SMembers(ctx context.Context, key string) ([]string, error) {
	return c.rdb.SMembers(ctx, key).Result()
}

func (c *Client) SAdd(ctx context.Context, key string, members ...string) error {
	values := make([]any, 0, len(members))
	for _, member := range members {
		values = append(values, member)
	}
	return c.rdb.SAdd(ctx, key, values...).Err()
}

func (c *Client) ReplaceSet(ctx context.Context, key string, members ...string) error {
	_, err := c.rdb.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
		pipe.Del(ctx, key)
		if len(members) == 0 {
			return nil
		}

		values := make([]any, 0, len(members))
		for _, member := range members {
			values = append(values, member)
		}
		pipe.SAdd(ctx, key, values...)
		return nil
	})
	return err
}

func (c *Client) Close() error {
	return c.rdb.Close()
}
