package listingcontrol

import (
	"context"
	"testing"
	"time"
)

func TestRedisLeaderLockAcquiresAndReportsOwner(t *testing.T) {
	runtime := &fakeLeaderRuntime{acquired: true}
	lock := newRedisLeaderLock(runtime, "listing:control-plane:leader:shein", "node-a", 30*time.Second)

	snapshot, acquired, err := lock.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire returned error: %v", err)
	}
	if !acquired {
		t.Fatal("Acquire acquired = false, want true")
	}
	if snapshot.Key != "listing:control-plane:leader:shein" ||
		snapshot.Owner != "node-a" ||
		!snapshot.IsLeader ||
		snapshot.TTL != "30s" ||
		snapshot.AcquiredAt == nil ||
		snapshot.RenewedAt == nil {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if runtime.key != "listing:control-plane:leader:shein" || runtime.owner != "node-a" || runtime.ttl != 30*time.Second {
		t.Fatalf("runtime call = key:%q owner:%q ttl:%v", runtime.key, runtime.owner, runtime.ttl)
	}
}

func TestRedisLeaderLockReportsStandbyOwner(t *testing.T) {
	runtime := &fakeLeaderRuntime{currentOwner: "node-b"}
	lock := newRedisLeaderLock(runtime, "listing:control-plane:leader:shein", "node-a", 30*time.Second)

	snapshot, acquired, err := lock.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire returned error: %v", err)
	}
	if acquired {
		t.Fatal("Acquire acquired = true, want false")
	}
	if snapshot.Owner != "node-b" || snapshot.IsLeader {
		t.Fatalf("unexpected standby snapshot: %+v", snapshot)
	}
}

type fakeLeaderRuntime struct {
	key          string
	owner        string
	ttl          time.Duration
	acquired     bool
	currentOwner string
	err          error
}

func (f *fakeLeaderRuntime) AcquireLeaderLock(ctx context.Context, key, owner string, ttl time.Duration) (string, bool, error) {
	f.key = key
	f.owner = owner
	f.ttl = ttl
	if f.err != nil {
		return "", false, f.err
	}
	if f.acquired {
		return owner, true, nil
	}
	return f.currentOwner, false, nil
}
