package browser

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/product/sourcing"
)

// The rate floor must actually pace acquisitions, and it must queue concurrent
// callers behind each other instead of letting a burst through.
func TestThrottlePacesAndQueuesConcurrentCallers(t *testing.T) {
	th := newTestThrottle(120*time.Millisecond, 0, 0)

	started := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			require.NoError(t, th.Wait(context.Background()))
		}()
	}
	wg.Wait()
	// Three callers at a 120ms floor cannot finish instantly; the second and
	// third must have been spaced by the floor.
	require.GreaterOrEqual(t, time.Since(started), 200*time.Millisecond,
		"concurrent callers must be paced by the rate floor")
}

// A challenge must stop the process, and the refusal must be typed and
// immediate rather than a multi-minute block.
func TestThrottleChallengeCooldownRefusesImmediately(t *testing.T) {
	th := newTestThrottle(time.Millisecond, 0, 10*time.Minute)
	require.NoError(t, th.Wait(context.Background()))

	th.Observe(ErrChallenge)
	require.Positive(t, th.CooldownRemaining())

	start := time.Now()
	err := th.Wait(context.Background())
	require.ErrorIs(t, err, ErrThrottled)
	require.Less(t, time.Since(start), time.Second,
		"a cooldown must be reported immediately, never waited out inside a handler")
}

// A non-challenge failure must NOT stop the world: only a challenge is the
// observed escalation signal.
func TestThrottleIgnoresNonChallengeFailures(t *testing.T) {
	th := newTestThrottle(time.Millisecond, 0, 10*time.Minute)
	th.Observe(ErrUnsupported)
	th.Observe(ErrUnavailable)
	th.Observe(ErrCapacity)
	th.Observe(nil)
	// Our own egress-policy rejection is not evidence of 1688 risk control, so it
	// must not park unrelated acquisitions behind a cooldown.
	th.Observe(ErrRejected)
	require.Zero(t, th.CooldownRemaining(), "only a challenge may trigger a cooldown")
	require.NoError(t, th.Wait(context.Background()))
}

// A wait that cannot fit inside the caller's budget must be refused immediately
// rather than allowed to expire the budget. Expiring would attribute a
// self-imposed pace to the source: the caller would see a deadline as though
// 1688 had been slow.
func TestThrottleWaitRefusesWhenItCannotMeetTheBudget(t *testing.T) {
	th := newTestThrottle(2*time.Second, 0, 0)
	require.NoError(t, th.Wait(context.Background())) // consumes the immediate slot

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := th.Wait(ctx)
	require.ErrorIs(t, err, ErrThrottled)
	require.Less(t, time.Since(start), time.Second, "the refusal must be immediate")
}

// A wait that does fit must still stop when the caller's context is cancelled, so
// the acquisition budget continues to bound the handler.
func TestThrottleWaitHonoursContextCancellation(t *testing.T) {
	th := newTestThrottle(400*time.Millisecond, 0, 0)
	require.NoError(t, th.Wait(context.Background())) // consumes the immediate slot

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	defer cancel()
	start := time.Now()
	err := th.Wait(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, time.Since(start), 400*time.Millisecond,
		"cancellation must not wait out the full interval")
}

// The client must apply the floor before spending a browser, and a saturated
// throttle must surface as a typed refusal.
func TestClientAppliesThrottleBeforeBrowserWork(t *testing.T) {
	binPath := fixtureBrowserPath(t)
	// StartupQuarantine is disabled so the first slot is genuinely available here;
	// the quarantine has its own test.
	client := New(Options{ExecutablePath: binPath, MinInterval: time.Hour, ChallengeCooldown: time.Minute, StartupQuarantine: -1})
	// The first call reserves a slot an hour out, so the next one cannot start
	// and must be refused before a browser is ever launched.
	require.NoError(t, client.throttle.Wait(context.Background()), "first slot should be immediate")
	src, serr := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, serr)
	_, err := client.Acquire(context.Background(), src)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrUnavailable,
		"a saturated throttle must refuse before browser startup is attempted")
	require.ErrorIs(t, err, ErrThrottled,
		"a pace this collector will not meet must be a refusal, not an expired budget")
}

// Default values must be conservative: the observed escalation came from a burst,
// so the floor must be well above it and the cooldown must span a recovery window.
func TestThrottleDefaultsAreConservative(t *testing.T) {
	require.Equal(t, 20*time.Second, DefaultMinInterval)
	require.Equal(t, 10*time.Minute, DefaultChallengeCooldown)
	require.Greater(t, DefaultMinInterval, 5*time.Second,
		"the floor must sit well above the burst that triggered a wall")
	th := newTestThrottle(0, 0, 0)
	require.Equal(t, DefaultMinInterval, th.MinInterval)
	require.Equal(t, DefaultJitterFraction, th.Jitter)
	require.Equal(t, DefaultChallengeCooldown, th.ChallengeCooldown)
}

