package sdslogin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/core/config"

	goredis "github.com/redis/go-redis/v9"
)

const sdsSharedAuthStateKey = "sds:auth:global"

type redisPersistedPayload struct {
	AccessToken string         `json:"access_token"`
	OutToken    string         `json:"out_token,omitempty"`
	MerchantID  int64          `json:"merchant_id,omitempty"`
	UserID      int64          `json:"user_id,omitempty"`
	Cookies     []CookieRecord `json:"cookies,omitempty"`
}

type RedisStateStore struct {
	client *goredis.Client
}

func newRedisStateStoreFromClient(client *goredis.Client) *RedisStateStore {
	return &RedisStateStore{client: client}
}

func NewRedisStateStore(cfg config.RedisConfig) (*RedisStateStore, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, nil
	}
	poolSize := cfg.PoolSize
	if poolSize <= 0 {
		poolSize = 10
	}
	client := goredis.NewClient(&goredis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: poolSize,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &RedisStateStore{client: client}, nil
}

func (s *RedisStateStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *RedisStateStore) Load(ctx context.Context) (*AuthPayload, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	raw, err := s.client.Get(ctx, sdsSharedAuthStateKey).Result()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var persisted redisPersistedPayload
	if err := json.Unmarshal([]byte(raw), &persisted); err != nil {
		return nil, err
	}
	return &AuthPayload{
		AccessToken: persisted.AccessToken,
		OutToken:    persisted.OutToken,
		MerchantID:  persisted.MerchantID,
		UserID:      persisted.UserID,
		Cookies:     append([]CookieRecord(nil), persisted.Cookies...),
	}, nil
}

func (s *RedisStateStore) Save(ctx context.Context, payload *AuthPayload) error {
	if s == nil || s.client == nil {
		return nil
	}
	if payload == nil {
		return fmt.Errorf("payload is nil")
	}
	body, err := json.Marshal(redisPersistedPayload{
		AccessToken: payload.AccessToken,
		OutToken:    payload.OutToken,
		MerchantID:  payload.MerchantID,
		UserID:      payload.UserID,
		Cookies:     append([]CookieRecord(nil), payload.Cookies...),
	})
	if err != nil {
		return err
	}
	return s.client.Set(ctx, sdsSharedAuthStateKey, body, 0).Err()
}

func (s *RedisStateStore) Clear(ctx context.Context) error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Del(ctx, sdsSharedAuthStateKey).Err()
}
