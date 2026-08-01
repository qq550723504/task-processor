package sheinlogin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/core/config"

	goredis "github.com/redis/go-redis/v9"
)

const (
	cookieKeyPrefix           = "shein:cookie"
	verifyCodePrefix          = "shein:verify_code"
	verifyCodeQueuePrefix     = "shein:verify_code_queue"
	verifyAttemptCodePrefix   = "shein:verify_code_attempt"
	verifyAttemptQueuePrefix  = "shein:verify_code_attempt_queue"
	verifyWaitPrefix          = "shein:wait_verify_code"
	autoVerifyCodePrefix      = "shein:auto_verify_code_count"
	lastLoginTimePrefix       = "shein:last_login_time"
	lastFailurePrefix         = "shein:last_failure"
	lastFailureDetailPrefix   = "shein:last_failure_detail"
	loginAttemptPrefix        = "shein:login_attempt"
	loginAttemptActivePrefix  = "shein:login_attempt_active"
	loginAttemptLatestPrefix  = "shein:login_attempt_latest"
	loginAttemptStream        = "shein:login_attempts"
	loginAttemptGroup         = "shein-login-workers"
	loginAttemptControlPrefix = "shein:login_attempt_control"
)

const (
	loginAttemptTTL       = 30 * 24 * time.Hour
	activeAttemptLeaseTTL = 15 * time.Minute
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

func (s *RedisStore) LoadCookieState(ctx context.Context, tenantID, storeID int64) (string, bool, error) {
	raw, err := s.client.Get(ctx, cookieKey(tenantID, storeID)).Result()
	if err == goredis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return raw, strings.TrimSpace(raw) != "", nil
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

func (s *RedisStore) AutoVerifyCodeSendCount(ctx context.Context, tenantID, storeID int64, day time.Time) (int64, error) {
	raw, err := s.client.Get(ctx, autoVerifyCodeCountKey(tenantID, storeID, day)).Int64()
	if err == goredis.Nil {
		return 0, nil
	}
	return raw, err
}

func (s *RedisStore) RecordAutoVerifyCodeSent(ctx context.Context, tenantID, storeID int64, day time.Time) (int64, error) {
	key := autoVerifyCodeCountKey(tenantID, storeID, day)
	count, err := s.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		_ = s.client.ExpireAt(ctx, key, nextLocalMidnight(day)).Err()
	}
	return count, nil
}

func (s *RedisStore) SubmitVerifyCode(ctx context.Context, tenantID, storeID int64, code string, ttl time.Duration) error {
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, verifyCodeKey(tenantID, storeID), code, ttl)
	pipe.Set(ctx, verifyWaitKey(tenantID, storeID), "waiting", ttl)
	pipe.Del(ctx, verifyCodeQueueKey(tenantID, storeID))
	pipe.RPush(ctx, verifyCodeQueueKey(tenantID, storeID), code)
	pipe.Expire(ctx, verifyCodeQueueKey(tenantID, storeID), ttl)
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
	pipe.Del(ctx, verifyCodeQueueKey(tenantID, storeID))
	_, execErr := pipe.Exec(ctx)
	return value, true, execErr
}

func (s *RedisStore) WaitAndConsumeVerifyCode(ctx context.Context, tenantID, storeID int64, timeout time.Duration) (string, bool, error) {
	if code, ok, err := s.ConsumeVerifyCode(ctx, tenantID, storeID); err != nil || ok {
		return code, ok, err
	}
	if timeout <= 0 {
		return "", false, nil
	}

	values, err := s.client.BLPop(ctx, timeout, verifyCodeQueueKey(tenantID, storeID)).Result()
	if err == goredis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if len(values) != 2 {
		return "", false, nil
	}
	code := strings.TrimSpace(values[1])
	if code == "" {
		return "", false, nil
	}

	pipe := s.client.TxPipeline()
	pipe.Del(ctx, verifyCodeKey(tenantID, storeID))
	pipe.Del(ctx, verifyWaitKey(tenantID, storeID))
	pipe.Del(ctx, verifyCodeQueueKey(tenantID, storeID))
	_, execErr := pipe.Exec(ctx)
	if execErr != nil {
		return "", false, execErr
	}
	return code, true, nil
}