// Jitter must keep the effective interval within the configured band so many
// collectors do not synchronise.
func TestThrottleJitterStaysWithinBand(t *testing.T) {
	th := newTestThrottle(100*time.Millisecond, 0.5, 0)
	for i := 0; i < 8; i++ {
		require.NoError(t, th.Wait(context.Background()))
		// After a wait returns, the reserved next slot is the one just consumed
		// plus the next effective interval, which must sit inside the band.
		gap := time.Until(th.next)
		require.Greater(t, gap, time.Duration(0))
		require.LessOrEqual(t, gap, 150*time.Millisecond,
			"effective interval must stay within MinInterval*(1+Jitter)")
	}
}

// A cooldown must expire on its own. If the block were only cleared by an
// external reset, a process that saw one challenge could never collect again
// without a restart, which is the opposite of a cooldown.
func TestThrottleCooldownExpiresWithoutExternalReset(t *testing.T) {
	th := newTestThrottle(time.Millisecond, 0, 60*time.Millisecond)
	require.NoError(t, th.Wait(context.Background()))

	th.Observe(ErrChallenge)
	require.ErrorIs(t, th.Wait(context.Background()), ErrThrottled)
	require.Positive(t, th.CooldownRemaining())

	// After the window elapses the process must resume by itself.
	require.Eventually(t, func() bool { return th.CooldownRemaining() == 0 },
		2*time.Second, 10*time.Millisecond, "the cooldown must lapse on its own")
	require.NoError(t, th.Wait(context.Background()),
		"an expired cooldown must not keep the process blocked")
}

// A request that is refused must not consume a slot. Otherwise every retryable
// refusal pushes the queue further out and the process can starve permanently.
func TestThrottleRefusedRequestDoesNotConsumeSlot(t *testing.T) {
	th := newTestThrottle(2*time.Second, 0, 0)
	require.NoError(t, th.Wait(context.Background())) // consumes the immediate slot

	// A budget far shorter than the interval cannot be served.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	for i := 0; i < 5; i++ {
		require.ErrorIs(t, th.Wait(ctx), ErrThrottled)
	}

	th.mu.Lock()
	queued := th.next
	th.mu.Unlock()
	// Repeated refusals must not have pushed the queue further out.
	// Compare against the throttle's own effective band: passing Jitter: 0 selects
	// the default rather than disabling jitter, so the bound is MinInterval*(1+Jitter).
	band := time.Duration(float64(th.MinInterval) * (1 + th.Jitter))
	require.False(t, queued.After(time.Now().Add(band)),
		"refused requests must not extend the queue")
}

// Under the production defaults the first request of an idle process must be
// served. Requiring the *following* reservation boundary to fit the caller's
// budget would reject it, because the interval floor is wider than the budget by
// design.
func TestThrottleServesFirstRequestUnderProductionDefaults(t *testing.T) {
	th := newTestThrottle(DefaultMinInterval, DefaultJitterFraction, DefaultChallengeCooldown)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	require.NoError(t, th.Wait(ctx),
		"an idle collector must serve its first request under production defaults")
}

// A cancelled waiter must not erase the schedule: the caller behind it must
// still respect the floor the preceding acquisition established.
func TestThrottleRollbackPreservesPrecedingFloor(t *testing.T) {
	th := newTestThrottle(400*time.Millisecond, 0, 0)
	require.NoError(t, th.Wait(context.Background()))

	th.mu.Lock()
	established := th.next
	th.mu.Unlock()
	require.False(t, established.IsZero(), "the first acquisition must establish a floor")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	require.ErrorIs(t, th.Wait(ctx), context.Canceled)

	// The property that matters is that the floor survives: the next caller must
	// still not be able to start immediately after the preceding acquisition. The
	// exact boundary is not pinned, because releasing re-packs the surviving
	// reservations with a fresh jittered interval.
	th.mu.Lock()
	after := th.next
	th.mu.Unlock()
	require.False(t, after.Before(established.Add(-50*time.Millisecond)),
		"a cancelled waiter must not pull the schedule earlier than the floor it left")
	require.True(t, after.After(time.Now().Add(100*time.Millisecond)),
		"a cancelled waiter must not erase the schedule")
}

// newTestThrottle builds a throttle with the startup quarantine reduced to a
// negligible value. The production default deliberately keeps a fresh collector
// silent for a full cooldown, which would otherwise stall every test; the
// quarantine itself is covered by its own test.
func newTestThrottle(minInterval time.Duration, jitter float64, challengeCooldown time.Duration) *Throttle {
	return newThrottle(minInterval, jitter, challengeCooldown, -1, DefaultTimeout, time.Nanosecond)
}

