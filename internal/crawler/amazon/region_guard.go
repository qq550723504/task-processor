package amazon

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"task-processor/internal/core/config"
)

type regionGuard struct {
	mu               sync.Mutex
	enabled          bool
	failureThreshold int
	window           time.Duration
	cooldown         time.Duration
	now              func() time.Time
	states           map[string]*regionGuardState
}

type regionGuardState struct {
	failures  []time.Time
	openUntil time.Time
}

func newRegionGuard(cfg config.AmazonRegionGuardConfig) *regionGuard {
	return &regionGuard{
		enabled:          cfg.Enabled,
		failureThreshold: cfg.FailureThreshold,
		window:           time.Duration(cfg.EvaluationWindowSeconds) * time.Second,
		cooldown:         time.Duration(cfg.CooldownSeconds) * time.Second,
		now:              time.Now,
		states:           make(map[string]*regionGuardState),
	}
}

func (g *regionGuard) Check(region string) (time.Time, bool) {
	if g == nil || !g.enabled {
		return time.Time{}, false
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()
	state := g.getStateLocked(region)
	if state.openUntil.After(now) {
		return state.openUntil, true
	}
	if !state.openUntil.IsZero() {
		state.openUntil = time.Time{}
	}
	return time.Time{}, false
}

func (g *regionGuard) RecordSuccess(region string) {
	if g == nil || !g.enabled {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	state := g.getStateLocked(region)
	state.failures = nil
	if !state.openUntil.IsZero() && !state.openUntil.After(g.now()) {
		state.openUntil = time.Time{}
	}
}

func (g *regionGuard) RecordFailure(region string, err error) (time.Time, bool) {
	if g == nil || !g.enabled || !shouldTripRegionGuard(err) {
		return time.Time{}, false
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()
	state := g.getStateLocked(region)
	g.trimFailuresLocked(state, now)
	state.failures = append(state.failures, now)
	if len(state.failures) < g.failureThreshold {
		return time.Time{}, false
	}

	state.openUntil = now.Add(g.cooldown)
	state.failures = nil
	return state.openUntil, true
}

func (g *regionGuard) Snapshot() map[string]int64 {
	if g == nil || !g.enabled {
		return map[string]int64{}
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()
	result := make(map[string]int64)
	for region, state := range g.states {
		if state.openUntil.After(now) {
			result[region] = 1
			continue
		}
		if !state.openUntil.IsZero() {
			state.openUntil = time.Time{}
		}
	}
	return result
}

func (g *regionGuard) getStateLocked(region string) *regionGuardState {
	normalized := normalizeMetricsRegion(strings.ToLower(strings.TrimSpace(region)))
	state, ok := g.states[normalized]
	if !ok {
		state = &regionGuardState{}
		g.states[normalized] = state
	}
	return state
}

func (g *regionGuard) trimFailuresLocked(state *regionGuardState, now time.Time) {
	if len(state.failures) == 0 || g.window <= 0 {
		return
	}
	cutoff := now.Add(-g.window)
	trimmed := state.failures[:0]
	for _, ts := range state.failures {
		if ts.After(cutoff) {
			trimmed = append(trimmed, ts)
		}
	}
	state.failures = trimmed
}

func shouldTripRegionGuard(err error) bool {
	classified := ClassifyFetchError(err)
	if classified == nil {
		return false
	}

	switch classified.ErrorType() {
	case FetchErrorTypeCaptcha,
		FetchErrorTypeAuthentication,
		FetchErrorTypeBrowserCrash,
		FetchErrorTypeTimeout,
		FetchErrorTypeNetwork,
		FetchErrorTypeServerError:
		return true
	default:
		return false
	}
}

func newRegionCircuitOpenError(region string, openUntil time.Time) *FetchError {
	return &FetchError{
		Type:      FetchErrorTypeRegionCircuitOpen,
		Retryable: true,
		Cause:     fmt.Errorf("region %s temporarily blocked until %s", region, openUntil.Format(time.RFC3339)),
	}
}
