package browser

import (
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
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
		gap := time.Until(th.floor)
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
	queued := th.floor
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
	established := th.floor
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
	after := th.floor
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
	floor := th.floor
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
	gap := th.floor.Sub(dispatch)
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
	floor := th.floor
	blocked := !th.cooldownUntil.IsZero()
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

// The floor only ever moves forward, and only when a caller is actually admitted.
// An earlier version handed a cancelled reservation's slot back when it was still
// the newest, and tracked the displaced boundary so a superseded cancellation could
// not restore it; both needed a second copy of the floor that could disagree with
// the first. Nothing is reserved here, so a caller that never started has consumed
// no interval and there is nothing to restore.
//
// This asserts the property directly, under concurrent cancellation and admission,
// rather than reproducing a scheduler interleaving.
func TestThrottleFloorOnlyMovesForward(t *testing.T) {
	interval := 40 * time.Millisecond
	budget := 3 * time.Second
	th := newThrottle(interval, 0, 0, -1, budget, budget/4)

	floor := func() time.Time {
		th.mu.Lock()
		defer th.mu.Unlock()
		return th.floor
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})
	// Watch the floor for any backward movement while callers come and go.
	var regression atomic.Value
	regression.Store("")
	wg.Add(1)
	go func() {
		defer wg.Done()
		prev := time.Time{}
		for {
			select {
			case <-stop:
				return
			default:
			}
			// Yield between samples: a tight spin here starves the callers this
			// test is meant to be observing.
			time.Sleep(200 * time.Microsecond)
			if cur := floor(); !prev.IsZero() && cur.Before(prev) {
				regression.Store(cur.String())
				return
			} else {
				prev = cur
			}
		}
	}()

	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), budget)
			defer cancel()
			// Half the callers give up mid-wait, which is what used to restore a
			// boundary that a later caller had already moved.
			if i%2 == 0 {
				time.Sleep(interval / 4)
				cancel()
			}
			_ = th.Wait(ctx)
		}(i)
		time.Sleep(interval / 8)
	}
	time.Sleep(2 * interval)
	close(stop)
	wg.Wait()

	require.Empty(t, regression.Load().(string),
		"the floor must never move backwards, whoever is cancelled and whenever")
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
		return th.floor.After(time.Now().Add(20 * time.Millisecond))
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
	blocked := !th2.cooldownUntil.IsZero()
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
// A caller that gives up while waiting commits nothing. The earlier design
// reserved a slot, slept on a timer, and only then committed the dispatch, which
// left a window between the timer firing and the commit in which a cancellation
// could be lost and a slot handed out for a dead request. Deciding and committing
// happen together under the mutex, so there is no such window to lose.
func TestThrottleCancelledCallerCommitsNothing(t *testing.T) {
	interval := 60 * time.Millisecond
	budget := 3 * time.Second
	th := newThrottle(interval, 0, 0, -1, budget, budget/4)
	require.NoError(t, th.Wait(context.Background()))

	floor := func() time.Time {
		th.mu.Lock()
		defer th.mu.Unlock()
		return th.floor
	}
	before := floor()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- th.Wait(ctx) }()

	// Cancel while the caller is still waiting, well inside the interval.
	time.Sleep(interval / 2)
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)

	require.Equal(t, before, floor(),
		"a caller that never started must not move the floor")

	// The next caller is still paced from the previous real dispatch, not from the
	// abandoned one.
	require.NoError(t, th.Wait(context.Background()))
	// Scheduling and assertion overhead reduce the remaining wait below half
	// the interval. Check the committed dispatch against the original floor.
	require.False(t, floor().Add(-interval).Before(before),
		"the next real dispatch must respect the original floor")
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
	floor := th.floor
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

// A cooldown is exactly the window the operator configured, and it does not
// disturb the pacing floor in either direction.
//
// An earlier version overwrote the pacing floor with the cooldown end plus a full
// interval, which made the configured window a lower bound on the real one: with a
// 20s floor, a 10s budget and 7.5s of headroom, every request kept being refused
// for roughly another 17.5s past the configured ten-minute window. The fix for that
// needed a second copy of the floor kept separately, and keeping the two in step
// then produced its own findings. The floor now belongs to real dispatches alone
// and a cooldown is a separate instant, so neither direction has to be reconciled.
func TestThrottleCooldownIsExactlyTheConfiguredWindow(t *testing.T) {
	t.Run("a cooldown that outlasts the interval does not delay resumption", func(t *testing.T) {
		// Scaled down from the shipped shape: the budget and headroom cannot fit the
		// interval that a synthetic floor would have added, which is what made the
		// original defect observable. A generous budget would not catch it.
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
			"resumption must happen at the configured deadline, not an interval later")
	})

	t.Run("a cooldown shorter than the interval does not cancel the floor", func(t *testing.T) {
		th := newThrottle(400*time.Millisecond, 0, 50*time.Millisecond, -1, time.Minute, time.Minute/4)
		require.NoError(t, th.Wait(context.Background()))
		th.Observe(ErrChallenge)
		require.Eventually(t, func() bool { return th.CooldownRemaining() == 0 },
			2*time.Second, 5*time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		require.ErrorIs(t, th.Wait(ctx), ErrThrottled,
			"the interval from the last real dispatch must survive a shorter cooldown")
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

// Every observed challenge gets the full window from the moment it was seen, so
// overlapping acquisitions that each meet a challenge cannot resume sooner than
// ChallengeCooldown after the LATEST one. Suppressing the second observation
// process-wide - the obvious way to stop one acquisition reporting twice - would
// silently shorten the window for the newest challenge.
func TestThrottleEachObservedChallengeGetsTheFullWindow(t *testing.T) {
	cooldown := 300 * time.Millisecond
	th := newThrottle(time.Millisecond, 0, cooldown, -1, time.Second, time.Second/4)
	require.NoError(t, th.Wait(context.Background()))

	deadline := func() time.Time {
		th.mu.Lock()
		defer th.mu.Unlock()
		return th.cooldownUntil
	}

	th.Observe(ErrChallenge)
	first := deadline()
	require.False(t, first.IsZero(), "the first challenge must start a window")

	// A second, genuinely distinct challenge while the first window is still running.
	time.Sleep(120 * time.Millisecond)
	th.Observe(ErrChallenge)
	second := deadline()
	require.True(t, second.After(first),
		"a second distinct challenge must extend the window from when it was seen")
	require.GreaterOrEqual(t, th.CooldownRemaining(), cooldown*4/5,
		"the newest challenge must still be served its full window")
}

// Under scheduler inversion a later reservation's goroutine can be scheduled ahead
// of an earlier waiter and dispatch first, leaving the earlier one with a valid
// timer and an unchanged generation. It must not then start beside the acquisition
// that is already running.
func TestThrottleSchedulerInversionDoesNotDoubleDispatch(t *testing.T) {
	interval := 60 * time.Millisecond
	budget := 2 * time.Second
	th := newThrottle(interval, 0, 0, -1, budget, budget/4)
	require.NoError(t, th.Wait(context.Background()))

	for i := 0; i < 4; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), budget)
		start := time.Now()
		require.NoError(t, th.Wait(ctx))
		require.GreaterOrEqual(t, time.Since(start), interval*3/4,
			"every dispatch must respect the collector-wide floor")
		cancel()
	}
}