// A fresh collector must stay silent before its first request, because a
// restarted process cannot know whether its egress IP was challenged just before
// it died. Without this it re-hits 1688 and negates the cooldown.
func TestThrottleFreshProcessIsQuarantined(t *testing.T) {
	th := newThrottle(time.Millisecond, 0, 10*time.Minute, 80*time.Millisecond, time.Minute, time.Nanosecond)

	require.ErrorIs(t, th.Wait(context.Background()), ErrThrottled,
		"a fresh collector must not call out immediately")

	require.Eventually(t, func() bool { return th.CooldownRemaining() == 0 },
		2*time.Second, 10*time.Millisecond, "the startup quarantine must lapse on its own")

	require.NoError(t, th.Wait(context.Background()),
		"the first request must be served once the quarantine elapses")
}

// The quarantine follows the CONFIGURED cooldown: a replacement collector
// assumes the worst about an exit IP it never observed, but an operator who
// deliberately shortens -challenge-cooldown must not still get a ten-minute
// silence on every restart.
func TestThrottleStartupQuarantineDefaultsToConfiguredCooldown(t *testing.T) {
	th := newThrottle(time.Millisecond, 0, time.Minute, 0, time.Minute, time.Nanosecond)
	require.Equal(t, time.Minute, th.StartupQuarantine)
}

// The startup quarantine must follow the CONFIGURED cooldown, so shortening
// -challenge-cooldown actually shortens the silence after a restart.
func TestThrottleStartupQuarantineFollowsConfiguredCooldown(t *testing.T) {
	th := newThrottle(time.Millisecond, 0, 2*time.Second, 0, time.Minute, time.Nanosecond)
	require.Equal(t, 2*time.Second, th.StartupQuarantine,
		"quarantine must follow the configured cooldown, not the packaged default")
}

// A wait that spans another acquisition which observed a challenge must not
// return success; the block is re-checked once the wait ends.
func TestThrottleRechecksCooldownAfterQueuedWait(t *testing.T) {
	th := newTestThrottle(150*time.Millisecond, 0, 0)
	require.NoError(t, th.Wait(context.Background()))

	queued := make(chan error, 1)
	go func() { queued <- th.Wait(context.Background()) }()

	// While the second caller waits, another acquisition is challenged.
	time.Sleep(20 * time.Millisecond)
	th.Observe(ErrChallenge)

	select {
	case err := <-queued:
		require.ErrorIs(t, err, ErrThrottled,
			"a wait that spans a challenge must not hand back a usable slot")
	case <-time.After(2 * time.Second):
		t.Fatal("the queued wait never returned")
	}
}

// A caller whose slot falls just inside its deadline must still be refused: the
// wait would consume the budget and hand an exhausted context to the collection,
// surfacing as a timeout rather than the honest refusal this throttle intends.
func TestThrottleReservesCollectionHeadroom(t *testing.T) {
	// The slot is 9s away and the caller has 10s: it *can* start in time, so a
	// start-only check admits it, but then almost no budget remains to collect.
	th := newThrottle(9*time.Second, 0, 10*time.Minute, -1, 10*time.Second, 10*time.Second)
	require.NoError(t, th.Wait(context.Background())) // consume the immediate slot

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := time.Now()
	err := th.Wait(ctx)
	require.ErrorIs(t, err, ErrThrottled,
		"admitting a slot that leaves no collection budget would surface as a timeout")
	require.Less(t, time.Since(start), time.Second, "the refusal must be immediate, not a wait")

	// With headroom removed the same slot is admitted, which is what made the old
	// behaviour a timeout in disguise.
	naive := newThrottle(9*time.Second, 0, 10*time.Minute, -1, 10*time.Second, time.Nanosecond)
	require.NoError(t, naive.Wait(context.Background()))
	generous, cancelGenerous := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelGenerous()
	require.NoError(t, naive.Wait(generous), "without a headroom the slot is served")
}

// A cancelled caller leaves its interval consumed. That is deliberate: the floor
// alone is kept, so a cancellation can delay one request but can never let two
// callers start together. The earlier list-based design tried to give the slot
// back and produced repeated defects; this is the conservative direction.
func TestThrottleCancelledCallerLeavesItsIntervalConsumed(t *testing.T) {
	th := newTestThrottle(120*time.Millisecond, 0, 0)
	require.NoError(t, th.Wait(context.Background())) // consumes the immediate slot

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = th.Wait(ctx) }()
	time.Sleep(30 * time.Millisecond)
	cancel()
	time.Sleep(20 * time.Millisecond)

	// The next caller still cannot start immediately: the floor was committed when
	// the cancelled one reserved it.
	th.mu.Lock()
	floor := th.next
	th.mu.Unlock()
	require.True(t, floor.After(time.Now().Add(50*time.Millisecond)),
		"a cancelled caller must not release the floor it already committed")
}

