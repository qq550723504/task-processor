package listingcontrol

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/core/config"
)

const defaultLeaderLockTTL = 30 * time.Second

type leaderLockRuntime interface {
	AcquireLeaderLock(ctx context.Context, key, owner string, ttl time.Duration) (string, bool, error)
}

type redisLeaderLock struct {
	runtime leaderLockRuntime
	key     string
	owner   string
	ttl     time.Duration
}

func newRedisLeaderLock(runtime leaderLockRuntime, key, owner string, ttl time.Duration) *redisLeaderLock {
	key = strings.TrimSpace(key)
	owner = strings.TrimSpace(owner)
	if ttl <= 0 {
		ttl = defaultLeaderLockTTL
	}
	return &redisLeaderLock{
		runtime: runtime,
		key:     key,
		owner:   owner,
		ttl:     ttl,
	}
}

func (l *redisLeaderLock) Acquire(ctx context.Context) (LeaderSnapshot, bool, error) {
	if l == nil || l.runtime == nil {
		return LeaderSnapshot{}, false, fmt.Errorf("leader lock runtime is not configured")
	}
	if l.key == "" {
		return LeaderSnapshot{}, false, fmt.Errorf("leader lock key is required")
	}
	if l.owner == "" {
		return LeaderSnapshot{}, false, fmt.Errorf("leader lock owner is required")
	}

	currentOwner, acquired, err := l.runtime.AcquireLeaderLock(ctx, l.key, l.owner, l.ttl)
	if err != nil {
		return LeaderSnapshot{}, false, err
	}
	if currentOwner == "" {
		currentOwner = l.owner
	}
	now := time.Now()
	snapshot := LeaderSnapshot{
		Key:      l.key,
		Owner:    currentOwner,
		IsLeader: acquired,
		TTL:      l.ttl.String(),
	}
	if acquired {
		snapshot.AcquiredAt = &now
		snapshot.RenewedAt = &now
	}
	return snapshot, acquired, nil
}

func resolveLeaderLockKey(controlCfg config.ListingControlPlaneConfig, platform string) string {
	key := strings.TrimSpace(controlCfg.LeaderLockKey)
	if key != "" {
		return key
	}
	platform = normalizePlatform(platform)
	return "listing:control-plane:leader:" + platform
}

func resolveLeaderOwner(cfg *config.Config) string {
	if cfg != nil && cfg.RabbitMQ != nil {
		if nodeID := strings.TrimSpace(cfg.RabbitMQ.Node.NodeID); nodeID != "" {
			return nodeID
		}
	}
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "unknown"
	}
	return fmt.Sprintf("%s-%d", strings.TrimSpace(host), os.Getpid())
}

func leaderLockTTL(controlCfg config.ListingControlPlaneConfig) time.Duration {
	if controlCfg.LeaderLockTTL > 0 {
		return controlCfg.LeaderLockTTL
	}
	return defaultLeaderLockTTL
}

func leaderRenewInterval(controlCfg config.ListingControlPlaneConfig) time.Duration {
	ttl := leaderLockTTL(controlCfg)
	interval := ttl / 3
	if interval <= 0 {
		return time.Second
	}
	if interval < time.Second {
		return time.Second
	}
	return interval
}

func leaderLockTTLMilliseconds(ttl time.Duration) string {
	if ttl <= 0 {
		ttl = defaultLeaderLockTTL
	}
	return strconv.FormatInt(ttl.Milliseconds(), 10)
}
