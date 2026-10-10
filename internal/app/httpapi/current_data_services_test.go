package httpapi

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	datahttp "task-processor/internal/dataservice/httpapi"
	"task-processor/internal/httproute"
	"task-processor/internal/storecenter"
	"task-processor/internal/workbenchcontext"
	"testing"
	"time"
)

func TestDataServicesRejectsMissingOrganizationSuspensionOwner(t *testing.T) {
	a, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	resolver := workbenchcontext.NewResolver(nil, "project", "contract", nil)
	_, _, err = buildDataServices(context.Background(), currentApplicationOptions{dataServices: &DataServicesDependencies{}}, routeAuthDependencies{organizationResolver: resolver}, a, &config.Config{}, storecenter.RuntimeCapabilities{})
	require.ErrorContains(t, err, "organization suspension authority")
}

func TestDataServicesNativeAdmissionPreservesAllThreeAuthBoundaries(t *testing.T) {
	var base []httproute.Descriptor
	for _, r := range currentWorkbenchApplicationRoutes {
		base = append(base, httproute.Descriptor{Method: r.Method, Path: r.Path})
	}
	routes := datahttp.BuildRoutes(datahttp.Dependencies{})
	all := append(append([]httproute.Descriptor{}, base...), routes...)
	validate := func(r []httproute.Descriptor, on bool) error {
		return validateCurrentApplicationRoutesInternal(r, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{DataServices: on})
	}
	require.NoError(t, validate(all, true))
	require.Error(t, validate(all, false))
	require.Error(t, validate(base, true))
	for i := range routes {
		for _, mutate := range []func(*httproute.Descriptor){
			func(r *httproute.Descriptor) { r.AuthPolicy = httproute.AuthPolicyCurrentIdentity },
			func(r *httproute.Descriptor) {
				r.OrganizationAccessPolicy = httproute.OrganizationAccessPolicyContextRead
			},
			func(r *httproute.Descriptor) { r.Permission = "workbench.chat.read" },
			func(r *httproute.Descriptor) { r.Module = "ai-workbench" },
			func(r *httproute.Descriptor) { r.RequestTimeout = time.Minute },
			func(r *httproute.Descriptor) { r.RejectUnreadRequestBody = !r.RejectUnreadRequestBody },
		} {
			changed := append([]httproute.Descriptor{}, all...)
			mutate(&changed[len(base)+i])
			require.Error(t, validate(changed, true), routes[i].Method+" "+routes[i].Path)
		}
	}
}
