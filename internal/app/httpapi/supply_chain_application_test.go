package httpapi

import (
	"github.com/stretchr/testify/require"
	supplyapp "task-processor/internal/app/supplychain"
	"task-processor/internal/httproute"
	"testing"
)

func TestSupplyRouteAdmissionRetainsExactPrivatePermissions(t *testing.T) {
	var routes []httproute.Descriptor
	for _, r := range currentWorkbenchApplicationRoutes {
		routes = append(routes, httproute.Descriptor{Method: r.Method, Path: r.Path})
	}
	routes = append(routes, supplyapp.SupplyRoutes(nil, nil)...)
	check := func(r []httproute.Descriptor, enabled bool) error {
		return validateCurrentApplicationRoutesInternal(r, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{SupplyChain: enabled})
	}
	require.NoError(t, check(routes, true))
	require.Error(t, check(routes, false))
	for i := len(currentWorkbenchApplicationRoutes); i < len(routes); i++ {
		for _, mutate := range []func(*httproute.Descriptor){func(r *httproute.Descriptor) { r.AuthPolicy = httproute.AuthPolicyPublic }, func(r *httproute.Descriptor) {
			r.OrganizationAccessPolicy = httproute.OrganizationAccessPolicyCachedRead
		}, func(r *httproute.Descriptor) { r.Permission = "workbench.collection.read" }, func(r *httproute.Descriptor) { r.RequestTimeout = 0 }} {
			changed := append([]httproute.Descriptor(nil), routes...)
			mutate(&changed[i])
			require.Error(t, check(changed, true))
		}
	}
}
