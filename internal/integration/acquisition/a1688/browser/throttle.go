package browser

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"
)

// Throttle shapes the anonymous collection rate of one collector process.
//
// It exists because the observed 1688 behaviour is frequency-triggered: a
// short burst of anonymous requests escalates the shared egress IP to a
// challenge wall, and the IP recovers only after a cooldown. Rotating egress
// multiplies the budget but does not change that root cause, so the rate must be
// governed here regardless of how many exit IPs are configured.
//
// It is deliberately self-imposed and conservative by default. Every duration is
// configurable so a deployment can measure its own budget rather than inherit a
// guess.
type Throttle struct {
	// MinInterval is the floor between the START of two acquisitions. Zero uses
	// DefaultMinInterval.
	MinInterval time.Duration
	// Jitter spreads the effective interval randomly up to this fraction so many
	// collector processes do not fall into a synchronised pattern. Zero uses
	// DefaultJitterFraction.
	Jitter float64
	// ChallengeCooldown is how long to refuse new work after a challenge. Zero
	// uses DefaultChallengeCooldown.
	ChallengeCooldown time.Duration

	mu       sync.Mutex
	next     time.Time // earliest allowed start
	cooledAt time.Time
	blocked  bool
	// rand is guarded by mu.
	rand *rand.Rand
}

// Conservative defaults.
//
// MinInterval is set well above the observed burst that triggered a wall, and
// ChallengeCooldown is long enough for the observed recovery window. Both are
// starting points to be measured, not tuned truths.
const (
	DefaultMinInterval       = 20 * time.Second
	DefaultJitterFraction    = 0.3
	DefaultChallengeCooldown = 10 * time.Minute
)

func newThrottle(minInterval time.Duration, jitter float64, challengeCooldown time.Duration) *Throttle {
	if minInterval <= 0 {
		minInterval = DefaultMinInterval
	}
	if jitter <= 0 || jitter > 1 {
		jitter = DefaultJitterFraction
	}
	if challengeCooldown <= 0 {
		challengeCooldown = DefaultChallengeCooldown
	}
	return &Throttle{
		MinInterval:       minInterval,
		Jitter:            jitter,
		ChallengeCooldown: challengeCooldown,
		rand:              rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Wait blocks until this acquisition may start, the context expires, or the
// throttle reports that it is refusing work entirely because a challenge put the
// process into cooldown.
//
// A cooldown is returned as a typed refusal rather than waited out: holding an
// HTTP handler for ten minutes is worse than telling the caller to retry, and the
// caller's budget is far shorter than the cooldown anyway.
func (t *Throttle) Wait(ctx context.Context) error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	if t.blocked {
		t.mu.Unlock()
		return ErrThrottled
	}
	now := time.Now()
	if !t.cooledAt.IsZero() && now.Before(t.cooledAt) {
		// A cooldown is still running; the next allowed start is the later of the
		// cooldown expiry and the interval floor.
		start := t.cooledAt
		if t.next.After(start) {
			start = t.next
		}
		t.next = start
		t.cooledAt = time.Time{}
	}
	wait := time.Duration(0)
	if now.Before(t.next) {
		wait = t.next.Sub(now)
	}
	span := float64(t.MinInterval) * t.Jitter
	effective := t.MinInterval
	if span > 0 {
		effective += time.Duration(t.rand.Float64() * span)
	}
	// Reserve this slot so concurrent callers queue behind each other instead of
	// all passing the gate together. The reservation extends the slot that was
	// already taken, not the current instant: using `now` here would let every
	// waiter in the same instant claim the same next slot and defeat the floor.
	start := t.next
	if start.Before(now) {
		start = now
	}
	t.next = start.Add(effective)
	t.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	// If the wait cannot fit inside the caller's remaining budget, refuse now
	// instead of blocking until the budget expires. Letting it expire would
	// attribute a self-imposed pace to the source: the caller would see a
	// deadline, as though 1688 had been slow, when in fact this collector
	// declined to call out yet.
	if deadline, ok := ctx.Deadline(); ok && time.Now().Add(wait).After(deadline) {
		return ErrThrottled
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Observe reports the outcome of an acquisition so the throttle can react.
//
// A challenge is the only signal treated as a reason to stop the world: it is
// the observed escalation, and continuing immediately is what deepens it.
func (t *Throttle) Observe(err error) {
	if t == nil || err == nil {
		return
	}
	if !errorsIs(err, ErrChallenge) && !errorsIs(err, ErrRejected) {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.blocked = true
	t.cooledAt = time.Now().Add(t.ChallengeCooldown)
	// Push the interval floor past the cooldown so work resumes paced.
	if after := t.cooledAt.Add(t.MinInterval); after.After(t.next) {
		t.next = after
	}
}

// Reset clears a cooldown, for an operator-confirmed recovery. It is not called
// on the request path: recovery is a time-based decision, not a per-call guess.
func (t *Throttle) Reset() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.blocked = false
	t.cooledAt = time.Time{}
}

// CooldownRemaining reports how long the current refusal lasts, for
// observability and for the collector to surface in logs.
func (t *Throttle) CooldownRemaining() time.Duration {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.blocked || t.cooledAt.IsZero() {
		return 0
	}
	if d := time.Until(t.cooledAt); d > 0 {
		return d
	}
	return 0
}

// errorsIs is a tiny indirection so the throttle does not need to import errors
// for a single call site and stays trivially testable.
func errorsIs(err, target error) bool { return errors.Is(err, target) }
