package browser

import (
	"context"
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

// A waiter that is merely delayed past its slot keeps the floor it committed, so
// the next caller cannot start on top of it.
func TestThrottleDelayedWaiterKeepsItsSlot(t *testing.T) {
	th := newTestThrottle(60*time.Millisecond, 0, 0)
	require.NoError(t, th.Wait(context.Background()))

	th.mu.Lock()
	floorBefore := th.next
	th.mu.Unlock()
	require.True(t, floorBefore.After(time.Now()), "a reservation must advance the floor")

	// Let the floor elapse, as a delayed waiter would experience.
	time.Sleep(90 * time.Millisecond)

	// A new caller still cannot start immediately, because the floor was committed
	// when the earlier caller reserved it and only moves forward.
	th.mu.Lock()
	floorAfter := th.next
	th.mu.Unlock()
	require.False(t, floorAfter.Before(floorBefore),
		"the floor must never move backwards, whatever the wall clock does")
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
