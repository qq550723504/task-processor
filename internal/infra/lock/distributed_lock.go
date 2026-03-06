// Package lock 提供分布式锁功能
package lock

import (
	"context"
	"time"
)

// DistributedLock 分布式锁接口
type DistributedLock interface {
	// TryLock 尝试获取锁
	// key: 锁的键名
	// ttl: 锁的过期时间
	// 返回: 是否成功获取锁，错误信息
	TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error)

	// Unlock 释放锁
	// key: 锁的键名
	// 返回: 错误信息
	Unlock(ctx context.Context, key string) error

	// Extend 延长锁的过期时间
	// key: 锁的键名
	// ttl: 延长的时间
	// 返回: 是否成功延长，错误信息
	Extend(ctx context.Context, key string, ttl time.Duration) (bool, error)

	// IsLocked 检查锁是否被持有
	// key: 锁的键名
	// 返回: 是否被锁定，错误信息
	IsLocked(ctx context.Context, key string) (bool, error)
}

// LockOptions 锁配置选项
type LockOptions struct {
	// RetryCount 重试次数
	RetryCount int
	// RetryDelay 重试延迟
	RetryDelay time.Duration
	// AutoRenew 是否自动续期
	AutoRenew bool
	// RenewInterval 续期间隔
	RenewInterval time.Duration
}

// DefaultLockOptions 默认锁配置
func DefaultLockOptions() *LockOptions {
	return &LockOptions{
		RetryCount:    3,
		RetryDelay:    100 * time.Millisecond,
		AutoRenew:     false,
		RenewInterval: 5 * time.Second,
	}
}
