package local

import (
	"context"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/listingadmin"

	goredis "github.com/redis/go-redis/v9"
)

type localStoreRuntimeState struct {
	resources *RuntimeResources
	storeAPI  listingadmin.StoreAPI
}

func newLocalStoreRuntimeState(resources *RuntimeResources, storeAPI listingadmin.StoreAPI) *localStoreRuntimeState {
	if resources == nil || resources.redis == nil || storeAPI == nil {
		return nil
	}
	return &localStoreRuntimeState{resources: resources, storeAPI: storeAPI}
}

func (s *localStoreRuntimeState) GetStorePauseStatus(id int64) (bool, error) {
	detail, err := s.GetStorePauseStatusDetail(id)
	if err != nil || detail == nil {
		return false, err
	}
	return detail.Paused, nil
}

func (s *localStoreRuntimeState) GetStorePauseStatusDetail(id int64) (*listingadmin.StorePauseStatusRespDTO, error) {
	store, err := s.store(id)
	if err != nil || store == nil {
		return nil, err
	}
	key := localStorePauseKey(store)
	value, err := s.resources.redis.Get(context.Background(), key).Result()
	if err == goredis.Nil {
		return &listingadmin.StorePauseStatusRespDTO{}, nil
	}
	if err != nil {
		return nil, err
	}
	ttl, _ := s.resources.redis.TTL(context.Background(), key).Result()
	return &listingadmin.StorePauseStatusRespDTO{
		Paused:     true,
		Reason:     value,
		TTLSeconds: int64(ttl.Seconds()),
	}, nil
}

func (s *localStoreRuntimeState) SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error) {
	store, err := s.store(id)
	if err != nil || store == nil {
		return false, err
	}
	key := localStorePauseKey(store)
	if !pause {
		result := s.resources.redis.Del(context.Background(), key)
		return result.Err() == nil, result.Err()
	}
	err = s.resources.redis.Set(context.Background(), key, pauseType, 24*time.Hour).Err()
	return err == nil, err
}

func (s *localStoreRuntimeState) DeleteStoreCookie(id int64) (bool, error) {
	store, err := s.store(id)
	if err != nil || store == nil {
		return false, err
	}
	ctx := context.Background()
	platform := strings.ToLower(store.Platform)
	lastLoginKey := platform + ":last_login_time:" + strconv.FormatInt(store.TenantID, 10) + ":" + strconv.FormatInt(store.ID, 10)
	lastLoginTime, getErr := s.resources.redis.Get(ctx, lastLoginKey).Result()
	if getErr != nil && getErr != goredis.Nil {
		return false, getErr
	}
	if getErr == nil {
		lastLoginAt, parseErr := strconv.ParseFloat(strings.TrimSpace(lastLoginTime), 64)
		if parseErr == nil && float64(time.Now().Unix())-lastLoginAt < 300 {
			return false, nil
		}
	}
	key := platform + ":cookie:" + strconv.FormatInt(store.TenantID, 10) + ":" + strconv.FormatInt(store.ID, 10)
	deleted, err := s.resources.redis.Del(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return deleted > 0, nil
}

func (s *localStoreRuntimeState) store(id int64) (*listingadmin.StoreRespDTO, error) {
	if s == nil || s.resources == nil || s.resources.redis == nil || s.storeAPI == nil {
		return nil, nil
	}
	return s.storeAPI.GetStore(id)
}

func localStorePauseKey(store *listingadmin.StoreRespDTO) string {
	return "listing:task:pause:" + strings.ToLower(store.Platform) + ":" + strconv.FormatInt(store.TenantID, 10) + ":" + strconv.FormatInt(store.ID, 10)
}