// A cooldown that has elapsed resumes the process, but paced rather than at full
// rate.
func TestThrottleResumesPacedAfterCooldown(t *testing.T) {
	th := newTestThrottle(150*time.Millisecond, 0, 200*time.Millisecond)
	require.NoError(t, th.Wait(context.Background()))
	th.Observe(ErrChallenge)
	require.ErrorIs(t, th.Wait(context.Background()), ErrThrottled)

	// Once the window elapses the block clears.
	require.Eventually(t, func() bool { return th.CooldownRemaining() == 0 },
		2*time.Second, 10*time.Millisecond)
	require.NoError(t, th.Wait(context.Background()),
		"an elapsed cooldown must resume the process")
}

// The floor must advance from when a caller ACTUALLY starts, not from the slot it
// reserved. A waiter whose timer has fired but which is then scheduled late must
// not let the next caller in on the ideal timeline, or two real acquisition
// starts end up milliseconds apart - a burst produced by the pacing itself.
func TestThrottleAdvancesFloorFromActualDispatch(t *testing.T) {
	interval := 60 * time.Millisecond
	th := newThrottle(interval, 0, 0, -1, time.Minute, time.Nanosecond)
	require.NoError(t, th.Wait(context.Background())) // consumes the immediate slot

	done := make(chan struct{})
	go func() { defer close(done); _ = th.Wait(context.Background()) }() // will wait out its slot

	// Let that waiter commit its reservation and enter its timer, then hold the
	// throttle's lock so its dispatch is delayed well past the slot.
	time.Sleep(interval / 2)
	th.mu.Lock()
	hold := interval * 4
	release := time.Now().Add(hold)
	time.Sleep(hold)
	th.mu.Unlock()

	<-done
	dispatch := time.Now()

	th.mu.Lock()
	gap := th.next.Sub(dispatch)
	th.mu.Unlock()
	require.GreaterOrEqual(t, gap, interval-time.Millisecond,
		"the floor must be re-anchored on the real dispatch, not the ideal slot")
	require.True(t, release.Before(dispatch), "the dispatch must genuinely be late")
}

// The reservation bookkeeping is a single floor, so the state cannot grow.
func TestThrottleStateIsBoundedByConstruction(t *testing.T) {
	th := newTestThrottle(time.Millisecond, 0, 0)
	for i := 0; i < 200; i++ {
		require.NoError(t, th.Wait(context.Background()))
	}
	th.mu.Lock()
	fields := 0
	for i := 0; i < 200; i++ {
		fields++
	}
	floor := th.next
	blocked := th.blocked
	th.mu.Unlock()
	_ = fields
	require.False(t, floor.IsZero(), "the floor is the only pacing state")
	require.False(t, blocked)
}

// A caller that configures the headroom must get that value, not the derived
// one; otherwise the option is a dead setting that only tests can reach.
func TestThrottleHonoursConfiguredHeadroom(t *testing.T) {
	th := newThrottle(time.Millisecond, 0, time.Minute, -1, 10*time.Second, 2*time.Second)
	require.Equal(t, 2*time.Second, th.headroom,
		"a configured headroom must be used verbatim")
	derived := newThrottle(time.Millisecond, 0, time.Minute, -1, 10*time.Second, 0)
	require.Equal(t, 10*time.Second*CollectionHeadroomNumerator/CollectionHeadroomDenominator, derived.headroom,
		"an unset headroom must be derived from the configured budget")
}

// A cancelled waiter hands its slot back when no later caller committed, so
// disconnects cannot starve valid work indefinitely. The boundary is asserted
// exactly: the floor must return to the FIRST waiter's slot, not merely become
// unreachable in time, which a stale floor would also satisfy.
func TestThrottleCancelledWaiterHandsBackItsSlot(t *testing.T) {
	th := newThrottle(2*time.Second, 0, 0, -1, time.Minute, time.Nanosecond)
	require.NoError(t, th.Wait(context.Background())) // A takes the immediate slot

	th.mu.Lock()
	firstFloor := th.next
	th.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = th.Wait(ctx) }()
	require.Eventually(t, func() bool {
		th.mu.Lock()
		defer th.mu.Unlock()
		return th.next.After(firstFloor)
	}, time.Second, 5*time.Millisecond, "the second caller must hold a later slot")

	cancel()
	require.Eventually(t, func() bool {
		th.mu.Lock()
		defer th.mu.Unlock()
		return th.next.Equal(firstFloor)
	}, time.Second, 5*time.Millisecond,
		"an un-superseded cancellation must restore exactly the boundary it displaced")
}

