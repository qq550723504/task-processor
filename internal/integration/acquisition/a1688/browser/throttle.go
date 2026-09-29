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
//
// # Why the state is this small
//
// The invariant is one sentence: two acquisitions must not START less than
// MinInterval apart, and none may start while a challenge cooldown is in force.
// That is two instants, so the throttle holds two instants and nothing else -
// floor, the earliest start the last real dispatch permits, and cooldownUntil.
//
// An earlier version of this file also kept a queue tail, a generation counter, a
// reservation owner, the boundary a reservation had displaced, the real dispatch
// floor and the instant of the last dispatch, and it re-derived the answer at four
// separate points in Wait. Every one of those encoded the same fact, so each
// invariant was enforced in several places that could disagree, and a decision made
// from a subset of them was wrong in exactly the case that subset did not cover.
//
// Nothing is reserved here. A caller waits on the advertised floor and then
// re-derives the decision from the floor under the mutex, in the same critical
// section that commits its own start. A stale timer, a scheduler inversion or a
// cancelled caller can therefore delay work but can never admit it early, because
// admission is never taken from a value observed before the wait.
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

	mu sync.Mutex
	// floor is the earliest instant an acquisition may start, established by the
	// last real dispatch. It only ever moves forward, and only when a caller is
	// actually admitted.
	floor time.Time
	// cooldownUntil is when a challenge cooldown ends, or the zero time.
	cooldownUntil time.Time
	// headroom is how much budget the caller must keep after waiting.
	headroom time.Duration
	// rand is guarded by mu.
	rand *rand.Rand
}

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
		th.cooldownUntil = time.Now().Add(startupQuarantine)
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
// Wait may loop. Each pass re-reads the floor under the mutex and re-derives the
// decision, so a caller that lost a race simply waits again rather than proceeding
// on the value it saw before its wait. It cannot spin: the next pass always has a
// strictly later floor to wait on unless it is admitted or refused.
func (t *Throttle) Wait(ctx context.Context) error {
	if t == nil {
		return nil
	}
	for {
		wait, err := t.admit(ctx)
		if err != nil {
			return err
		}
		if wait <= 0 {
			return nil
		}
		if err := t.sleep(ctx, wait); err != nil {
			return err
		}
	}
}

// admit derives the caller's fate from the current floor and, when that caller may
// start, records the start in the same critical section.
//
// It returns the duration to wait before asking again, or nil error with a
// non-positive wait once the caller has been admitted. Admitting and committing
// together is what makes the interval hold: there is no window in which a caller
// has been told "you may start" but has not yet moved the floor, which is where the
// earlier version of this file let two acquisitions start side by side.
func (t *Throttle) admit(ctx context.Context) (time.Duration, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	// A cooldown is a refusal, not a wait: the configured window is already longer
	// than any caller's budget, so waiting would only convert it into a timeout.
	if now.Before(t.cooldownUntil) {
		return 0, ErrThrottled
	}

	earliest := t.floor
	if earliest.Before(now) {
		earliest = now
	}
	// Admit only if the caller can start early enough to keep the collection
	// headroom. Checking this against the earliest possible start, rather than
	// after the wait, is what turns "this would have timed out" into an honest
	// refusal instead of a request that spends its whole budget queueing.
	if deadline, ok := ctx.Deadline(); ok && earliest.Add(t.headroom).After(deadline) {
		return 0, ErrThrottled
	}

	if earliest.After(now) {
		return earliest.Sub(now), nil
	}

	// The commit. The floor is measured from the instant this acquisition really
	// starts, not from any slot it may have been aiming at, so the next caller is
	// paced from reality.
	t.floor = now.Add(t.spanLocked())
	return 0, nil
}

// spanLocked is the configured interval plus this process's jitter. The caller must
// hold mu, because it draws from the shared generator.
func (t *Throttle) spanLocked() time.Duration {
	span := t.MinInterval
	if extra := float64(t.MinInterval) * t.Jitter; extra > 0 {
		span += time.Duration(t.rand.Float64() * extra)
	}
	return span
}

// sleep waits for d, or returns the context error if the caller gives up first.
//
// A cancelled caller changes nothing: the floor is only moved by an admitted
// caller, so a request that never started has consumed no interval and there is
// nothing to give back. That also removes any way for repeated cancelled or
// refused callers to push the queue forward and starve later requests.
func (t *Throttle) sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
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
	//
	// Note what this does NOT do: it does not push the pacing floor past the end of
	// the window. The floor belongs to real dispatches only. An earlier version
	// overwrote it with the cooldown end plus an interval, which made the operator's
	// configured window a lower bound on the real one rather than the window itself.
	t.cooldownUntil = time.Now().Add(t.ChallengeCooldown)
}

// Reset clears a cooldown, for an operator-confirmed recovery. It is not called
// on the request path: recovery is a time-based decision, not a per-call guess.
//
// It does not clear the pacing floor, which records when acquisitions actually
// started and is not a symptom of anything an operator can confirm.
func (t *Throttle) Reset() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cooldownUntil = time.Time{}
}

// CooldownRemaining reports how long the current refusal lasts, for
// observability and for the collector to surface in logs.
func (t *Throttle) CooldownRemaining() time.Duration {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if d := time.Until(t.cooldownUntil); d > 0 {
		return d
	}
	return 0
}

// errorsIs is a tiny indirection so the throttle does not need to import errors
// for a single call site and stays trivially testable.
func errorsIs(err, target error) bool { return errors.Is(err, target) }
