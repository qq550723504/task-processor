package httpapi

import (
	"github.com/stretchr/testify/require"
	"task-processor/internal/httproute"
	cockpit "task-processor/internal/operationscockpit/httpapi"
	"testing"
)

func TestCockpitRouteAdmissionRequiresEnabledStoreAndExactDescriptor(t *testing.T) {
	base := []httproute.Descriptor{}
	for _, r := range currentWorkbenchApplicationRoutes {
		base = append(base, httproute.Descriptor{Method: r.Method, Path: r.Path})
	}
	stores := currentStoreTestRoutes(t)
	routes := cockpit.Routes(nil)
	all := append(append(append([]httproute.Descriptor{}, base...), stores...), routes...)
	validate := func(rs []httproute.Descriptor, enabled, store bool) error {
		return validateCurrentApplicationRoutesInternal(rs, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{StoreCenter: store, OperationsCockpit: enabled})
	}
	require.Error(t, validate(all, false, true))
	require.Error(t, validate(all, true, false))
	require.Error(t, validate(append(base, stores...), true, true))
	require.NoError(t, validate(all, true, true))
	for i := range routes {
		for _, mutate := range []func(*httproute.Descriptor){func(d *httproute.Descriptor) {
			d.OrganizationAccessPolicy = httproute.OrganizationAccessPolicyCachedRead
		}, func(d *httproute.Descriptor) { d.Permission = "unadmitted.permission" }, func(d *httproute.Descriptor) { d.RequestTimeout = 0 }, func(d *httproute.Descriptor) { d.RejectUnreadRequestBody = !d.RejectUnreadRequestBody }, func(d *httproute.Descriptor) { d.AuthPolicy = httproute.AuthPolicyVerifiedIdentity }} {
			changed := append([]httproute.Descriptor{}, all...)
			mutate(&changed[len(base)+len(stores)+i])
			require.Error(t, validate(changed, true, true))
		}
	}
}