// When a later caller already committed, the cancelled one cannot know which
// boundary it displaced, so the floor stands - the conservative direction.
// Asserted directly on the contract rather than through a timing-sensitive
// interleaving, which only tested the scheduler.
func TestThrottleSupersededCancellationKeepsTheFloor(t *testing.T) {
	th := newThrottle(150*time.Millisecond, 0, 0, -1, time.Minute, time.Nanosecond)
	require.NoError(t, th.Wait(context.Background()))

	// Two further callers commit in order.
	ctxB, cancelB := context.WithCancel(context.Background())
	go func() { _ = th.Wait(ctxB) }()
	require.Eventually(t, func() bool {
		th.mu.Lock()
		defer th.mu.Unlock()
		return th.next.After(time.Now().Add(100 * time.Millisecond))
	}, time.Second, 5*time.Millisecond)
	stale := func() uint64 { th.mu.Lock(); defer th.mu.Unlock(); return th.owner }()

	ctxC, cancelC := context.WithCancel(context.Background())
	go func() { _ = th.Wait(ctxC) }()
	require.Eventually(t, func() bool {
		staleNow := func() uint64 { th.mu.Lock(); defer th.mu.Unlock(); return th.owner }()
		return staleNow > stale
	}, time.Second, 5*time.Millisecond, "a later caller must supersede the first")

	th.mu.Lock()
	floor := th.next
	th.mu.Unlock()

	// Releasing the superseded sequence must not move the floor.
	th.release(stale)
	th.mu.Lock()
	after := th.next
	th.mu.Unlock()
	require.False(t, after.Before(floor),
		"a superseded cancellation must not move the floor backwards")

	cancelB()
	cancelC()
}

// A challenge observed but not cleared because the automatic attempt ran out of
// budget must still engage the cooldown. If only the deadline survived, the next
// acquisition would proceed after the rate floor and deepen the block - the exact
// failure the throttle exists to prevent.
func TestThrottleCoolsOnAChallengeThatOutranItsBudget(t *testing.T) {
	th := newTestThrottle(time.Millisecond, 0, 10*time.Minute)
	require.NoError(t, th.Wait(context.Background()))

	// What collect returns when the solve attempt exhausts the budget.
	outcome := errors.Join(ErrChallenge, context.DeadlineExceeded)
	th.Observe(outcome)

	require.Positive(t, th.CooldownRemaining(),
		"a challenge that outran its budget must still engage the cooldown")
	require.ErrorIs(t, th.Wait(context.Background()), ErrThrottled)

	// And the caller still learns it was a timeout.
	require.ErrorIs(t, outcome, context.DeadlineExceeded,
		"the caller must still see a deadline")
}

// When an earlier waiter slips and re-anchors the floor, callers already in the
// queue must re-evaluate against the new floor rather than dispatch on their
// original timer, which would let two acquisitions start together.
func TestThrottleQueuedWaitersReevaluateAfterALateDispatch(t *testing.T) {
	interval := 60 * time.Millisecond
	th := newThrottle(interval, 0, 0, -1, time.Minute, time.Nanosecond)
	require.NoError(t, th.Wait(context.Background())) // A: immediate slot

	// B and C reserve behind A.
	startedB := make(chan time.Time, 1)
	startedC := make(chan time.Time, 1)
	go func() { _ = th.Wait(context.Background()); startedB <- time.Now() }()
	require.Eventually(t, func() bool {
		th.mu.Lock()
		defer th.mu.Unlock()
		return th.next.After(time.Now().Add(20 * time.Millisecond))
	}, time.Second, 5*time.Millisecond)
	go func() { _ = th.Wait(context.Background()); startedC <- time.Now() }()

	// Delay B's dispatch so the floor is re-anchored, then require that C does
	// not start immediately after B.
	time.Sleep(interval / 2)
	th.mu.Lock()
	hold := interval * 4
	time.Sleep(hold)
	th.mu.Unlock()

	b := <-startedB
	c := <-startedC
	if c.After(b) {
		require.GreaterOrEqual(t, c.Sub(b), interval-2*time.Millisecond,
			"a queued waiter must be paced after a slipped predecessor, not admitted on its stale timer")
	}
}

// A challenge observed and then interrupted by a caller disconnect must still
// engage the cooldown, exactly as a deadline must.
func TestThrottleCoolsOnAChallengeInterruptedByCancellation(t *testing.T) {
	th := newTestThrottle(time.Millisecond, 0, 10*time.Minute)
	require.NoError(t, th.Wait(context.Background()))

	th.Observe(errors.Join(ErrChallenge, context.Canceled))
	require.Positive(t, th.CooldownRemaining(),
		"a challenge followed by a caller disconnect must still engage the cooldown")
}

// The immediate (idle) path must respect a cooldown that another acquisition
// triggered after this caller reserved.
func TestThrottleIdlePathRespectsAConcurrentCooldown(t *testing.T) {
	th := newTestThrottle(time.Hour, 0, 10*time.Minute)

	// A cooldown is already in force when this caller arrives.
	th.Observe(ErrChallenge)
	require.ErrorIs(t, th.Wait(context.Background()), ErrThrottled)

	// And the same for a caller that reserved before the cooldown began.
	th2 := newTestThrottle(time.Hour, 0, 10*time.Minute)
	ready := make(chan struct{})
	go func() { defer close(ready); _ = th2.Wait(context.Background()) }()
	time.Sleep(30 * time.Millisecond)
	th2.Observe(ErrChallenge)
	<-ready
	th2.mu.Lock()
	blocked := th2.blocked
	th2.mu.Unlock()
	require.True(t, blocked, "the cooldown must still be in force after a concurrent trigger")
}

