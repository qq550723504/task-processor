package scheduler

import (
	"context"
	"time"
)

// DistributedLock is the scheduler-local port for cross-process task exclusion.
type DistributedLock interface {
	TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Unlock(ctx context.Context, key string) error
	Extend(ctx context.Context, key string, ttl time.Duration) (bool, error)
	IsLocked(ctx context.Context, key string) (bool, error)
}
