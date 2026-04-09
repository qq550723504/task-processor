package amazon

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"task-processor/internal/core/config"
)

type concurrencyControl struct {
	enabled        bool
	acquireTimeout time.Duration
	maxWaiting     int64
	waiting        atomic.Int64
	global         chan struct{}
	perRegion      map[string]chan struct{}
}

type concurrencyTicket struct {
	global chan struct{}
	region chan struct{}
	cc     *concurrencyControl
}

func newConcurrencyControl(cfg config.AmazonConcurrencyControlConfig) *concurrencyControl {
	if !cfg.Enabled || cfg.MaxInFlight <= 0 {
		return nil
	}

	cc := &concurrencyControl{
		enabled:        true,
		acquireTimeout: time.Duration(cfg.AcquireTimeoutSeconds) * time.Second,
		maxWaiting:     int64(cfg.MaxWaiting),
		global:         make(chan struct{}, cfg.MaxInFlight),
		perRegion:      make(map[string]chan struct{}, len(cfg.PerRegion)),
	}
	for region, limit := range cfg.PerRegion {
		normalized := normalizeConcurrencyRegion(region)
		if normalized == "" || limit <= 0 {
			continue
		}
		cc.perRegion[normalized] = make(chan struct{}, limit)
	}
	if cc.acquireTimeout <= 0 {
		cc.acquireTimeout = 20 * time.Second
	}
	return cc
}

func (cc *concurrencyControl) Acquire(ctx context.Context, region string) (*concurrencyTicket, error) {
	if cc == nil || !cc.enabled {
		return &concurrencyTicket{}, nil
	}

	if cc.maxWaiting >= 0 {
		waitingNow := cc.waiting.Add(1)
		defer cc.waiting.Add(-1)
		if cc.maxWaiting > 0 && waitingNow > cc.maxWaiting {
			return nil, newSystemBusyError("crawler concurrency limit exceeded: waiting queue is full", nil)
		}
	}

	acquireCtx, cancel := context.WithTimeout(ctx, cc.acquireTimeout)
	defer cancel()

	ticket := &concurrencyTicket{cc: cc}
	if err := cc.acquireSlot(acquireCtx, cc.global); err != nil {
		return nil, newSystemBusyError(fmt.Sprintf("crawler concurrency acquire timeout: %v", err), err)
	}
	ticket.global = cc.global

	if regionCh := cc.perRegion[normalizeConcurrencyRegion(region)]; regionCh != nil {
		if err := cc.acquireSlot(acquireCtx, regionCh); err != nil {
			ticket.Release()
			return nil, newSystemBusyError(fmt.Sprintf("crawler concurrency acquire timeout: %v", err), err)
		}
		ticket.region = regionCh
	}

	return ticket, nil
}

func (cc *concurrencyControl) acquireSlot(ctx context.Context, semaphore chan struct{}) error {
	select {
	case semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (cc *concurrencyControl) Snapshot() map[string]any {
	if cc == nil || !cc.enabled {
		return nil
	}

	perRegionLimit := make(map[string]int64, len(cc.perRegion))
	perRegionInFlight := make(map[string]int64, len(cc.perRegion))
	for region, semaphore := range cc.perRegion {
		perRegionLimit[region] = int64(cap(semaphore))
		perRegionInFlight[region] = int64(len(semaphore))
	}

	return map[string]any{
		"concurrency_waiting_total":             cc.waiting.Load(),
		"concurrency_global_inflight":           int64(len(cc.global)),
		"concurrency_global_limit":              int64(cap(cc.global)),
		"concurrency_region_limit_by_region":    perRegionLimit,
		"concurrency_region_inflight_by_region": perRegionInFlight,
	}
}

func (t *concurrencyTicket) Release() {
	if t == nil {
		return
	}
	if t.region != nil {
		select {
		case <-t.region:
		default:
		}
		t.region = nil
	}
	if t.global != nil {
		select {
		case <-t.global:
		default:
		}
		t.global = nil
	}
}

func normalizeConcurrencyRegion(region string) string {
	return strings.TrimSpace(strings.ToLower(region))
}