func (s *RedisStore) SubmitVerifyCodeForAttempt(ctx context.Context, tenantID, storeID int64, attemptID, code string, ttl time.Duration) error {
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, verifyAttemptCodeKey(tenantID, storeID, attemptID), code, ttl)
	pipe.Set(ctx, verifyWaitKey(tenantID, storeID), "waiting", ttl)
	pipe.Del(ctx, verifyAttemptQueueKey(tenantID, storeID, attemptID))
	pipe.RPush(ctx, verifyAttemptQueueKey(tenantID, storeID, attemptID), code)
	pipe.Expire(ctx, verifyAttemptQueueKey(tenantID, storeID, attemptID), ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStore) WaitAndConsumeVerifyCodeForAttempt(ctx context.Context, tenantID, storeID int64, attemptID string, timeout time.Duration) (string, bool, error) {
	codeKey := verifyAttemptCodeKey(tenantID, storeID, attemptID)
	queueKey := verifyAttemptQueueKey(tenantID, storeID, attemptID)
	if value, err := s.client.Get(ctx, codeKey).Result(); err == nil {
		pipe := s.client.TxPipeline()
		pipe.Del(ctx, codeKey)
		pipe.Del(ctx, verifyWaitKey(tenantID, storeID))
		pipe.Del(ctx, queueKey)
		_, execErr := pipe.Exec(ctx)
		return value, strings.TrimSpace(value) != "", execErr
	} else if err != goredis.Nil {
		return "", false, err
	}
	if timeout <= 0 {
		return "", false, nil
	}
	values, err := s.client.BLPop(ctx, timeout, queueKey).Result()
	if err == goredis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if len(values) != 2 {
		return "", false, nil
	}
	code := strings.TrimSpace(values[1])
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, codeKey)
	pipe.Del(ctx, verifyWaitKey(tenantID, storeID))
	pipe.Del(ctx, queueKey)
	_, execErr := pipe.Exec(ctx)
	return code, code != "", execErr
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
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, lastFailureKey(tenantID, storeID), body, ttl)
	pipe.Del(ctx, lastFailureDetailKey(tenantID, storeID))
	_, err = pipe.Exec(ctx)
	return err
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
	return s.client.Del(ctx, lastFailureKey(tenantID, storeID), lastFailureDetailKey(tenantID, storeID)).Err()
}

func (s *RedisStore) RecordLastFailureDetail(ctx context.Context, tenantID, storeID int64, detail *FailureDetail, ttl time.Duration) error {
	if detail == nil {
		return s.client.Del(ctx, lastFailureDetailKey(tenantID, storeID)).Err()
	}
	body, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, lastFailureDetailKey(tenantID, storeID), body, ttl).Err()
}