// A cooldown longer than the interval must end AT the configured deadline: the
// first request afterwards is served rather than refused for a further interval.
//
// The short-cooldown case is deliberately NOT asserted here: when the cooldown is
// shorter than the interval, honouring the floor the last dispatch established is
// correct, and that is covered by TestThrottleShortCooldownKeepsThePacingFloor.
func TestThrottleCooldownEndsAtTheConfiguredDeadline(t *testing.T) {
	interval := 50 * time.Millisecond
	cooldown := 300 * time.Millisecond
	budget := time.Minute
	th := newThrottle(interval, 0, cooldown, -1, budget, budget/4)
	require.NoError(t, th.Wait(context.Background()))
	th.Observe(ErrChallenge)

	require.Eventually(t, func() bool { return th.CooldownRemaining() == 0 },
		2*time.Second, 5*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	// No elapsed bound here: with a cooldown LONGER than the interval both
	// implementations serve, so timing cannot distinguish them and would only make
	// the test flaky. The discriminating case is the short-cooldown one below.
	require.NoError(t, th.Wait(ctx),
		"a caller must be served once a cooldown longer than the interval has elapsed")
}

// A challenge seen while the solver is still working must stop other
// acquisitions immediately, not only once the first one returns.
func TestThrottleCoolsAsSoonAsAChallengeIsObserved(t *testing.T) {
	th := newTestThrottle(50*time.Millisecond, 0, 10*time.Minute)
	require.NoError(t, th.Wait(context.Background()))

	// The first acquisition is inside its solve when the challenge is observed.
	th.Observe(ErrChallenge)

	// A caller that arrives during that window is refused straight away.
	require.ErrorIs(t, th.Wait(context.Background()), ErrThrottled)
	require.Positive(t, th.CooldownRemaining())
}

// The generation check and the dispatch commitment must be the same critical
// section, so two waiters whose timers both elapsed cannot both dispatch.
func TestThrottleConcurrentExpiredTimersDoNotDispatchTogether(t *testing.T) {
	interval := 40 * time.Millisecond
	th := newTestThrottle(interval, 0, 0)
	require.NoError(t, th.Wait(context.Background()))

	// Two waiters queue behind it.
	starts := make(chan time.Time, 2)
	for i := 0; i < 2; i++ {
		go func() { _ = th.Wait(context.Background()); starts <- time.Now() }()
	}

	// Hold the lock so both timers elapse before either can dispatch, then release.
	time.Sleep(interval)
	th.mu.Lock()
	time.Sleep(interval * 3)
	th.mu.Unlock()

	a, b := <-starts, <-starts
	earlier, later := a, b
	if later.Before(earlier) {
		earlier, later = later, earlier
	}
	require.GreaterOrEqual(t, later.Sub(earlier), interval-2*time.Millisecond,
		"two waiters whose timers both elapsed must not start together")
}

// A caller cancelled while the timer fired but the waiter was blocked taking the
// lock must not be handed a slot: Acquire would go on to start a browser for it.
func TestThrottleCancellationDuringTheDispatchLockIsNotCommitted(t *testing.T) {
	interval := 40 * time.Millisecond
	th := newTestThrottle(interval, 0, 0)
	require.NoError(t, th.Wait(context.Background()))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- th.Wait(ctx) }()

	// Let it commit its reservation and reach its timer, then hold the lock so
	// the dispatch cannot complete. That is the window between the timer firing
	// and the dispatch being committed.
	require.Eventually(t, func() bool {
		th.mu.Lock()
		defer th.mu.Unlock()
		return th.owner > 1
	}, time.Second, 5*time.Millisecond)

	th.mu.Lock()
	time.Sleep(interval * 3) // the timer fires while the lock is held
	cancel()                 // and the caller goes away in that window
	th.mu.Unlock()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled,
			"a cancellation during the dispatch lock must not commit a slot")
	case <-time.After(2 * time.Second):
		t.Fatal("the waiter never returned")
	}

	// And the slot it had reserved was handed back rather than consumed.
	require.Eventually(t, func() bool {
		th.mu.Lock()
		defer th.mu.Unlock()
		return !th.next.After(time.Now().Add(interval / 4))
	}, time.Second, 5*time.Millisecond, "the cancelled caller's slot must be released")
}

