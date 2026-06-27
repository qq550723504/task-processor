package local

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/core/config"

	goredis "github.com/redis/go-redis/v9"
)

const sheinCookieRedisNamespace = "shein:cookie"

type SheinCookieLookupResult struct {
	TenantID   int64
	CookieJSON string
}

type SheinCookieProvider interface {
	GetCookie(ctx context.Context, storeID int64) (*SheinCookieLookupResult, error)
	DeleteCookie(ctx context.Context, storeID int64) (bool, error)
}

type redisSheinCookieProvider struct {
	client *goredis.Client
}

func newRedisSheinCookieProvider(cfg *config.RedisConfig) (SheinCookieProvider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("shein cookie redis config is nil")
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, fmt.Errorf("shein cookie redis host is empty")
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
		return nil, fmt.Errorf("connect shein cookie redis (%s:%d/%d): %w", cfg.Host, cfg.Port, cfg.DB, err)
	}

	return &redisSheinCookieProvider{client: client}, nil
}

func (p *redisSheinCookieProvider) GetCookie(ctx context.Context, storeID int64) (*SheinCookieLookupResult, error) {
	if p == nil || p.client == nil {
		return nil, fmt.Errorf("shein cookie redis provider is unavailable")
	}
	if storeID <= 0 {
		return nil, fmt.Errorf("invalid shein store id: %d", storeID)
	}

	pattern := fmt.Sprintf("%s:*:%d", sheinCookieRedisNamespace, storeID)
	keys, err := p.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("scan shein cookie keys: %w", err)
	}
	if len(keys) == 0 {
		return nil, nil
	}

	for _, key := range keys {
		tenantID, ok := extractTenantIDFromSheinCookieKey(key, storeID)
		if !ok {
			continue
		}

		raw, err := p.client.Get(ctx, key).Result()
		if err == goredis.Nil {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get shein cookie key %s: %w", key, err)
		}

		cookieJSON, err := normalizeSheinCookiePayload(raw)
		if err != nil {
			return nil, fmt.Errorf("normalize shein cookie payload for %s: %w", key, err)
		}
		if strings.TrimSpace(cookieJSON) == "" {
			continue
		}

		return &SheinCookieLookupResult{
			TenantID:   tenantID,
			CookieJSON: cookieJSON,
		}, nil
	}

	return nil, nil
}

func (p *redisSheinCookieProvider) DeleteCookie(ctx context.Context, storeID int64) (bool, error) {
	if p == nil || p.client == nil {
		return false, fmt.Errorf("shein cookie redis provider is unavailable")
	}
	if storeID <= 0 {
		return false, fmt.Errorf("invalid shein store id: %d", storeID)
	}

	pattern := fmt.Sprintf("%s:*:%d", sheinCookieRedisNamespace, storeID)
	keys, err := p.client.Keys(ctx, pattern).Result()
	if err != nil {
		return false, fmt.Errorf("scan shein cookie keys: %w", err)
	}
	if len(keys) == 0 {
		return false, nil
	}

	for _, key := range keys {
		tenantID, ok := extractTenantIDFromSheinCookieKey(key, storeID)
		if !ok {
			continue
		}

		lastLoginKey := fmt.Sprintf("shein:last_login_time:%d:%d", tenantID, storeID)
		lastLoginTimeStr, getErr := p.client.Get(ctx, lastLoginKey).Result()
		if getErr != nil && getErr != goredis.Nil {
			return false, fmt.Errorf("get shein last login time %s: %w", lastLoginKey, getErr)
		}
		if getErr == nil {
			lastLoginTime, parseErr := strconv.ParseFloat(strings.TrimSpace(lastLoginTimeStr), 64)
			if parseErr == nil {
				currentTime := float64(time.Now().Unix())
				if currentTime-lastLoginTime < 300 {
					return false, nil
				}
			}
		}

		deleted, delErr := p.client.Del(ctx, key).Result()
		if delErr != nil {
			return false, fmt.Errorf("delete shein cookie key %s: %w", key, delErr)
		}
		return deleted > 0, nil
	}

	return false, nil
}

func extractTenantIDFromSheinCookieKey(key string, storeID int64) (int64, bool) {
	parts := strings.Split(key, ":")
	if len(parts) != 4 || parts[0] != "shein" || parts[1] != "cookie" {
		return 0, false
	}
	if parts[3] != strconv.FormatInt(storeID, 10) {
		return 0, false
	}
	tenantID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || tenantID <= 0 {
		return 0, false
	}
	return tenantID, true
}

func normalizeSheinCookiePayload(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &wrapper); err == nil {
		if cookies, ok := wrapper["cookies"]; ok && len(cookies) > 0 && string(cookies) != "null" {
			return string(cookies), nil
		}
	}

	var list []json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &list); err == nil {
		return trimmed, nil
	}

	return trimmed, nil
}