func (s *RedisStore) LastFailureDetail(ctx context.Context, tenantID, storeID int64) (*FailureDetail, error) {
	raw, err := s.client.Get(ctx, lastFailureDetailKey(tenantID, storeID)).Result()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var detail FailureDetail
	if err := json.Unmarshal([]byte(raw), &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

// EnqueueLoginAttempt creates at most one active login attempt for a store and
// writes only store identifiers and requested browser options to Redis. Store
// credentials intentionally never cross this queue boundary.
func (s *RedisStore) EnqueueLoginAttempt(ctx context.Context, tenantID, storeID int64, req LoginRequest) (*LoginAttempt, bool, error) {
	id, err := newLoginAttemptID()
	if err != nil {
		return nil, false, err
	}
	attempt := &LoginAttempt{
		ID:         id,
		TenantID:   tenantID,
		StoreID:    storeID,
		ForceLogin: req.ForceLogin,
		Headless:   req.Headless,
		Status:     LoginAttemptQueued,
		CreatedAt:  time.Now().UTC(),
	}
	body, err := json.Marshal(attempt)
	if err != nil {
		return nil, false, err
	}

	const createAttemptScript = `
local active = redis.call("GET", KEYS[1])
if active then return {0, active} end
redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2])
redis.call("SET", KEYS[2], ARGV[3], "PX", ARGV[4])
redis.call("SET", KEYS[3], ARGV[1], "PX", ARGV[4])
redis.call("XADD", KEYS[4], "*", "attempt_id", ARGV[1])
return {1, ARGV[1]}`
	result, err := s.client.Eval(ctx, createAttemptScript,
		[]string{
			loginAttemptActiveKey(tenantID, storeID),
			loginAttemptKey(attempt.ID),
			loginAttemptLatestKey(tenantID, storeID),
			loginAttemptStream,
		},
		attempt.ID,
		activeAttemptLeaseTTL.Milliseconds(),
		body,
		loginAttemptTTL.Milliseconds(),
	).Result()
	if err != nil {
		return nil, false, err
	}
	values, ok := result.([]any)
	if !ok || len(values) != 2 {
		return nil, false, fmt.Errorf("unexpected login attempt create result: %v", result)
	}
	created, ok := values[0].(int64)
	if !ok {
		return nil, false, fmt.Errorf("unexpected login attempt create flag: %v", values[0])
	}
	if created == 1 {
		return attempt, true, nil
	}
	existingID, ok := values[1].(string)
	if !ok || strings.TrimSpace(existingID) == "" {
		return nil, false, fmt.Errorf("unexpected active login attempt id: %v", values[1])
	}
	existing, err := s.LoadLoginAttempt(ctx, existingID)
	if err != nil {
		return nil, false, err
	}
	if existing == nil {
		return nil, false, fmt.Errorf("active login attempt %q is still being created; retry the request", existingID)
	}
	return existing, false, nil
}

func (s *RedisStore) LoadLoginAttempt(ctx context.Context, attemptID string) (*LoginAttempt, error) {
	raw, err := s.client.Get(ctx, loginAttemptKey(attemptID)).Result()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var attempt LoginAttempt
	if err := json.Unmarshal([]byte(raw), &attempt); err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (s *RedisStore) LatestLoginAttempt(ctx context.Context, tenantID, storeID int64) (*LoginAttempt, error) {
	id, err := s.client.Get(ctx, loginAttemptLatestKey(tenantID, storeID)).Result()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.LoadLoginAttempt(ctx, id)
}

func (s *RedisStore) UpdateLoginAttempt(ctx context.Context, attempt *LoginAttempt) error {
	if attempt == nil || strings.TrimSpace(attempt.ID) == "" {
		return fmt.Errorf("login attempt is required")
	}
	if err := s.saveLoginAttempt(ctx, attempt); err != nil {
		return err
	}
	if attempt.Status.IsActive() {
		return s.refreshLoginAttemptLease(ctx, attempt)
	}
	return s.releaseLoginAttemptLease(ctx, attempt)
}

func (s *RedisStore) EnsureLoginAttemptConsumerGroup(ctx context.Context) error {
	err := s.client.XGroupCreateMkStream(ctx, loginAttemptStream, loginAttemptGroup, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

func (s *RedisStore) ReadLoginAttemptJobs(ctx context.Context, consumer string, block time.Duration) ([]goredis.XMessage, error) {
	streams, err := s.client.XReadGroup(ctx, &goredis.XReadGroupArgs{
		Group:    loginAttemptGroup,
		Consumer: consumer,
		Streams:  []string{loginAttemptStream, ">"},
		Count:    1,
		Block:    block,
		NoAck:    false,
	}).Result()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var messages []goredis.XMessage
	for _, stream := range streams {
		messages = append(messages, stream.Messages...)
	}
	return messages, nil
}

func (s *RedisStore) AcknowledgeLoginAttemptJob(ctx context.Context, messageID string) error {
	return s.client.XAck(ctx, loginAttemptStream, loginAttemptGroup, messageID).Err()
}

func (s *RedisStore) ClaimStaleLoginAttemptJobs(ctx context.Context, consumer string, minIdle time.Duration) ([]goredis.XMessage, error) {
	messages, _, err := s.client.XAutoClaim(ctx, &goredis.XAutoClaimArgs{
		Stream:   loginAttemptStream,
		Group:    loginAttemptGroup,
		Consumer: consumer,
		MinIdle:  minIdle,
		Start:    "0-0",
		Count:    10,
	}).Result()
	if err == goredis.Nil {
		return nil, nil
	}
	return messages, err
}

// CancelLoginAttempt durably signals the worker that owns the browser session
// and marks the attempt terminal before allowing another attempt for the store.
func (s *RedisStore) CancelLoginAttempt(ctx context.Context, tenantID, storeID int64, message string) (*LoginAttempt, bool, error) {
	attempt, err := s.LatestLoginAttempt(ctx, tenantID, storeID)
	if err != nil || attempt == nil || !attempt.Status.IsActive() {
		return attempt, false, err
	}
	now := time.Now().UTC()
	attempt.Status = LoginAttemptCancelled
	attempt.Message = message
	attempt.ErrorCode = "LOGIN_CANCELLED"
	attempt.CompletedAt = &now
	body, err := json.Marshal(attempt)
	if err != nil {
		return nil, false, err
	}
	const cancelAttemptScript = `
redis.call("SET", KEYS[2], ARGV[2], "PX", ARGV[3])
redis.call("SET", KEYS[3], ARGV[1], "PX", ARGV[3])
redis.call("RPUSH", KEYS[4], "cancel")
redis.call("PEXPIRE", KEYS[4], ARGV[4])
if redis.call("GET", KEYS[1]) == ARGV[1] then redis.call("DEL", KEYS[1]) end
return 1`
	if err := s.client.Eval(ctx, cancelAttemptScript,
		[]string{
			loginAttemptActiveKey(tenantID, storeID),
			loginAttemptKey(attempt.ID),
			loginAttemptLatestKey(tenantID, storeID),
			loginAttemptControlKey(attempt.ID),
		},
		attempt.ID,
		body,
		loginAttemptTTL.Milliseconds(),
		activeAttemptLeaseTTL.Milliseconds(),
	).Err(); err != nil {
		return nil, false, err
	}
	return attempt, true, nil
}

// WaitAndConsumeVerifyCodeOrCancel wakes for either the matching verification
// code or a cancellation command sent by the API to the session-owning worker.
func (s *RedisStore) WaitAndConsumeVerifyCodeOrCancel(ctx context.Context, tenantID, storeID int64, attemptID string, timeout time.Duration) (code string, received bool, cancelled bool, err error) {
	controlKey := loginAttemptControlKey(attemptID)
	if command, popErr := s.client.LPop(ctx, controlKey).Result(); popErr == nil {
		return "", false, strings.EqualFold(strings.TrimSpace(command), "cancel"), nil
	} else if popErr != goredis.Nil {
		return "", false, false, popErr
	}
	codeKey := verifyAttemptCodeKey(tenantID, storeID, attemptID)
	queueKey := verifyAttemptQueueKey(tenantID, storeID, attemptID)
	if value, getErr := s.client.Get(ctx, codeKey).Result(); getErr == nil {
		pipe := s.client.TxPipeline()
		pipe.Del(ctx, codeKey, verifyWaitKey(tenantID, storeID), queueKey)
		_, execErr := pipe.Exec(ctx)
		return value, strings.TrimSpace(value) != "", false, execErr
	} else if getErr != goredis.Nil {
		return "", false, false, getErr
	}
	if timeout <= 0 {
		return "", false, false, nil
	}
	deadline := time.Now().Add(timeout)
	for {
		if err := ctx.Err(); err != nil {
			return "", false, false, err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return "", false, false, nil
		}
		block := time.Second
		if remaining < block {
			block = remaining
		}
		values, popErr := s.client.BLPop(ctx, block, queueKey, controlKey).Result()
		if popErr == goredis.Nil {
			continue
		}
		if popErr != nil {
			return "", false, false, popErr
		}
		if len(values) != 2 {
			continue
		}
		if values[0] == controlKey {
			return "", false, strings.EqualFold(strings.TrimSpace(values[1]), "cancel"), nil
		}
		code = strings.TrimSpace(values[1])
		pipe := s.client.TxPipeline()
		pipe.Del(ctx, codeKey, verifyWaitKey(tenantID, storeID), queueKey)
		_, execErr := pipe.Exec(ctx)
		return code, code != "", false, execErr
	}
}

func (s *RedisStore) saveLoginAttempt(ctx context.Context, attempt *LoginAttempt) error {
	body, err := json.Marshal(attempt)
	if err != nil {
		return err
	}
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, loginAttemptKey(attempt.ID), body, loginAttemptTTL)
	pipe.Set(ctx, loginAttemptLatestKey(attempt.TenantID, attempt.StoreID), attempt.ID, loginAttemptTTL)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) releaseLoginAttemptLease(ctx context.Context, attempt *LoginAttempt) error {
	const releaseLeaseScript = `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) end return 0`
	return s.client.Eval(ctx, releaseLeaseScript, []string{loginAttemptActiveKey(attempt.TenantID, attempt.StoreID)}, attempt.ID).Err()
}

func (s *RedisStore) refreshLoginAttemptLease(ctx context.Context, attempt *LoginAttempt) error {
	const refreshLeaseScript = `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("PEXPIRE", KEYS[1], ARGV[2]) end return 0`
	return s.client.Eval(ctx, refreshLeaseScript, []string{loginAttemptActiveKey(attempt.TenantID, attempt.StoreID)}, attempt.ID, activeAttemptLeaseTTL.Milliseconds()).Err()
}

func newLoginAttemptID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
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
func autoVerifyCodeCountKey(tenantID, storeID int64, day time.Time) string {
	localDay := day.In(sheinLoginLocalLocation())
	return fmt.Sprintf("%s:%d:%d:%s", autoVerifyCodePrefix, tenantID, storeID, localDay.Format("20060102"))
}
func verifyCodeQueueKey(tenantID, storeID int64) string {
	return fmt.Sprintf("%s:%d:%d", verifyCodeQueuePrefix, tenantID, storeID)
}
func verifyAttemptCodeKey(tenantID, storeID int64, attemptID string) string {
	return fmt.Sprintf("%s:%d:%d:%s", verifyAttemptCodePrefix, tenantID, storeID, attemptID)
}
func verifyAttemptQueueKey(tenantID, storeID int64, attemptID string) string {
	return fmt.Sprintf("%s:%d:%d:%s", verifyAttemptQueuePrefix, tenantID, storeID, attemptID)
}
func lastLoginKey(tenantID, storeID int64) string {
	return fmt.Sprintf("%s:%d:%d", lastLoginTimePrefix, tenantID, storeID)
}
func lastFailureKey(tenantID, storeID int64) string {
	return fmt.Sprintf("%s:%d:%d", lastFailurePrefix, tenantID, storeID)
}
func lastFailureDetailKey(tenantID, storeID int64) string {
	return fmt.Sprintf("%s:%d:%d", lastFailureDetailPrefix, tenantID, storeID)
}
func loginAttemptKey(attemptID string) string {
	return fmt.Sprintf("%s:%s", loginAttemptPrefix, attemptID)
}
func loginAttemptActiveKey(tenantID, storeID int64) string {
	return fmt.Sprintf("%s:%d:%d", loginAttemptActivePrefix, tenantID, storeID)
}
func loginAttemptLatestKey(tenantID, storeID int64) string {
	return fmt.Sprintf("%s:%d:%d", loginAttemptLatestPrefix, tenantID, storeID)
}
func loginAttemptControlKey(attemptID string) string {
	return fmt.Sprintf("%s:%s", loginAttemptControlPrefix, attemptID)
}

func nextLocalMidnight(day time.Time) time.Time {
	local := day.In(sheinLoginLocalLocation())
	y, m, d := local.Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, local.Location())
}

func sheinLoginLocalLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Local
	}
	return loc
}
