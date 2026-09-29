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
	// slipTolerance is how far past its reserved slot a dispatch may run before the
	// waiters queued behind it are considered to have been scheduled against a
	// schedule that no longer holds. A clock tick of goroutine scheduling is not a
	// slip; half an interval plainly is.
	slipTolerance time.Duration
	// lastDispatchAt is when the last REAL acquisition dispatch actually ran. The
	// floor is a whole interval past it and would overshoot, so the instant is kept
	// separately to decide whether a waiter is still first in line.
	lastDispatchAt time.Time
	// dispatchFloor is the earliest start allowed by the last REAL acquisition
	// dispatch. Observe overwrites next with a synthetic cooldown floor, so the real
	// one has to be kept here or it is lost; on expiry the floor resumes at
	// max(now, dispatchFloor).
	dispatchFloor time.Time
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
		slipTolerance:     minInterval / 2,
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
		// Resume at the configured deadline. Observe overwrote next with a synthetic
		// cooldown+interval floor, so leaving it there refuses every request for a
		// further interval - well past the window the operator configured - because
		// the budget and headroom no longer fit. The real floor is kept in
		// dispatchFloor, so resumption is at max(now, dispatchFloor): immediate when
		// the cooldown outlasted the interval, still paced when a real dispatch is
		// still inside its own interval.
		t.next = now
		if t.dispatchFloor.After(t.next) {
			t.next = t.dispatchFloor
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
	// No acquisition may start before BOTH the queue tail and the floor left by the
	// last real dispatch. The two are not the same value: a dispatch that landed
	// inside an existing tail moved neither, and under scheduler inversion a later
	// reservation can reach its dispatch first and leave the tail behind while its
	// own floor is still ahead. Consulting only the tail there lets an earlier waiter
	// overwrite that floor and start beside an acquisition already running, which is
	// the burst this throttle exists to prevent.
	start := t.next
	if t.dispatchFloor.After(start) {
		start = t.dispatchFloor
	}
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
			// Roll back only while this reservation is still the newest. If another
			// waiter already re-anchored the floor, restoring this caller's stale
			// prevNext would let the next caller start beside that waiter.
			if t.generationLocked(seen) {
				if t.owner == mine {
					t.prevNext = time.Time{}
				}
			} else if t.owner == mine {
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
		// Re-anchor from THIS caller's own slot, not from the queue tail: a later
		// reservation must not stop this dispatch from recording the interval that
		// follows it, or the next waiter can start too soon after this one.
		actual := time.Now()
		dispatched := actual.Add(span)
		t.dispatchFloor = dispatched
		// See scheduleMoved: lateness is measured on actual, the tail on the floor.
		if t.scheduleMoved(dispatched, actual, start) {
			t.generation++
		}
		if dispatched.After(t.next) {
			t.next = dispatched
		}
		t.mu.Unlock()
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		// Same rule everywhere: roll back only while this reservation is still the
		// newest. Restoring a stale prevNext after another waiter re-anchored would
		// let the next caller start before that dispatch's interval elapsed.
		if !t.rollbackIfNewest(mine) {
			t.discard(mine)
		}
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
		// Same rule as the immediate path and the requeue path: roll back only
		// while this reservation is still the newest. If another waiter already
		// re-anchored the floor, restoring this caller's stale prevNext would let
		// the next caller start beside that waiter.
		if t.generationLocked(seen) {
			if t.owner == mine {
				t.prevNext = time.Time{}
			}
		} else if t.owner == mine {
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
	// Re-check the real dispatch floor at the moment of dispatch, not only when the
	// reservation was made. Under scheduler inversion a later waiter can dispatch
	// first and leave a floor ahead while this caller sleeps; with enough callers
	// queued the tail can also sit beyond that floor, so nothing else here would
	// notice and this caller would overwrite the newer floor and return within
	// MinInterval of an acquisition that is already running.
	if t.staleBehindNewerDispatch(start) {
		t.discardLocked(mine)
		t.mu.Unlock()
		return t.Wait(ctx)
	}
	// Advance the floor from when this caller ACTUALLY starts, not from the slot
	// it reserved. A goroutine whose timer fires but which is scheduled late would
	// otherwise let the next reservation be admitted on the ideal timeline, so two
	// real acquisition starts could be only milliseconds apart - the burst this
	// throttle exists to prevent, produced by the pacing itself.
	// Re-check the real dispatch floor at the moment of dispatch, not only when the
	// reservation was made. Under scheduler inversion a later waiter can dispatch
	// first and leave a floor ahead while this caller sleeps; with enough callers
	// queued the tail can also sit beyond that floor, so nothing else here would
	// notice and this caller would overwrite the newer floor and return within
	// MinInterval of an acquisition that is already running.
	if t.staleBehindNewerDispatch(start) {
		t.discardLocked(mine)
		t.mu.Unlock()
		return t.Wait(ctx)
	}
	// Re-anchor from THIS caller's own slot, not from the queue tail: a later
	// reservation must not stop this dispatch from recording the interval that
	// follows it, or the next waiter can start too soon after this one.
	actual := time.Now()
	dispatched := actual.Add(span)
	t.lastDispatchAt = actual
	t.dispatchFloor = dispatched
	// See the immediate path: lateness on actual, the tail on the floor.
	if t.scheduleMoved(dispatched, actual, start) {
		t.generation++
	}
	if dispatched.After(t.next) {
		t.next = dispatched
	}
	t.mu.Unlock()
	return nil
}

// discardLocked drops a reservation without restoring any boundary. The caller must
// already hold the mutex.
func (t *Throttle) discardLocked(seq uint64) {
	if t.owner == seq {
		t.prevNext = time.Time{}
	}
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

// staleBehindNewerDispatch reports whether a real acquisition dispatched after this
// caller reserved the slot it is about to use. If one did, this caller is second in
// line no matter how valid its own timer looks: under scheduler inversion the
// generation, the tail and the admission-time floor can all fail to reveal it, and
// committing here would start an acquisition beside one already running.
func (t *Throttle) staleBehindNewerDispatch(start time.Time) bool {
	return t.lastDispatchAt.After(start)
}

// scheduleMoved reports whether a dispatch that actually ran at actual, having
// reserved a slot at start and leaving a floor of dispatched, invalidates the
// waiters queued behind it.
//
// Both interleaveings are real and they pull in opposite directions. Comparing only
// against the queue tail misses a genuine slip whenever a later reservation has
// already pushed that tail out, letting the next waiter start right behind a
// predecessor that ran late. Comparing the floor against the slot with no tolerance
// counts a single clock tick of goroutine scheduling as a slip, because the floor
// is a full interval ahead of the dispatch by construction and would clear any
// tolerance on every single dispatch. So the lateness has to be measured on the
// instant the dispatch ran, and the tail on the floor it left.
func (t *Throttle) scheduleMoved(dispatched, actual, start time.Time) bool {
	return dispatched.After(t.next) || actual.After(start.Add(t.slipTolerance))
}

// rollbackIfNewest restores the displaced boundary only when this reservation is
// still the newest. It reports whether it rolled back.
func (t *Throttle) rollbackIfNewest(seq uint64) bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.owner != seq {
		return false
	}
	t.next = t.prevNext
	t.prevNext = time.Time{}
	return true
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
	// Every observed challenge gets the full configured window from the moment it was
	// seen, so overlapping acquisitions that each meet a challenge cannot resume
	// sooner than ChallengeCooldown after the latest one.
	//
	// The duplicate notification a single acquisition produces - once at detection,
	// again when the bounded solve gives up - is suppressed by the caller, which
	// knows both notifications belong to the same acquisition. Suppressing it here
	// instead would be process-wide and would drop the second of two genuinely
	// distinct challenges.
	t.blocked = true
	t.cooledAt = time.Now().Add(t.ChallengeCooldown)
	// Push the interval floor past the cooldown so work resumes paced.
	t.next = t.cooledAt.Add(t.MinInterval)
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
