package httpapi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	browser "task-processor/internal/integration/acquisition/a1688/browser"
	"task-processor/internal/product/sourcing"
)

// The acquisition route budget is a single deadline that starts before the body
// is read, while the operation is persisted only after acquisition returns. Body
// read, acquisition and publication must therefore all fit inside it, and the
// BFF (22s) and browser client (25s) must stay outside it.
func TestAcquisitionRouteBudgetIsPartitioned(t *testing.T) {
	// Body read plus acquisition must leave real time for mapping and
	// publication inside the same route deadline.
	require.LessOrEqual(t,
		acquisitionBodyReadTimeout+browser.DefaultTimeout,
		sourcing.AcquisitionTimeout,
		"body read (%s) plus acquisition (%s) must fit inside the route budget (%s) with publication headroom",
		acquisitionBodyReadTimeout, browser.DefaultTimeout, sourcing.AcquisitionTimeout)

	// Leave an explicit publication allowance rather than exactly consuming the
	// budget, otherwise mapping and publication have nothing left.
	require.Greater(t,
		sourcing.AcquisitionTimeout-(acquisitionBodyReadTimeout+browser.DefaultTimeout),
		time.Duration(0),
		"the route budget must retain publication headroom after body read and acquisition")
}
