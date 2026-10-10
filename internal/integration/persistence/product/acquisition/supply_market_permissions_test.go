package acquisition

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMarketPODRuntimeGrantsAreExplicit(t *testing.T) {
	market := runtimePermissionsFor(RuntimeCapabilities{Collections: true, SupplyMarket: true})
	require.Contains(t, market, "('supply_market_records','UPDATE')")
	require.NotContains(t, market, "('product_pod_fences','DELETE')")
	pod := runtimePermissionsFor(RuntimeCapabilities{Collections: true, SupplyMarket: true, POD: true})
	require.Contains(t, pod, "('product_pod_fences','DELETE')")
	require.Contains(t, pod, "('listing_submission_execution_attempts','UPDATE')")
	require.NotContains(t, pod, "('product_collection_items','DELETE')")
}
