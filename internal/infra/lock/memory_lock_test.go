package lock

import (
	"context"
	"testing"
	"time"
)

// mockLogger 模拟日志器
type mockLogger struct{}

func (m *mockLogger) Debugf(format string, args ...interface{}) {}
func (m *mockLogger) Infof(format string, args ...interface{})  {}
func (m *mockLogger) Warnf(format string, args ...interface{})  {}
func (m *mockLogger) Errorf(format string, args ...interface{}) {}

func TestMemoryLock_TryLock(t *testing.T) {
	ml := NewMemoryLock(&mockLogger{})
	defer ml.Close()

	ctx := context.Background()
	key := "test-lock"
	ttl := 5 * time.Second

	// 第一次获取锁应该成功
	acquired, err := ml.TryLock(ctx, key, ttl)
	if err != nil {
		t.Fatalf("TryLock failed: %v", err)
	}
	if !acquired {
		t.Fatal("Expected to acquire lock, but failed")
	}

	// 第二次获取同一个锁应该失败
	acquired, err = ml.TryLock(ctx, key, ttl)
	if err != nil {
		t.Fatalf("TryLock failed: %v", err)
	}
	if acquired {
		t.Fatal("Expected to fail acquiring lock, but succeeded")
	}
}

func TestMemoryLock_Unlock(t *testing.T) {
	ml := NewMemoryLock(&mockLogger{})
	defer ml.Close()

	ctx := context.Background()
	key := "test-lock"
	ttl := 5 * time.Second

	// 获取锁
	acquired, err := ml.TryLock(ctx, key, ttl)
	if err != nil || !acquired {
		t.Fatal("Failed to acquire lock")
	}

	// 释放锁
	err = ml.Unlock(ctx, key)
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	// 再次获取锁应该成功
	acquired, err = ml.TryLock(ctx, key, ttl)
	if err != nil || !acquired {
		t.Fatal("Failed to acquire lock after unlock")
	}
}

func TestMemoryLock_Extend(t *testing.T) {
	ml := NewMemoryLock(&mockLogger{})
	defer ml.Close()

	ctx := context.Background()
	key := "test-lock"
	ttl := 1 * time.Second

	// 获取锁
	acquired, err := ml.TryLock(ctx, key, ttl)
	if err != nil || !acquired {
		t.Fatal("Failed to acquire lock")
	}

	// 延长锁
	extended, err := ml.Extend(ctx, key, 5*time.Second)
	if err != nil || !extended {
		t.Fatalf("Failed to extend lock: %v", err)
	}

	// 等待原始TTL过期
	time.Sleep(2 * time.Second)

	// 锁应该仍然有效
	locked, err := ml.IsLocked(ctx, key)
	if err != nil || !locked {
		t.Fatal("Lock should still be active after extension")
	}
}

func TestMemoryLock_IsLocked(t *testing.T) {
	ml := NewMemoryLock(&mockLogger{})
	defer ml.Close()

	ctx := context.Background()
	key := "test-lock"
	ttl := 5 * time.Second

	// 锁不存在时应该返回 false
	locked, err := ml.IsLocked(ctx, key)
	if err != nil || locked {
		t.Fatal("Lock should not exist")
	}

	// 获取锁
	acquired, err := ml.TryLock(ctx, key, ttl)
	if err != nil || !acquired {
		t.Fatal("Failed to acquire lock")
	}

	// 锁存在时应该返回 true
	locked, err = ml.IsLocked(ctx, key)
	if err != nil || !locked {
		t.Fatal("Lock should exist")
	}
}

func TestMemoryLock_Expiration(t *testing.T) {
	ml := NewMemoryLock(&mockLogger{})
	defer ml.Close()

	ctx := context.Background()
	key := "test-lock"
	ttl := 1 * time.Second

	// 获取锁
	acquired, err := ml.TryLock(ctx, key, ttl)
	if err != nil || !acquired {
		t.Fatal("Failed to acquire lock")
	}

	// 等待锁过期
	time.Sleep(2 * time.Second)

	// 锁应该已过期
	locked, err := ml.IsLocked(ctx, key)
	if err != nil || locked {
		t.Fatal("Lock should have expired")
	}

	// 应该能够再次获取锁
	acquired, err = ml.TryLock(ctx, key, ttl)
	if err != nil || !acquired {
		t.Fatal("Failed to acquire lock after expiration")
	}
}
