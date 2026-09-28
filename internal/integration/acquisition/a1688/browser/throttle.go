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
	// owner/prevNext describe the newest committed reservation, so a caller that
	// is cancelled can hand its slot back when no later caller has committed
	// since. Only the newest is tracked: if it has been superseded this process
	// cannot tell which boundary was displaced, and leaving the floor is the
	// conservative choice - it can delay a request, never let two start together.
	owner    uint64
	prevNext time.Time
	// generation is bumped whenever the floor is re-anchored on a late dispatch.
	// Waiters capture it before sleeping and re-loop when it changed, so a
	// caller already in the queue is not admitted on its original timer after an
	// earlier waiter slipped.
	generation uint64
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

func newThrottle(minInterval time.Duration, jitter float64, challengeCooldown, startupQuarantine time.Duration, budget, headroom time.Duration) *Throttle {
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
	// A caller may configure the headroom explicitly; otherwise it is derived
	// from the CONFIGURED budget. Ignoring the option would make it a dead
	// setting that only appears in tests.
	if headroom <= 0 {
		headroom = budget * CollectionHeadroomNumerator / CollectionHeadroomDenominator
	}
	th := &Throttle{
		headroom:          headroom,
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
		// Resume at the configured deadline. Observe had pushed the floor to
		// cooldown+interval; leaving it there would refuse every request for a
		// further interval - well past the window the operator configured - because
		// the budget and headroom no longer fit. The cooldown itself already served
		// as the wait, so the floor returns to now and the next caller is paced
		// normally from there.
		t.next = now
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
	// Commit the slot, remembering the boundary it displaced.
	//
	// Only the newest reservation is tracked. An earlier version kept a list of
	// every reservation, which produced repeated defects - unbounded growth,
	// wall-clock retirement deleting slots from delayed waiters, and compaction
	// that never actually closed a gap. A single value is enough: it lets the
	// common cancellation hand its slot back, and when a later caller has already
	// committed there is nothing safe to do, so the floor stands.
	t.owner++
	mine := t.owner
	t.prevNext = t.next
	t.next = start.Add(span)
	seen := t.generation
	t.mu.Unlock()

	if wait <= 0 {
		// The immediate path gets exactly the same discipline as the timer path: a
		// generation re-check, a block check and a cancellation check, all under one
		// lock. It is otherwise a route that can dispatch beside a waiter that
		// re-anchored the floor while queued reservations expired.
		t.mu.Lock()
		if t.generationLocked(seen) {
			if t.owner == mine {
				t.prevNext = time.Time{}
			}
			t.mu.Unlock()
			return t.Wait(ctx)
		}
		if err := ctx.Err(); err != nil {
			if t.owner == mine {
				t.next = t.prevNext
				t.prevNext = time.Time{}
			}
			t.mu.Unlock()
			return err
		}
		if t.blockedLocked() {
			if t.owner == mine {
				t.next = t.prevNext
				t.prevNext = time.Time{}
			}
			t.mu.Unlock()
			return ErrThrottled
		}
		if dispatched := time.Now().Add(span); dispatched.After(t.next) {
			t.next = dispatched
			t.generation++
		}
		t.mu.Unlock()
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		t.release(mine)
		return ctx.Err()
	case <-timer.C:
	}
	// The wait may have spanned another acquisition, and that one may have
	// observed a challenge. Returning success here would hand the caller a slot
	// the process has already decided to refuse, so the block is re-checked once
	// the wait is over.
	// The generation check, the block check and the dispatch commitment must all be
	// ONE critical section. If the generation is validated separately, two waiters
	// whose timers both elapsed can both see the old value and then dispatch
	// together; if Observe can interleave, a caller starts during a cooldown.
	t.mu.Lock()
	// The timer already fired, but the caller may have been cancelled while this
	// waiter waited for the lock. Committing a dispatch for a dead request would
	// hand out a slot and let Acquire start a browser for it.
	if err := ctx.Err(); err != nil {
		if t.owner == mine {
			t.next = t.prevNext
			t.prevNext = time.Time{}
		}
		t.mu.Unlock()
		return err
	}
	if t.generationLocked(seen) {
		// Someone else re-anchored the floor while this caller slept. Drop this
		// reservation without restoring anything - the newer floor stands - and
		// re-evaluate against it.
		if t.owner == mine {
			t.prevNext = time.Time{}
		}
		t.mu.Unlock()
		return t.Wait(ctx)
	}
	if t.blockedLocked() {
		if t.owner == mine {
			t.next = t.prevNext
			t.prevNext = time.Time{}
		}
		t.mu.Unlock()
		return ErrThrottled
	}
	// Advance the floor from when this caller ACTUALLY starts, not from the slot
	// it reserved. A goroutine whose timer fires but which is scheduled late would
	// otherwise let the next reservation be admitted on the ideal timeline, so two
	// real acquisition starts could be only milliseconds apart - the burst this
	// throttle exists to prevent, produced by the pacing itself.
	if dispatched := time.Now().Add(span); dispatched.After(t.next) {
		t.next = dispatched
		t.generation++
	}
	t.mu.Unlock()
	return nil
}

// discard drops a reservation without restoring any boundary.
func (t *Throttle) discard(seq uint64) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.owner == seq {
		t.prevNext = time.Time{}
	}
}

// blockedLocked reports the block state, expiring an elapsed window. The caller
// must hold the lock.
func (t *Throttle) blockedLocked() bool {
	if t.blocked && !t.cooledAt.IsZero() && !time.Now().Before(t.cooledAt) {
		t.blocked = false
		t.cooledAt = time.Time{}
		if t.next.Before(time.Now().Add(t.MinInterval)) {
			t.next = time.Now().Add(t.MinInterval)
		}
	}
	return t.blocked
}

// generationLocked reports whether the floor was re-anchored since it was read.
// The caller must hold the lock.
func (t *Throttle) generationLocked(seen uint64) bool {
	return t.generation != seen
}

// release hands a committed slot back when no later caller has committed since.
// When one has, the floor stands: this process cannot tell which boundary the
// cancelled reservation displaced, and delaying one request is safer than
// letting two start together.
func (t *Throttle) release(seq uint64) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.owner == seq {
		t.next = t.prevNext
		t.prevNext = time.Time{}
	}
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
