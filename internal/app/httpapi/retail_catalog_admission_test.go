package httpapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	billinghttp "task-processor/internal/commercial/billing/httpapi"
	"task-processor/internal/httproute"
)

func billingDescriptorsForTest() []httproute.Descriptor {
	routes, err := billinghttp.Routes(billinghttp.NewHandler(nil))
	if err != nil {
		panic(err)
	}
	return routes
}

func TestCurrentApplicationRejectsPublicPriceBoundaryDrift(t *testing.T) {
	var routes []httproute.Descriptor
	for _, r := range currentWorkbenchApplicationRoutes {
		routes = append(routes, httproute.Descriptor{Method: r.Method, Path: r.Path})
	}
	routes = append(routes, billingDescriptorsForTest()...)
	require.NoError(t, validateCurrentApplicationRoutesWithBrowser(routes, false, false, false, false, false))
	index := -1
	for i, r := range routes {
		if r.Path == billinghttp.PublicResourceOfferPath {
			index = i
		}
	}
	require.NotEqual(t, -1, index)
	for _, mutate := range []func(*httproute.Descriptor){
		func(r *httproute.Descriptor) { r.AuthPolicy = httproute.AuthPolicyCurrentIdentity },
		func(r *httproute.Descriptor) { r.Permission = "private.permission" },
		func(r *httproute.Descriptor) {
			r.OrganizationAccessPolicy = httproute.OrganizationAccessPolicyLiveWrite
		},
		func(r *httproute.Descriptor) {
			r.OrganizationTargetResolver = func(*http.Request) (string, error) { return "private-org", nil }
		},
		func(r *httproute.Descriptor) { r.RequestTimeout = time.Minute },
		func(r *httproute.Descriptor) { r.RejectUnreadRequestBody = false },
		func(r *httproute.Descriptor) { r.Module = "wrong-owner" },
		func(r *httproute.Descriptor) { r.Handler = nil },
	} {
		changed := append([]httproute.Descriptor(nil), routes...)
		mutate(&changed[index])
		require.Error(t, validateCurrentApplicationRoutesWithBrowser(changed, false, false, false, false, false))
	}
}
