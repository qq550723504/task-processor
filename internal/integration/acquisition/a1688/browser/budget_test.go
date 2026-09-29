package browser

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/product/sourcing"
)

// The provider budget must stay inside the current-application acquisition route
// budget. D1 persists an operation only after acquisition returns, so a provider
// that outlives the route hands the user a deadline with nothing to replay.
func TestProviderBudgetFitsInsideTheAcquisitionRouteBudget(t *testing.T) {
	require.Less(t,
		DefaultTimeout,
		sourcing.AcquisitionTimeout,
		"provider budget (%s) must be below the acquisition route budget (%s); "+
			"raising it requires raising the route budget and both frontend deadlines together",
		DefaultTimeout, sourcing.AcquisitionTimeout)

	// The route timer starts before the body is read, and D1 persists only after
	// acquisition returns, so the provider must leave room for body read plus
	// mapping and publication inside the same route budget. Half the route is a
	// deliberately conservative ceiling for that remainder.
	require.LessOrEqual(t, DefaultTimeout, sourcing.AcquisitionTimeout/2,
		"provider budget (%s) must leave at least half the route budget (%s) for body read, mapping and publication",
		DefaultTimeout, sourcing.AcquisitionTimeout)
}

// A zero budget must resolve to the default rather than to an unbounded one.
func TestZeroBudgetFallsBackToDefault(t *testing.T) {
	require.Equal(t, DefaultTimeout, Options{}.budget())
	require.Equal(t, 45*time.Second, Options{Budget: 45 * time.Second}.budget())
}

// A navigation timeout above the acquisition budget would let navigation alone
// consume the whole budget, so it is capped by the budget when unset.
func TestNavigationTimeoutDefaultsWithinBudget(t *testing.T) {
	opts := Options{Budget: 12 * time.Second}
	require.LessOrEqual(t, opts.navigationTimeout(), opts.budget())
}

// Every acquisition launches its own Chromium, so a burst across organizations
// must be bounded on the collector rather than only per organization.
func TestConcurrentAcquisitionsAreBounded(t *testing.T) {
	require.Equal(t, 2, Options{}.maxConcurrent(), "the default collector cap must follow design D8 / 12-B2")
	require.Equal(t, 5, Options{MaxConcurrent: 5}.maxConcurrent())
	require.Equal(t, DefaultMaxConcurrent, newTestClient(Options{}).maxConcurrentInternal())
}

// The concurrency slot must be taken before any browser work, so an over-limit
// request is rejected instead of launching Chromium.
func TestConcurrencySlotRejectsWhenFull(t *testing.T) {
	browserPath := fixtureBrowserPath(t)
	client := newTestClient(Options{ExecutablePath: browserPath, MaxConcurrent: 1})
	client.slots <- struct{}{} // occupy the only slot
	_, err := client.Acquire(context.Background(), mustCanonicalSource(t))
	require.ErrorIs(t, err, ErrCapacity)
	<-client.slots // release
}

func mustCanonicalSource(t *testing.T) sourcing.AcquisitionSource {
	t.Helper()
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	return source
}
