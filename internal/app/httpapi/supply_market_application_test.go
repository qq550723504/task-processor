package httpapi

import (
	"github.com/stretchr/testify/require"
	podhttp "task-processor/internal/app/pod/httpapi"
	"task-processor/internal/httproute"
	markethttp "task-processor/internal/product/supplymarket/httpapi"
	"testing"
)

func TestCurrentMarketPODRouteAdmission(t *testing.T) {
	var base []httproute.Descriptor
	for _, r := range currentWorkbenchApplicationRoutes {
		base = append(base, httproute.Descriptor{Method: r.Method, Path: r.Path})
	}
	feature := append(markethttp.Routes(nil, nil, nil), podhttp.Routes(nil, nil)...)
	routes := append(base, feature...)
	check := func(rs []httproute.Descriptor, f currentApplicationOptionalRoutes) error {
		return validateCurrentApplicationRoutesInternal(rs, false, false, false, false, false, false, false, f)
	}
	flags := currentApplicationOptionalRoutes{SupplyMarket: true, POD: true}
	require.NoError(t, check(routes, flags))
	require.Error(t, check(routes, currentApplicationOptionalRoutes{}))
	for i := range feature {
		mutated := append([]httproute.Descriptor{}, routes...)
		mutated[len(base)+i].AuthPolicy = httproute.AuthPolicyPublic
		require.Error(t, check(mutated, flags))
		mutated[len(base)+i] = feature[i]
		mutated[len(base)+i].Permission = "workbench.collection.read"
		require.Error(t, check(mutated, flags))
	}
	require.Error(t, check(routes[:len(routes)-1], flags))
}
