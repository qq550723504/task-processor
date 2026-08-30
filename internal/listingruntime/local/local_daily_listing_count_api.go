package local

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"task-processor/internal/listingadmin"

	goredis "github.com/redis/go-redis/v9"
)

const localDailyCountTTL = 30 * 24 * time.Hour

var localDailyQuotaConsumeScript = goredis.NewScript(`
local current = redis.call("GET", KEYS[1])
if not current then
  current = 0
else
  current = tonumber(current) or 0
end

local increment = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local next = current + increment
if next > limit then
  local remaining = limit - current
  if remaining < 0 then
    remaining = 0
  end
  return {0, current, remaining, current >= limit and 1 or 0}
end

redis.call("SET", KEYS[1], next, "PX", ARGV[3])
local remaining = limit - next
if remaining < 0 then
  remaining = 0
end
return {1, next, remaining, next >= limit and 1 or 0}
`)

type localDailyListingCountAPI struct {
	redis *goredis.Client
}

func NewLocalDailyListingCountAPI(redis *goredis.Client) listingadmin.DailyListingCountAPI {
	if redis == nil {
		return nil
	}
	return &localDailyListingCountAPI{redis: redis}
}

func (a *localDailyListingCountAPI) GetDailyListingCount(tenantID, storeID, userID int64, date string) (*listingadmin.DailyListingCountRespDTO, error) {
	if a == nil || a.redis == nil {
		return nil, nil
	}
	key := fmt.Sprintf("listing:daily:count:%d:%d:%s", tenantID, storeID, date)
	val, err := a.redis.Get(context.Background(), key).Result()
	if err == goredis.Nil {
		return &listingadmin.DailyListingCountRespDTO{TenantID: tenantID, StoreID: storeID, UserID: userID, Date: date, Count: 0}, nil
	}
	if err != nil {
		return nil, err
	}
	count, _ := strconv.ParseInt(val, 10, 64)
	return &listingadmin.DailyListingCountRespDTO{TenantID: tenantID, StoreID: storeID, UserID: userID, Date: date, Count: count}, nil
}

func (a *localDailyListingCountAPI) SetDailyListingCount(req *listingadmin.DailyListingCountSetReqDTO) error {
	if a == nil || a.redis == nil || req == nil {
		return nil
	}
	key := fmt.Sprintf("listing:daily:count:%d:%d:%s", req.TenantID, req.StoreID, req.Date)
	return a.redis.Set(context.Background(), key, strconv.FormatInt(req.Count, 10), localDailyCountTTL).Err()
}

func (a *localDailyListingCountAPI) TryConsumeDailyQuota(req *listingadmin.TryConsumeDailyQuotaReqDTO) (*listingadmin.TryConsumeDailyQuotaRespDTO, error) {
	if a == nil || a.redis == nil || req == nil {
		return nil, nil
	}
	key := fmt.Sprintf("listing:daily:count:%d:%d:%s", req.TenantID, req.StoreID, req.Date)
	result, err := localDailyQuotaConsumeScript.Run(
		context.Background(),
		a.redis,
		[]string{key},
		req.Increment,
		req.Limit,
		localDailyCountTTL.Milliseconds(),
	).Result()
	if err != nil {
		return nil, err
	}
	values, ok := result.([]interface{})
	if !ok || len(values) != 4 {
		return nil, fmt.Errorf("unexpected daily quota script result %T", result)
	}
	allowed, err := dailyQuotaScriptInt64(values[0])
	if err != nil {
		return nil, err
	}
	count, err := dailyQuotaScriptInt64(values[1])
	if err != nil {
		return nil, err
	}
	remaining, err := dailyQuotaScriptInt64(values[2])
	if err != nil {
		return nil, err
	}
	reachedLimit, err := dailyQuotaScriptInt64(values[3])
	if err != nil {
		return nil, err
	}
	return &listingadmin.TryConsumeDailyQuotaRespDTO{
		Allowed:      allowed != 0,
		NewCount:     count,
		Remaining:    remaining,
		ReachedLimit: reachedLimit != 0,
	}, nil
}

func dailyQuotaScriptInt64(value interface{}) (int64, error) {
	switch value := value.(type) {
	case int64:
		return value, nil
	case string:
		return strconv.ParseInt(value, 10, 64)
	case []byte:
		return strconv.ParseInt(string(value), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected daily quota script value %T", value)
	}
}

func (a *localDailyListingCountAPI) RollbackDailyQuota(req *listingadmin.RollbackDailyQuotaReqDTO) (int64, error) {
	if a == nil || a.redis == nil || req == nil {
		return 0, nil
	}
	resp, err := a.GetDailyListingCount(req.TenantID, req.StoreID, req.UserID, req.Date)
	if err != nil {
		return 0, err
	}
	next := resp.Count - req.Decrement
	if next < 0 {
		next = 0
	}
	return next, a.SetDailyListingCount(&listingadmin.DailyListingCountSetReqDTO{TenantID: req.TenantID, StoreID: req.StoreID, UserID: req.UserID, Date: req.Date, Count: next})
}

func (a *localDailyListingCountAPI) SetRemainingListingQuota(tenantID, storeID int64, quota int) (bool, error) {
	if a == nil || a.redis == nil {
		return false, nil
	}
	key := fmt.Sprintf("listing:remaining:quota:%d:%d", tenantID, storeID)
	err := a.redis.Set(context.Background(), key, strconv.Itoa(quota), 0).Err()
	return err == nil, err
}
