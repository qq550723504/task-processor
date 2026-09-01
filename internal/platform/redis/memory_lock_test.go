package redis

import (
	"context"
	"testing"
	"time"
)

type mockLogger struct{}

func (m *mockLogger) Debugf(format string, args ...any) {}
func (m *mockLogger) Infof(format string, args ...any)  {}
func (m *mockLogger) Warnf(format string, args ...any)  {}
func (m *mockLogger) Errorf(format string, args ...any) {}

func TestMemoryLock_TryLock(t *testing.T) {
	ml := NewMemoryLock(&mockLogger{})
	defer ml.Close()

	ctx := context.Background()
	key := "test-lock"
	ttl := 5 * time.Second

	acquired, err := ml.TryLock(ctx, key, ttl)
	if err != nil {
		t.Fatalf("TryLock failed: %v", err)
	}
	if !acquired {
		t.Fatal("Expected to acquire lock, but failed")
	}

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

	acquired, err := ml.TryLock(ctx, key, ttl)
	if err != nil || !acquired {
		t.Fatal("Failed to acquire lock")
	}

	err = ml.Unlock(ctx, key)
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

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

	acquired, err := ml.TryLock(ctx, key, ttl)
	if err != nil || !acquired {
		t.Fatal("Failed to acquire lock")
	}

	extended, err := ml.Extend(ctx, key, 5*time.Second)
	if err != nil || !extended {
		t.Fatalf("Failed to extend lock: %v", err)
	}

	time.Sleep(2 * time.Second)

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

	locked, err := ml.IsLocked(ctx, key)
	if err != nil || locked {
		t.Fatal("Lock should not exist")
	}

	acquired, err := ml.TryLock(ctx, key, ttl)
	if err != nil || !acquired {
		t.Fatal("Failed to acquire lock")
	}

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

	acquired, err := ml.TryLock(ctx, key, ttl)
	if err != nil || !acquired {
		t.Fatal("Failed to acquire lock")
	}

	time.Sleep(2 * time.Second)

	locked, err := ml.IsLocked(ctx, key)
	if err != nil || locked {
		t.Fatal("Lock should have expired")
	}

	acquired, err = ml.TryLock(ctx, key, ttl)
	if err != nil || !acquired {
		t.Fatal("Failed to acquire lock after expiration")
	}
}

func TestMemoryLockNilLoggerIsSafeAcrossLifecycle(t *testing.T) {
	ml := NewMemoryLock(nil)
	ctx := context.Background()

	acquired, err := ml.TryLock(ctx, "test-lock", time.Minute)
	if err != nil || !acquired {
		t.Fatalf("TryLock() = %v, %v", acquired, err)
	}
	acquired, err = ml.TryLock(ctx, "test-lock", time.Minute)
	if err != nil || acquired {
		t.Fatalf("second TryLock() = %v, %v", acquired, err)
	}
	extended, err := ml.Extend(ctx, "test-lock", 2*time.Minute)
	if err != nil || !extended {
		t.Fatalf("Extend() = %v, %v", extended, err)
	}
	if err := ml.Unlock(ctx, "test-lock"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}

	ml.mu.Lock()
	ml.locks["expired"] = &lockEntry{expiresAt: time.Now().Add(-time.Second)}
	ml.mu.Unlock()
	ml.removeExpiredLocks(time.Now())
	if locked, err := ml.IsLocked(ctx, "expired"); err != nil || locked {
		t.Fatalf("expired IsLocked() = %v, %v", locked, err)
	}

	ml.Close()
}

func TestMemoryLockCloseStopsCleanupAndIsIdempotent(t *testing.T) {
	ml := NewMemoryLock(&mockLogger{})
	done := ml.done

	closed := make(chan struct{})
	go func() {
		ml.Close()
		ml.Close()
		close(closed)
	}()

	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close did not return after stopping cleanup")
	}

	select {
	case <-done:
	default:
		t.Fatal("Close returned before cleanup goroutine stopped")
	}
}
