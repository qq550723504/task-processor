package productenrich

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type memRedisEntry struct {
	value     string
	expiresAt time.Time
}

type MemRedisClient struct {
	mu    sync.RWMutex
	store map[string]memRedisEntry
	lists map[string][]string
}

func NewMemRedisClient() RedisClient {
	return &MemRedisClient{
		store: make(map[string]memRedisEntry),
		lists: make(map[string][]string),
	}
}

func (r *MemRedisClient) Push(_ context.Context, key string, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lists[key] = append(r.lists[key], value)
	return nil
}

func (r *MemRedisClient) Get(_ context.Context, key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.store[key]
	if !ok {
		return "", fmt.Errorf("key not found: %s", key)
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		return "", fmt.Errorf("key expired: %s", key)
	}
	return entry.value, nil
}

func (r *MemRedisClient) Set(_ context.Context, key string, value string, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := memRedisEntry{value: value}
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}
	r.store[key] = entry
	return nil
}

func (r *MemRedisClient) Delete(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.store, key)
	delete(r.lists, key)
	return nil
}
