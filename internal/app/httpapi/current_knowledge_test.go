package httpapi

import (
	"github.com/stretchr/testify/require"
	"task-processor/internal/httproute"
	knowledgehttp "task-processor/internal/knowledge/httpapi"
	"testing"
)

func TestCurrentKnowledgeRouteAdmissionRequiresLiveReadAndExactSurface(t *testing.T) {
	var base []httproute.Descriptor
	for _, route := range currentWorkbenchApplicationRoutes {
		base = append(base, httproute.Descriptor{Method: route.Method, Path: route.Path})
	}
	routes := knowledgehttp.Routes(&knowledgehttp.Handler{})
	all := append(base, routes...)
	validate := func(routes []httproute.Descriptor, enabled bool) error {
		return validateCurrentApplicationRoutesInternal(routes, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{Knowledge: enabled})
	}
	require.Len(t, routes, 11)
	require.NoError(t, validate(base, false))
	require.Error(t, validate(all, false))
	require.Error(t, validate(base, true))
	require.NoError(t, validate(all, true))
	for i := range routes {
		for _, change := range []func(*httproute.Descriptor){
			func(d *httproute.Descriptor) {
				d.OrganizationAccessPolicy = httproute.OrganizationAccessPolicyCachedRead
			},
			func(d *httproute.Descriptor) { d.Permission = "" },
			func(d *httproute.Descriptor) { d.AuthPolicy = httproute.AuthPolicyVerifiedIdentity },
			func(d *httproute.Descriptor) { d.RequestTimeout = 0 },
		} {
			changed := append([]httproute.Descriptor(nil), all...)
			change(&changed[len(base)+i])
			require.Error(t, validate(changed, true))
		}
	}
}
