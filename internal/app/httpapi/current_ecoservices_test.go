package httpapi

import (
	"github.com/stretchr/testify/require"
	ehttp "task-processor/internal/ecoservices/httpapi"
	"task-processor/internal/httproute"
	"testing"
)

func TestEcoservicesRouteAdmissionKeepsPlatformAndOrganizationBoundaries(t *testing.T) {
	var base []httproute.Descriptor
	for _, r := range currentWorkbenchApplicationRoutes {
		base = append(base, httproute.Descriptor{Method: r.Method, Path: r.Path})
	}
	routes := ehttp.Routes(nil)
	all := append(append([]httproute.Descriptor{}, base...), routes...)
	validate := func(rs []httproute.Descriptor, on bool) error {
		return validateCurrentApplicationRoutesInternal(rs, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{Ecoservices: on})
	}
	require.NoError(t, validate(base, false))
	require.Error(t, validate(all, false))
	require.Error(t, validate(base, true))
	require.NoError(t, validate(all, true))
	for i := range routes {
		for _, change := range []func(*httproute.Descriptor){func(d *httproute.Descriptor) { d.Permission = "wrong.permission" }, func(d *httproute.Descriptor) { d.AuthPolicy = httproute.AuthPolicyVerifiedIdentity }, func(d *httproute.Descriptor) {
			d.OrganizationAccessPolicy = httproute.OrganizationAccessPolicyContextRead
		}, func(d *httproute.Descriptor) { d.RequestTimeout = 0 }} {
			changed := append([]httproute.Descriptor{}, all...)
			change(&changed[len(base)+i])
			require.Error(t, validate(changed, true))
		}
	}
}
func TestCommercialFirstCompositionCannotSkipEcoservicesAdmission(t *testing.T) {
	var routes []httproute.Descriptor
	for _, r := range currentWorkbenchApplicationRoutes {
		routes = append(routes, httproute.Descriptor{Method: r.Method, Path: r.Path})
	}
	routes = append(routes, billingDescriptorsForTest()...)
	start := len(routes)
	routes = append(routes, ehttp.Routes(nil)...)
	validate := func(rs []httproute.Descriptor) error {
		return validateCurrentApplicationRoutesInternal(rs, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{Ecoservices: true})
	}
	require.NoError(t, validate(routes))
	for i := start; i < len(routes); i++ {
		changed := append([]httproute.Descriptor{}, routes...)
		changed[i].AuthPolicy = httproute.AuthPolicyVerifiedIdentity
		require.Error(t, validate(changed), "commercial-first composition skipped %s", changed[i].Path)
	}
}
