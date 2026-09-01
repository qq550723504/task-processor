package redis

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type MemoryLock struct {
	locks     map[string]*lockEntry
	mu        sync.RWMutex
	logger    Logger
	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

type lockEntry struct {
	owner     string
	expiresAt time.Time
}

type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

func NewMemoryLock(logger Logger) *MemoryLock {
	ml := &MemoryLock{
		locks:  make(map[string]*lockEntry),
		logger: logger,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}

	go ml.cleanupExpiredLocks()
	return ml
}

func (ml *MemoryLock) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	if entry, exists := ml.locks[key]; exists {
		if time.Now().Before(entry.expiresAt) {
			if ml.logger != nil {
				ml.logger.Debugf("[MemoryLock] 锁 %s 已被持有", key)
			}
			return false, nil
		}
		delete(ml.locks, key)
	}

	owner := fmt.Sprintf("owner-%d", time.Now().UnixNano())
	ml.locks[key] = &lockEntry{owner: owner, expiresAt: time.Now().Add(ttl)}
	if ml.logger != nil {
		ml.logger.Debugf("[MemoryLock] 成功获取锁 %s，TTL: %v", key, ttl)
	}
	return true, nil
}

func (ml *MemoryLock) Unlock(ctx context.Context, key string) error {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	if _, exists := ml.locks[key]; !exists {
		return fmt.Errorf("锁 %s 不存在", key)
	}

	delete(ml.locks, key)
	if ml.logger != nil {
		ml.logger.Debugf("[MemoryLock] 释放锁 %s", key)
	}
	return nil
}

func (ml *MemoryLock) Extend(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	entry, exists := ml.locks[key]
	if !exists {
		return false, fmt.Errorf("锁 %s 不存在", key)
	}

	if time.Now().After(entry.expiresAt) {
		delete(ml.locks, key)
		return false, fmt.Errorf("锁 %s 已过期", key)
	}

	entry.expiresAt = time.Now().Add(ttl)
	if ml.logger != nil {
		ml.logger.Debugf("[MemoryLock] 延长锁 %s 的过期时间，新TTL: %v", key, ttl)
	}
	return true, nil
}

func (ml *MemoryLock) IsLocked(ctx context.Context, key string) (bool, error) {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	entry, exists := ml.locks[key]
	if !exists {
		return false, nil
	}
	if time.Now().After(entry.expiresAt) {
		return false, nil
	}
	return true, nil
}

func (ml *MemoryLock) cleanupExpiredLocks() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	defer close(ml.done)

	for {
		select {
		case now := <-ticker.C:
			ml.removeExpiredLocks(now)
		case <-ml.stop:
			return
		}
	}
}

func (ml *MemoryLock) removeExpiredLocks(now time.Time) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	for key, entry := range ml.locks {
		if now.After(entry.expiresAt) {
			delete(ml.locks, key)
			if ml.logger != nil {
				ml.logger.Debugf("[MemoryLock] 清理过期锁 %s", key)
			}
		}
	}
}

func (ml *MemoryLock) Close() {
	ml.closeOnce.Do(func() {
		close(ml.stop)
		<-ml.done

		ml.mu.Lock()
		ml.locks = make(map[string]*lockEntry)
		ml.mu.Unlock()
		if ml.logger != nil {
			ml.logger.Infof("[MemoryLock] 已关闭并清理所有锁")
		}
	})
}
