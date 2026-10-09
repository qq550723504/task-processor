package httpapi

import (
	"github.com/stretchr/testify/require"
	obshttp "task-processor/internal/app/storeobservations/httpapi"
	"task-processor/internal/httproute"
	"testing"
)

func TestCurrentObservationRouteAdmissionRejectsDrift(t *testing.T) {
	var base []httproute.Descriptor
	for _, route := range currentWorkbenchApplicationRoutes {
		base = append(base, httproute.Descriptor{Method: route.Method, Path: route.Path})
	}
	stores := currentStoreTestRoutes(t)
	observations := obshttp.Routes(nil, nil)
	all := append(append(append([]httproute.Descriptor{}, base...), stores...), observations...)
	validate := func(routes []httproute.Descriptor, enabled bool) error {
		return validateCurrentApplicationRoutesInternal(routes, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{StoreCenter: true, StoreObservations: enabled})
	}
	require.Error(t, validate(all, false))
	require.Error(t, validate(append(base, stores...), true))
	require.NoError(t, validate(all, true))
	for i := range observations {
		for _, mutate := range []func(*httproute.Descriptor){
			func(d *httproute.Descriptor) { d.Permission = "workbench.supply.read" },
			func(d *httproute.Descriptor) { d.AuthPolicy = httproute.AuthPolicyVerifiedIdentity },
			func(d *httproute.Descriptor) {
				d.OrganizationAccessPolicy = httproute.OrganizationAccessPolicyCachedRead
			},
			func(d *httproute.Descriptor) { d.RequestTimeout = 0 },
			func(d *httproute.Descriptor) { d.Handler = nil },
			func(d *httproute.Descriptor) { d.RejectUnreadRequestBody = !d.RejectUnreadRequestBody },
		} {
			changed := append([]httproute.Descriptor{}, all...)
			mutate(&changed[len(base)+len(stores)+i])
			require.Error(t, validate(changed, true))
		}
	}
}
