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
	// CollectionHeadroom is the time reserved, after the wait, for the collection
	// itself. A caller is only admitted if it can start early enough to still have
	// this much budget left; otherwise the wait would consume the whole acquisition
	// budget and the caller would see a timeout instead of an honest refusal.
	// Zero uses DefaultCollectionHeadroom.
	CollectionHeadroom time.Duration
	// StartupQuarantine is how long a freshly constructed throttle refuses its
	// first request. A restarted process has no memory of whether its egress IP
	// was challenged just before it died, so without this it would immediately
	// re-hit 1688 and negate the cooldown. Zero uses DefaultChallengeCooldown.
	StartupQuarantine time.Duration

	mu       sync.Mutex
	next     time.Time // earliest allowed start
	cooledAt time.Time
	blocked  bool
	// headroom is how much budget the caller must keep after waiting.
	headroom time.Duration
	// rand is guarded by mu.
	rand *rand.Rand
}

// Conservative defaults.
//
// MinInterval is set well above the observed burst that triggered a wall, and
// ChallengeCooldown is long enough for the observed recovery window. Both are
// starting points to be measured, not tuned truths.
const (
	DefaultMinInterval    = 20 * time.Second
	DefaultJitterFraction = 0.3
	// A replacement collector must assume the worst about an exit IP it never
	// observed, so the startup quarantine follows the configured cooldown.
	DefaultChallengeCooldown = 10 * time.Minute
	// CollectionHeadroomNumerator/Denominator express the headroom as a fraction
	// of the CONFIGURED budget, not of a packaged constant: an operator running a
	// 5s budget must not get a 7.5s headroom, which would refuse every request.
	// The margin is what lets an idle collector serve while still refusing a wait
	// that would spend the whole budget.
	CollectionHeadroomNumerator   = 3
	CollectionHeadroomDenominator = 4
)

func newThrottle(minInterval time.Duration, jitter float64, challengeCooldown, startupQuarantine time.Duration, budget time.Duration) *Throttle {
	if minInterval <= 0 {
		minInterval = DefaultMinInterval
	}
	if jitter <= 0 || jitter > 1 {
		jitter = DefaultJitterFraction
	}
	if challengeCooldown <= 0 {
		challengeCooldown = DefaultChallengeCooldown
	}
	// Zero (the unset default) means "follow the configured cooldown"; a negative
	// value explicitly disables the quarantine, which only tests need. Disabling
	// it by default would be the unsafe direction.
	//
	// It follows the CONFIGURED cooldown, not the packaged constant: an operator
	// who deliberately shortens -challenge-cooldown must not still get a
	// ten-minute silence on every restart, or the setting is not actually
	// tunable.
	quarantined := startupQuarantine >= 0
	if startupQuarantine == 0 {
		startupQuarantine = challengeCooldown
	}
	if budget <= 0 {
		budget = DefaultTimeout
	}
	th := &Throttle{
		headroom:          budget * CollectionHeadroomNumerator / CollectionHeadroomDenominator,
		MinInterval:       minInterval,
		Jitter:            jitter,
		ChallengeCooldown: challengeCooldown,
		StartupQuarantine: startupQuarantine,
		rand:              rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	if quarantined && startupQuarantine > 0 {
		// A new collector assumes the worst about an exit IP it never observed.
		th.blocked = true
		th.cooledAt = time.Now().Add(startupQuarantine)
	}
	return th
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
	// Reject an already-canceled caller before it can consume a slot, otherwise
	// an idle throttle returns success for a request nobody is waiting for.
	if err := ctx.Err(); err != nil {
		return err
	}
	t.mu.Lock()
	now := time.Now()
	// A cooldown that has elapsed is no longer a block. Resuming is paced rather
	// than immediate: the floor is set one interval out, so the process does not
	// immediately resume at full rate right after a challenge.
	if t.blocked && !t.cooledAt.IsZero() && !now.Before(t.cooledAt) {
		t.blocked = false
		t.cooledAt = time.Time{}
		if t.next.Before(now.Add(t.MinInterval)) {
			t.next = now.Add(t.MinInterval)
		}
	}
	if t.blocked {
		t.mu.Unlock()
		return ErrThrottled
	}

	span := t.MinInterval
	if extra := float64(t.MinInterval) * t.Jitter; extra > 0 {
		span += time.Duration(t.rand.Float64() * extra)
	}
	start := t.next
	if start.Before(now) {
		start = now
	}
	wait := start.Sub(now)
	// Admit only if the caller can start early enough to keep collection
	// headroom left. Checking the start alone let a caller that merely squeaks in
	// just before its deadline spend the entire remaining budget on the wait, and
	// then hand that exhausted budget to the collection - surfacing as a timeout
	// rather than the refusal this throttle intends.
	if deadline, ok := ctx.Deadline(); ok && start.Add(t.headroom).After(deadline) {
		t.mu.Unlock()
		return ErrThrottled
	}
	// Commit the slot. Only the floor is kept, deliberately: an earlier version
	// tracked every reservation in a list so that a cancelled caller could give
	// its slot back, and that bookkeeping produced repeated defects - unbounded
	// growth, walls deleting slots from delayed waiters, and compaction that never
	// actually closed a gap. A cancelled caller simply leaves its interval
	// consumed, which is the safe direction: it can delay one request, never
	// allow a burst.
	t.next = start.Add(span)
	t.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	// The wait may have spanned another acquisition, and that one may have
	// observed a challenge. Returning success here would hand the caller a slot
	// the process has already decided to refuse, so the block is re-checked once
	// the wait is over.
	if t.nowBlocked() {
		return ErrThrottled
	}
	return nil
}

// nowBlocked reports whether a challenge has put the process into cooldown,
// after lazily expiring a window that has already elapsed.
func (t *Throttle) nowBlocked() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.blocked && !t.cooledAt.IsZero() && !time.Now().Before(t.cooledAt) {
		t.blocked = false
		t.cooledAt = time.Time{}
	}
	return t.blocked
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
