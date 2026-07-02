package lock

import (
	"context"
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
