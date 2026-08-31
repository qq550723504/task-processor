package redis

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisLockOnlyOwnerCanUnlock(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	defer client.Close()

	ctx := context.Background()
	first := NewRedisLockWithClient(client, "pod-1", nil)
	second := NewRedisLockWithClient(client, "pod-2", nil)

	locked, err := first.TryLock(ctx, "listing:scheduler:lock:SHEIN:inventory:962", time.Minute)
	if err != nil {
		t.Fatalf("TryLock() error = %v", err)
	}
	if !locked {
		t.Fatal("expected first owner to acquire lock")
	}

	if err := second.Unlock(ctx, "listing:scheduler:lock:SHEIN:inventory:962"); err != nil {
		t.Fatalf("second owner Unlock() error = %v", err)
	}
	stillLocked, err := first.IsLocked(ctx, "listing:scheduler:lock:SHEIN:inventory:962")
	if err != nil {
		t.Fatalf("IsLocked() error = %v", err)
	}
	if !stillLocked {
		t.Fatal("expected lock to remain held after non-owner unlock")
	}

	if err := first.Unlock(ctx, "listing:scheduler:lock:SHEIN:inventory:962"); err != nil {
		t.Fatalf("owner Unlock() error = %v", err)
	}
	stillLocked, err = first.IsLocked(ctx, "listing:scheduler:lock:SHEIN:inventory:962")
	if err != nil {
		t.Fatalf("IsLocked() after owner unlock error = %v", err)
	}
	if stillLocked {
		t.Fatal("expected owner unlock to remove lock")
	}
}

func TestNewRedisLockValidatesConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{name: "nil", cfg: nil, want: "redis config is nil"},
		{name: "empty host", cfg: &Config{}, want: "redis host is required"},
		{name: "blank host", cfg: &Config{Host: "  "}, want: "redis host is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lock, err := NewRedisLock(tt.cfg, "pod", nil)
			if lock != nil || err == nil || err.Error() != tt.want {
				t.Fatalf("NewRedisLock() = %#v, %v", lock, err)
			}
		})
	}
}

func TestNewRedisLockReportsConnectionFailure(t *testing.T) {
	server := miniredis.RunT(t)
	cfg := redisConfigForServer(t, server, "", 0, 1)
	server.Close()

	lock, err := NewRedisLock(cfg, "pod", nil)
	if lock != nil || err == nil {
		t.Fatalf("NewRedisLock() = %#v, %v", lock, err)
	}
	prefix := fmt.Sprintf("redis lock connection failed (%s:%d): ", cfg.Host, cfg.Port)
	if !strings.HasPrefix(err.Error(), prefix) {
		t.Fatalf("error = %q, want prefix %q", err, prefix)
	}
}

func TestRedisLockNilLoggerSupportsFullLifecycle(t *testing.T) {
	server := miniredis.RunT(t)
	lock, err := NewRedisLock(redisConfigForServer(t, server, "", 0, 2), "pod", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lock.Close() })
	ctx := context.Background()

	acquired, err := lock.TryLock(ctx, "job", time.Minute)
	if err != nil || !acquired {
		t.Fatalf("TryLock() = %v, %v", acquired, err)
	}
	locked, err := lock.IsLocked(ctx, "job")
	if err != nil || !locked {
		t.Fatalf("IsLocked() = %v, %v", locked, err)
	}
	extended, err := lock.Extend(ctx, "job", 2*time.Minute)
	if err != nil || !extended {
		t.Fatalf("Extend() = %v, %v", extended, err)
	}
	if err := lock.Unlock(ctx, "job"); err != nil {
		t.Fatal(err)
	}
	locked, err = lock.IsLocked(ctx, "job")
	if err != nil || locked {
		t.Fatalf("IsLocked() after Unlock = %v, %v", locked, err)
	}
}

func TestRedisLockRejectsNonPositiveTTL(t *testing.T) {
	lock := NewRedisLockWithClient(goredis.NewClient(&goredis.Options{Addr: miniredis.RunT(t).Addr()}), "pod", nil)
	t.Cleanup(func() { _ = lock.Close() })
	ctx := context.Background()
	for _, ttl := range []time.Duration{0, -time.Second} {
		acquired, err := lock.TryLock(ctx, "job", ttl)
		if acquired || err == nil || err.Error() != "lock ttl must be positive" {
			t.Fatalf("TryLock(%s) = %v, %v", ttl, acquired, err)
		}
		extended, err := lock.Extend(ctx, "job", ttl)
		if extended || err == nil || err.Error() != "lock ttl must be positive" {
			t.Fatalf("Extend(%s) = %v, %v", ttl, extended, err)
		}
	}
}

func TestRedisLockOnlyOwnerCanExtendAndOwnerTTLIsApplied(t *testing.T) {
	server := miniredis.RunT(t)
	raw := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = raw.Close() })
	owner := NewRedisLockWithClient(raw, "pod-1", nil)
	other := NewRedisLockWithClient(raw, "pod-2", nil)
	ctx := context.Background()

	acquired, err := owner.TryLock(ctx, "job", time.Minute)
	if err != nil || !acquired {
		t.Fatalf("TryLock() = %v, %v", acquired, err)
	}
	extended, err := other.Extend(ctx, "job", 5*time.Minute)
	if err != nil || extended {
		t.Fatalf("non-owner Extend() = %v, %v", extended, err)
	}
	if got := server.TTL("job"); got != time.Minute {
		t.Fatalf("TTL after non-owner Extend = %s, want 1m", got)
	}
	extended, err = owner.Extend(ctx, "job", 3*time.Minute)
	if err != nil || !extended {
		t.Fatalf("owner Extend() = %v, %v", extended, err)
	}
	if got := server.TTL("job"); got != 3*time.Minute {
		t.Fatalf("TTL after owner Extend = %s, want 3m", got)
	}
}

func TestRedisLockCloseClosesUnderlyingClient(t *testing.T) {
	server := miniredis.RunT(t)
	raw := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	lock := NewRedisLockWithClient(raw, "pod", nil)
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	if err := raw.Ping(context.Background()).Err(); err == nil {
		t.Fatal("underlying client accepted Ping after RedisLock.Close")
	}
}

func TestRedisLockNilAndUnconfiguredReceiversReturnErrors(t *testing.T) {
	var nilLock *RedisLock
	for name, lock := range map[string]*RedisLock{"nil": nilLock, "unconfigured": {}} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if acquired, err := lock.TryLock(ctx, "job", time.Minute); acquired || err == nil || err.Error() != "redis lock client is not configured" {
				t.Fatalf("TryLock() = %v, %v", acquired, err)
			}
			if err := lock.Unlock(ctx, "job"); err == nil || err.Error() != "redis lock client is not configured" {
				t.Fatalf("Unlock() error = %v", err)
			}
			if extended, err := lock.Extend(ctx, "job", time.Minute); extended || err == nil || err.Error() != "redis lock client is not configured" {
				t.Fatalf("Extend() = %v, %v", extended, err)
			}
			if locked, err := lock.IsLocked(ctx, "job"); locked || err == nil || err.Error() != "redis lock client is not configured" {
				t.Fatalf("IsLocked() = %v, %v", locked, err)
			}
			if err := lock.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})
	}
}

func TestDefaultLockOptions(t *testing.T) {
	got := DefaultLockOptions()
	if got == nil || got.RetryCount != 3 || got.RetryDelay != 100*time.Millisecond || got.AutoRenew || got.RenewInterval != 5*time.Second {
		t.Fatalf("DefaultLockOptions() = %#v", got)
	}
}
