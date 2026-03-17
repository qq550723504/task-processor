// Package lock 提供分布式锁功能
package lock

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryLock 基于内存的锁实现（仅用于单机测试）
// 注意：不适用于生产环境的分布式场景
type MemoryLock struct {
	locks  map[string]*lockEntry
	mu     sync.RWMutex
	logger Logger
}

// lockEntry 锁条目
type lockEntry struct {
	owner     string
	expiresAt time.Time
}

// Logger 日志接口
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// NewMemoryLock 创建内存锁
func NewMemoryLock(logger Logger) *MemoryLock {
	ml := &MemoryLock{
		locks:  make(map[string]*lockEntry),
		logger: logger,
	}

	// 启动清理过期锁的goroutine
	go ml.cleanupExpiredLocks()

	return ml
}

// TryLock 尝试获取锁
func (ml *MemoryLock) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	// 检查锁是否存在且未过期
	if entry, exists := ml.locks[key]; exists {
		if time.Now().Before(entry.expiresAt) {
			ml.logger.Debugf("[MemoryLock] 锁 %s 已被持有", key)
			return false, nil
		}
		// 锁已过期，删除
		delete(ml.locks, key)
	}

	// 创建新锁
	owner := fmt.Sprintf("owner-%d", time.Now().UnixNano())
	ml.locks[key] = &lockEntry{
		owner:     owner,
		expiresAt: time.Now().Add(ttl),
	}

	ml.logger.Debugf("[MemoryLock] 成功获取锁 %s，TTL: %v", key, ttl)
	return true, nil
}

// Unlock 释放锁
func (ml *MemoryLock) Unlock(ctx context.Context, key string) error {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	if _, exists := ml.locks[key]; !exists {
		return fmt.Errorf("锁 %s 不存在", key)
	}

	delete(ml.locks, key)
	ml.logger.Debugf("[MemoryLock] 释放锁 %s", key)
	return nil
}

// Extend 延长锁的过期时间
func (ml *MemoryLock) Extend(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	entry, exists := ml.locks[key]
	if !exists {
		return false, fmt.Errorf("锁 %s 不存在", key)
	}

	// 检查锁是否已过期
	if time.Now().After(entry.expiresAt) {
		delete(ml.locks, key)
		return false, fmt.Errorf("锁 %s 已过期", key)
	}

	// 延长过期时间
	entry.expiresAt = time.Now().Add(ttl)
	ml.logger.Debugf("[MemoryLock] 延长锁 %s 的过期时间，新TTL: %v", key, ttl)
	return true, nil
}

// IsLocked 检查锁是否被持有
func (ml *MemoryLock) IsLocked(ctx context.Context, key string) (bool, error) {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	entry, exists := ml.locks[key]
	if !exists {
		return false, nil
	}

	// 检查是否过期
	if time.Now().After(entry.expiresAt) {
		return false, nil
	}

	return true, nil
}

// cleanupExpiredLocks 清理过期的锁
func (ml *MemoryLock) cleanupExpiredLocks() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ml.mu.Lock()
		now := time.Now()
		for key, entry := range ml.locks {
			if now.After(entry.expiresAt) {
				delete(ml.locks, key)
				ml.logger.Debugf("[MemoryLock] 清理过期锁 %s", key)
			}
		}
		ml.mu.Unlock()
	}
}

// Close 关闭内存锁（清理资源）
func (ml *MemoryLock) Close() {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	ml.locks = make(map[string]*lockEntry)
	ml.logger.Infof("[MemoryLock] 已关闭并清理所有锁")
}
