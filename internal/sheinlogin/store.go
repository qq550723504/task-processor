package sheinlogin

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

const (
	cookieKeyPrefix     = "shein:cookie"
	verifyCodePrefix    = "shein:verify_code"
	verifyWaitPrefix    = "shein:wait_verify_code"
	lastLoginTimePrefix = "shein:last_login_time"
	lastFailurePrefix   = "shein:last_failure"
)

type RedisStore struct {
	client *goredis.Client
}

func newRedisStoreFromClient(client *goredis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func NewRedisStore(cfg config.RedisConfig) (*RedisStore, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, fmt.Errorf("shein login redis host is empty")
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
	return &RedisStore{client: client}, nil
}

func (s *RedisStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *RedisStore) Ready(ctx context.Context) bool {
	return s != nil && s.client != nil && s.client.Ping(ctx).Err() == nil
}

func (s *RedisStore) SaveCookieState(ctx context.Context, tenantID, storeID int64, payload map[string]any, ttl time.Duration) error {
	body, err := json.Marshal(cookieOnlyBrowserState(payload))
	if err != nil {
		return err
	}
	return s.client.Set(ctx, cookieKey(tenantID, storeID), body, ttl).Err()
}

func (s *RedisStore) ClearCookie(ctx context.Context, tenantID, storeID int64) error {
	return s.client.Del(ctx, cookieKey(tenantID, storeID)).Err()
}

func (s *RedisStore) CookieTTL(ctx context.Context, tenantID, storeID int64) (time.Duration, bool, error) {
	ttl, err := s.client.TTL(ctx, cookieKey(tenantID, storeID)).Result()
	if err == goredis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if ttl <= 0 {
		return 0, false, nil
	}
	return ttl, true, nil
}

func (s *RedisStore) HasCookie(ctx context.Context, tenantID, storeID int64) (bool, error) {
	ttl, ok, err := s.CookieTTL(ctx, tenantID, storeID)
	if err != nil {
		return false, err
	}
	return ok && ttl > 0, nil
}

func (s *RedisStore) SetVerifyWait(ctx context.Context, tenantID, storeID int64, ttl time.Duration) error {
	return s.client.Set(ctx, verifyWaitKey(tenantID, storeID), "waiting", ttl).Err()
}

func (s *RedisStore) CancelVerifyWait(ctx context.Context, tenantID, storeID int64) (bool, error) {
	n, err := s.client.Del(ctx, verifyWaitKey(tenantID, storeID)).Result()
	return n > 0, err
}

func (s *RedisStore) IsWaitingVerifyCode(ctx context.Context, tenantID, storeID int64) (bool, error) {
	n, err := s.client.Exists(ctx, verifyWaitKey(tenantID, storeID)).Result()
	return n > 0, err
}

func (s *RedisStore) SubmitVerifyCode(ctx context.Context, tenantID, storeID int64, code string, ttl time.Duration) error {
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, verifyCodeKey(tenantID, storeID), code, ttl)
	pipe.Set(ctx, verifyWaitKey(tenantID, storeID), "waiting", ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStore) ConsumeVerifyCode(ctx context.Context, tenantID, storeID int64) (string, bool, error) {
	key := verifyCodeKey(tenantID, storeID)
	value, err := s.client.Get(ctx, key).Result()
	if err == goredis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, key)
	pipe.Del(ctx, verifyWaitKey(tenantID, storeID))
	_, execErr := pipe.Exec(ctx)
	return value, true, execErr
}

func (s *RedisStore) RecordLastLoginTime(ctx context.Context, tenantID, storeID int64, when time.Time) error {
	return s.client.Set(ctx, lastLoginKey(tenantID, storeID), strconv.FormatInt(when.Unix(), 10), 30*24*time.Hour).Err()
}

func (s *RedisStore) LastLoginTime(ctx context.Context, tenantID, storeID int64) (*time.Time, error) {
	raw, err := s.client.Get(ctx, lastLoginKey(tenantID, storeID)).Result()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	unixSeconds, parseErr := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if parseErr != nil {
		return nil, nil
	}
	when := time.Unix(unixSeconds, 0)
	return &when, nil
}

func (s *RedisStore) ClearPauseKeys(ctx context.Context, tenantID, storeID int64) error {
	keys := []string{
		fmt.Sprintf("listing:task:pause:shein:%d:%d", tenantID, storeID),
		fmt.Sprintf("listing:task:pause:%d:%d", tenantID, storeID),
	}
	return s.client.Del(ctx, keys...).Err()
}

func (s *RedisStore) RecordLastFailure(ctx context.Context, tenantID, storeID int64, summary *FailureSummary, ttl time.Duration) error {
	if summary == nil {
		return s.ClearLastFailure(ctx, tenantID, storeID)
	}
	body, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, lastFailureKey(tenantID, storeID), body, ttl).Err()
}

func (s *RedisStore) LastFailure(ctx context.Context, tenantID, storeID int64) (*FailureSummary, error) {
	raw, err := s.client.Get(ctx, lastFailureKey(tenantID, storeID)).Result()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var summary FailureSummary
	if err := json.Unmarshal([]byte(raw), &summary); err != nil {
		return nil, nil
	}
	return &summary, nil
}

func (s *RedisStore) ClearLastFailure(ctx context.Context, tenantID, storeID int64) error {
	return s.client.Del(ctx, lastFailureKey(tenantID, storeID)).Err()
}

func cookieKey(tenantID, storeID int64) string {
	return fmt.Sprintf("%s:%d:%d", cookieKeyPrefix, tenantID, storeID)
}
func verifyCodeKey(tenantID, storeID int64) string {
	return fmt.Sprintf("%s:%d:%d", verifyCodePrefix, tenantID, storeID)
}
func verifyWaitKey(tenantID, storeID int64) string {
	return fmt.Sprintf("%s:%d:%d", verifyWaitPrefix, tenantID, storeID)
}
func lastLoginKey(tenantID, storeID int64) string {
	return fmt.Sprintf("%s:%d:%d", lastLoginTimePrefix, tenantID, storeID)
}
func lastFailureKey(tenantID, storeID int64) string {
	return fmt.Sprintf("%s:%d:%d", lastFailurePrefix, tenantID, storeID)
}