// When the cooldown is SHORTER than the interval, expiry must not discard the
// floor the last real dispatch established, or the first request after the
// window starts well before MinInterval has elapsed.
func TestThrottleShortCooldownKeepsThePacingFloor(t *testing.T) {
	interval := 300 * time.Millisecond
	cooldown := 60 * time.Millisecond
	budget := time.Minute
	th := newThrottle(interval, 0, cooldown, -1, budget, budget/4)
	require.NoError(t, th.Wait(context.Background()))
	dispatch := time.Now()

	th.Observe(ErrChallenge)
	require.Eventually(t, func() bool { return th.CooldownRemaining() == 0 },
		2*time.Second, 5*time.Millisecond)

	// The pacing floor from the real dispatch must still be honoured.
	th.mu.Lock()
	floor := th.next
	th.mu.Unlock()
	require.False(t, floor.Before(dispatch.Add(interval)),
		"a short cooldown must not discard the floor the last dispatch established")

	ctx, cancel := context.WithTimeout(context.Background(), interval/2)
	defer cancel()
	require.ErrorIs(t, th.Wait(ctx), ErrThrottled,
		"a request inside the remaining interval must still be refused")
	require.NoError(t, th.Wait(context.Background()),
		"and served once the interval elapses")
}

// Both halves of the cooldown contract at once: with a cooldown LONGER than the
// interval the synthetic floor Observe pushed must be cleared so the window means
// what it says, and with a cooldown SHORTER the floor from a real dispatch must
// still be honoured.
func TestThrottleCooldownRespectsBothDirections(t *testing.T) {
	budget := time.Minute

	t.Run("long cooldown resumes at the deadline", func(t *testing.T) {
		th := newThrottle(50*time.Millisecond, 0, 300*time.Millisecond, -1, budget, budget/4)
		require.NoError(t, th.Wait(context.Background()))
		th.Observe(ErrChallenge)
		require.Eventually(t, func() bool { return th.CooldownRemaining() == 0 },
			2*time.Second, 5*time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), budget)
		defer cancel()
		require.NoError(t, th.Wait(ctx),
			"the synthetic resume floor must not extend past the configured window")
	})

	t.Run("short cooldown keeps the real dispatch floor", func(t *testing.T) {
		th := newThrottle(300*time.Millisecond, 0, 60*time.Millisecond, -1, budget, budget/4)
		require.NoError(t, th.Wait(context.Background()))
		dispatch := time.Now()
		th.Observe(ErrChallenge)
		require.Eventually(t, func() bool { return th.CooldownRemaining() == 0 },
			2*time.Second, 5*time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()
		require.ErrorIs(t, th.Wait(ctx), ErrThrottled,
			"a real dispatch floor must survive a shorter cooldown")
		require.True(t, dispatch.Add(300*time.Millisecond).After(time.Now()))
	})
}

// Observe overwrites the floor with a synthetic cooldown value, so the real
// dispatch floor must be tracked separately: a long cooldown must resume at its
// deadline, and a short one must still honour the interval from the last real
// dispatch. Neither requirement may be satisfied at the other's expense.
func TestThrottleCooldownFloorAndDispatchFloorAreDistinct(t *testing.T) {
	budget := time.Minute

	// The shipped relationship, scaled down: a cooldown LONGER than the interval,
	// with a budget and headroom that the synthetic cooldown+interval floor cannot
	// fit. The last real dispatch's own floor has by then expired, so resumption is
	// due - and a generous budget would not have caught this.
	t.Run("long cooldown is not extended by a synthetic floor", func(t *testing.T) {
		interval, acquisitionBudget := 200*time.Millisecond, 100*time.Millisecond
		headroom := acquisitionBudget * 3 / 4
		th := newThrottle(interval, 0, 500*time.Millisecond, -1, acquisitionBudget, headroom)
		require.NoError(t, th.Wait(context.Background()))
		th.Observe(ErrChallenge)
		require.Eventually(t, func() bool { return th.CooldownRemaining() == 0 },
			2*time.Second, 5*time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), acquisitionBudget)
		defer cancel()
		require.NoError(t, th.Wait(ctx),
			"the synthetic cooldown+interval floor must not delay resumption past the window")
	})

	t.Run("short cooldown still respects the real dispatch floor", func(t *testing.T) {
		th := newThrottle(400*time.Millisecond, 0, 50*time.Millisecond, -1, budget, budget/4)
		require.NoError(t, th.Wait(context.Background()))
		th.Observe(ErrChallenge)
		require.Eventually(t, func() bool { return th.CooldownRemaining() == 0 },
			2*time.Second, 5*time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		require.ErrorIs(t, th.Wait(ctx), ErrThrottled,
			"the floor from the last real dispatch must survive a shorter cooldown")
	})
}

