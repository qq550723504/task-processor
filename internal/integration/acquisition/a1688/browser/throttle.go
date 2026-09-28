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
	seq      uint64    // identifies the newest reservation, for safe rollback
	prevNext time.Time // boundary the newest reservation displaced
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
//
// A reservation is only committed when the caller can actually use the slot. A
// request that is refused, or whose context is cancelled while waiting, gives its
// slot back, so repeated retryable callers cannot push the queue forward forever
// and starve every later request.
func (t *Throttle) Wait(ctx context.Context) error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	// A cooldown that has elapsed is no longer a block: clear it here rather than
	// waiting for an external reset, so recovery is a time-based decision and the
	// process resumes by itself.
	if t.blocked && !t.cooledAt.IsZero() && !time.Now().Before(t.cooledAt) {
		t.blocked = false
		t.cooledAt = time.Time{}
	}
	if t.blocked {
		t.mu.Unlock()
		return ErrThrottled
	}

	now := time.Now()
	span := float64(t.MinInterval) * t.Jitter
	effective := t.MinInterval
	if span > 0 {
		effective += time.Duration(t.rand.Float64() * span)
	}
	start := t.next
	if start.Before(now) {
		start = now
	}
	slotEnd := start.Add(effective)
	wait := time.Duration(0)
	if now.Before(start) {
		wait = start.Sub(now)
	}

	// If this caller cannot even START inside its own budget, do not consume a
	// slot: the reservation would be a phantom that only pushes the queue
	// further away, and refusing is what the caller can act on.
	//
	// The check is on the scheduled start, not on slotEnd. The interval floor
	// deliberately pushes the *next* caller a full interval out, so requiring
	// that boundary to fit this caller's budget would reject the very first
	// request of an idle process under the production defaults.
	deadline, hasDeadline := ctx.Deadline()
	if hasDeadline && start.After(deadline) {
		t.mu.Unlock()
		return ErrThrottled
	}

	t.prevNext = t.next
	t.seq++
	mine := t.seq
	t.next = slotEnd
	t.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		t.release(mine)
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// release gives back a reservation that the caller could not use. It only rolls
// back when no later reservation was made, so a cancelled waiter never truncates
// the queue of callers behind it.
func (t *Throttle) release(seq uint64) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.seq == seq {
		// Restore the boundary this reservation displaced. Clearing the schedule
		// outright would let the next caller start immediately after the preceding
		// acquisition and recreate the burst this throttle exists to prevent.
		t.next = t.prevNext
	}
}

// Observe reports the outcome of an acquisition so the throttle can react.
//
// A challenge is the only signal treated as a reason to stop the world: it is
// the observed escalation, and continuing immediately is what deepens it.
//
// ErrRejected is deliberately NOT a trigger. It means this provider refused
// navigation under its OWN egress policy - an oversized document, or a redirect
// to a disallowed origin. That is a local policy decision, not evidence of 1688
// risk control, so treating it as a challenge would let one configuration or
// content-policy mismatch park every unrelated acquisition behind a multi-minute
// cooldown, and would keep repeating for as long as the mismatch persists.
func (t *Throttle) Observe(err error) {
	if t == nil || err == nil {
		return
	}
	if !errorsIs(err, ErrChallenge) {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.blocked = true
	t.cooledAt = time.Now().Add(t.ChallengeCooldown)
	t.seq++
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