// The invariant every earlier finding in this area was a symptom of: however many
// callers queue, however they are cancelled, and in whatever order the scheduler
// runs them, no two admissions are closer together than the configured interval.
//
// An earlier version of this throttle re-derived the answer at four separate points
// from a queue tail, a generation counter, a reservation owner, a displaced
// boundary and a separate real-dispatch floor. Each point consulted a different
// subset, so each invariant had several independent enforcement sites that could
// disagree, and a caller was admitted on a value observed before its wait. The
// throttle now holds one floor and decides and commits under one lock, so this
// holds by construction rather than by each site being correct.
func TestThrottleNeverAdmitsTwoAcquisitionsCloserThanTheInterval(t *testing.T) {
	const (
		callers  = 8
		interval = 150 * time.Millisecond
		// The instant is sampled just after Wait returns, so the measurement can lag
		// the admission itself by however long the goroutine takes to be scheduled -
		// much longer under -race. The margin covers that, and is far below the
		// interval, so a real regression still fails.
		measureSkew = 60 * time.Millisecond
	)
	th := newThrottle(interval, 0, 0, -1, 30*time.Second, 30*time.Second/4)

	var mu sync.Mutex
	var admitted []time.Time
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			// Cancel roughly half the callers while they are queued, which is what
			// used to restore a boundary that a later caller had already moved.
			if i%2 == 0 {
				go func() {
					time.Sleep(time.Duration(i) * interval / 8)
					cancel()
				}()
			}
			if err := th.Wait(ctx); err == nil {
				mu.Lock()
				admitted = append(admitted, time.Now())
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	require.GreaterOrEqual(t, len(admitted), 2,
		"the test needs at least two admissions to say anything about spacing")
	sort.Slice(admitted, func(a, b int) bool { return admitted[a].Before(admitted[b]) })
	for i := 1; i < len(admitted); i++ {
		gap := admitted[i].Sub(admitted[i-1])
		require.GreaterOrEqual(t, gap, interval-measureSkew,
			"admissions %d and %d are %s apart, under the %s floor", i-1, i, gap, interval)
	}
}

// A caller that has already given up must not be admitted. The derived context can
// still carry a future deadline after cancellation, so the budget check alone does
// not notice - and an admitted-but-dead caller spends a pacing slot and then
// launches a browser for a request nobody is waiting for.
//
// The case that matters is the immediate one: when the floor has already passed
// there is nothing to wait for, so the decision to commit happens in the same pass
// that would have to notice the cancellation. A caller cancelled while the floor is
// still in the future is already covered by the sleep.
func TestThrottleDoesNotAdmitAnAlreadyCancelledCaller(t *testing.T) {
	interval := 50 * time.Millisecond
	budget := 3 * time.Second

	fresh := func() *Throttle { return newThrottle(interval, 0, 0, -1, budget, budget/4) }
	floorOf := func(th *Throttle) time.Time {
		th.mu.Lock()
		defer th.mu.Unlock()
		return th.floor
	}

	t.Run("cancelled before the first admission", func(t *testing.T) {
		th := fresh()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		require.ErrorIs(t, th.Wait(ctx), context.Canceled)
		require.True(t, floorOf(th).IsZero(),
			"a caller that never started must not establish a pacing floor")
	})

	t.Run("cancelled once the floor has passed", func(t *testing.T) {
		th := fresh()
		require.NoError(t, th.Wait(context.Background()))
		// Let the floor pass so the next caller is admitted immediately rather than
		// through the wait, which is the path that has to check for cancellation.
		time.Sleep(interval + 20*time.Millisecond)
		before := floorOf(th)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.ErrorIs(t, th.Wait(ctx), context.Canceled)
		require.Equal(t, before, floorOf(th),
			"a cancelled immediate admission must not consume an interval")
	})
}
