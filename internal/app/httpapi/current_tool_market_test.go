package httpapi

import (
	"github.com/stretchr/testify/require"
	"task-processor/internal/httproute"
	tmhttp "task-processor/internal/toolmarket/httpapi"
	"testing"
)

func TestToolMarketCurrentRouteAdmissionPreservesBothIdentityBoundaries(t *testing.T) {
	var base []httproute.Descriptor
	for _, r := range currentWorkbenchApplicationRoutes {
		base = append(base, httproute.Descriptor{Method: r.Method, Path: r.Path})
	}
	routes := tmhttp.Routes(nil)
	all := append(append([]httproute.Descriptor{}, base...), routes...)
	validate := func(r []httproute.Descriptor, on bool) error {
		return validateCurrentApplicationRoutesInternal(r, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{ToolMarket: on})
	}
	require.NoError(t, validate(all, true))
	require.Error(t, validate(all, false))
	require.Error(t, validate(base, true))
	for i := range routes {
		for _, mutate := range []func(*httproute.Descriptor){
			func(r *httproute.Descriptor) { r.AuthPolicy = httproute.AuthPolicyPublic },
			func(r *httproute.Descriptor) {
				r.OrganizationAccessPolicy = httproute.OrganizationAccessPolicyContextRead
			},
			func(r *httproute.Descriptor) { r.Permission = "product_sourcing.write" },
			func(r *httproute.Descriptor) { r.RejectUnreadRequestBody = !r.RejectUnreadRequestBody },
		} {
			changed := append([]httproute.Descriptor{}, all...)
			mutate(&changed[len(base)+i])
			require.Error(t, validate(changed, true))
		}
	}
}