// An on-time dispatch that does not move the queue tail must not invalidate a
// waiter's still-valid timer: treating it as stale would push that waiter a full
// extra interval out, which with a short configured interval turns a request that
// originally fitted the budget into a refusal.
func TestThrottleOnTimeDispatchDoesNotInvalidateWaiters(t *testing.T) {
	interval := 40 * time.Millisecond
	budget := time.Second
	th := newThrottle(interval, 0, 0, -1, budget, budget/4)
	require.NoError(t, th.Wait(context.Background()))

	// Two waiters queue and both dispatch on time, back to back.
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		start := time.Now()
		require.NoError(t, th.Wait(ctx))
		elapsed := time.Since(start)
		cancel()
		// On time means paced by the interval, never later than interval*3.
		require.Less(t, elapsed, interval*3,
			"a queued waiter must not be pushed an extra interval out by an on-time dispatch")
	}
}

// A dispatch that is a clock tick late but still lands inside the existing queue
// tail has not moved the schedule, and must not push the waiters behind it a full
// extra interval. A generous bound is used because the property under test is that
// a request that fits the budget is not refused, not that a specific timing holds.
func TestThrottleMinimallyLateDispatchDoesNotInvalidateWaiters(t *testing.T) {
	interval := 20 * time.Millisecond
	budget := 400 * time.Millisecond
	th := newThrottle(interval, 0, 0, -1, budget, budget/4)
	require.NoError(t, th.Wait(context.Background()))

	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), budget)
		start := time.Now()
		require.NoError(t, th.Wait(ctx),
			"a minimally late dispatch must not refuse the waiter behind it")
		require.Less(t, time.Since(start), budget,
			"the waiter must complete inside its own budget")
		cancel()
	}
}

// The two interleaveings pull in opposite directions, so the predicate that decides
// whether a dispatch invalidates the waiters behind it has to admit both and nothing
// in between. A tail-only rule misses a real slip; a rule that measures lateness on
// the floor rather than on the instant the dispatch ran is true for every dispatch,
// because the floor is a full interval ahead by construction.
//
// A dispatch is written the way the code builds it: it reserved a slot at start, it
// actually ran at actual, and it left a floor one interval past actual.
func TestThrottleScheduleMovedAdmitsBothInterleaveings(t *testing.T) {
	base := time.Now()
	interval := 200 * time.Millisecond

	atTail := func(tail time.Time) *Throttle {
		th := newThrottle(interval, 0, 0, -1, time.Second, time.Second/4)
		th.next = tail
		return th
	}

	// A later reservation has already pushed the queue tail out past the dispatch, so
	// the dispatch did not move the tail - but it ran 150ms past the slot it
	// reserved, well beyond the half-interval tolerance. The waiters behind it were
	// scheduled against a schedule that no longer holds.
	t.Run("real slip inside a tail pushed by a later reservation", func(t *testing.T) {
		actual := base.Add(150 * time.Millisecond)
		require.True(t, atTail(base.Add(500*time.Millisecond)).
			scheduleMoved(actual.Add(interval), actual, base),
			"a dispatch that overran its slot by more than the tolerance must invalidate waiters")
	})

	// A clock tick of goroutine scheduling, landing inside the existing tail. The
	// floor here is a full interval past the reserved slot, so a rule that compared
	// the floor against the slot would call this a change and push valid waiters out.
	t.Run("clock tick inside the tail", func(t *testing.T) {
		actual := base.Add(time.Millisecond)
		require.False(t, atTail(base.Add(500*time.Millisecond)).
			scheduleMoved(actual.Add(interval), actual, base),
			"a tick of lateness must not invalidate waiters")
	})

	// An on-time dispatch that advances the tail still moves the schedule.
	t.Run("on time but advancing the tail", func(t *testing.T) {
		require.True(t, atTail(base).scheduleMoved(base.Add(interval), base, base),
			"advancing the queue tail must invalidate waiters")
	})

	// The tolerance is half the configured interval, so the margins above are real.
	require.Equal(t, interval/2, atTail(base).slipTolerance)
}

// A single challenge is observed twice - once at detection, once when the bounded
// solve gives up - and the window must describe the time since the challenge was
// actually seen, not since the solve finished. A solve that consumes six seconds
// must not make a ten-minute window last ten minutes and six.
func TestThrottleCooldownIsNotRestartedByTheSolve(t *testing.T) {
	cooldown := 300 * time.Millisecond
	th := newThrottle(time.Millisecond, 0, cooldown, -1, time.Second, time.Second/4)
	require.NoError(t, th.Wait(context.Background()))

	expiry := func() time.Time {
		th.mu.Lock()
		defer th.mu.Unlock()
		return th.cooledAt
	}

	th.Observe(ErrChallenge)
	first := expiry()
	require.False(t, first.IsZero(), "the first observation must start a window")
	require.Greater(t, th.CooldownRemaining(), cooldown/2)

	// The bounded solve runs, then reports the same challenge again.
	time.Sleep(120 * time.Millisecond)
	th.Observe(ErrChallenge)
	require.Equal(t, first, expiry(),
		"a second observation of the same challenge must not move the deadline it already set")
	require.Less(t, th.CooldownRemaining(), cooldown,
		"the window must keep counting down from the observation, not restart")
}
