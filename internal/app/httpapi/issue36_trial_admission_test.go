package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	storeapp "task-processor/internal/app/storecenter"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/listing/record"
	"task-processor/internal/product/review"
)

func trialRouteDescriptors() []httproute.Descriptor {
	routes := productReviewOnlyRoutes(nil, nil)
	for i := range routes {
		routes[i].RequestTimeout = review.Timeout + 2*time.Second
	}
	listing := append(sheinRecordRoutes(&record.Service{}), sheinRecordCollectionRoutes(&record.CollectionService{})...)
	listing = append(listing, sheinDiagnosticRoutes(&record.DiagnosticService{})...)
	for i := range listing {
		listing[i].RequestTimeout = record.Timeout + 2*time.Second
	}
	return append(routes, listing...)
}

func TestIssue36TrialRouteAdmission(t *testing.T) {
	var base []httproute.Descriptor
	for _, route := range currentWorkbenchApplicationRoutes {
		base = append(base, httproute.Descriptor{Method: route.Method, Path: route.Path})
	}
	trial := trialRouteDescriptors()
	require.Len(t, trial, 7)
	all := append(append([]httproute.Descriptor(nil), base...), trial...)
	validate := func(routes []httproute.Descriptor, enabled bool) error {
		return validateCurrentApplicationRoutesInternal(routes, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{LocalTrial: enabled})
	}
	require.NoError(t, validate(base, false))
	require.Error(t, validate(base, true))
	require.Error(t, validate(all, false))
	require.NoError(t, validate(all, true))
	for i := range trial {
		for _, mutate := range []func(*httproute.Descriptor){
			func(d *httproute.Descriptor) { d.AuthPolicy = httproute.AuthPolicyPublic },
			func(d *httproute.Descriptor) { d.Permission = "drift" },
			func(d *httproute.Descriptor) { d.RequestTimeout++ },
			func(d *httproute.Descriptor) { d.Handler = nil },
		} {
			changed := append([]httproute.Descriptor(nil), all...)
			mutate(&changed[len(base)+i])
			require.Error(t, validate(changed, true))
		}
	}
	createProposal := httproute.Descriptor{Method: http.MethodPost, Path: "/api/product/text-proposals"}
	require.Error(t, validate(append(append([]httproute.Descriptor(nil), all...), createProposal), true))
}

func TestIssue36TrialOptionalAssembly(t *testing.T) {
	for _, mode := range []string{"enabled", "missing-store", "nil-trial", "shared-store", "shared-owner", "duplicate", "missing-builder"} {
		t.Run(mode, func(t *testing.T) {
			deps := newRouteAuthDependencies()
			source, owner, stores, trial := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
			calls := 0
			factories := currentApplicationFactories{
				buildWorkbench: func(*config.Config, *logrus.Logger) (workbenchContextBuildResult, error) {
					return workbenchContextBuildResult{module: currentApplicationTestModule{name: "base", routes: currentWorkbenchApplicationRoutes}, authDependencies: &deps}, nil
				},
				buildSourceAccount: func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) { return nil, nil },
				buildCommercial:    func(*gorm.DB, *gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) { return nil, nil },
				buildResourceCharges: func(context.Context, *gorm.DB, *gorm.DB, *gorm.DB, *authz.ListingKitAuthorizer) (*orgresource.ConsumerChargeService, error) {
					return orgresource.NewConsumerChargeService(currentStoreChargeFixture{}, map[orgresource.ResourceConsumer]orgresource.ConsumerChargeOwner{orgresource.ConsumerStoreService: currentStoreChargeFixture{}})
				},
				buildStoreCenter: func(context.Context, *gorm.DB, *authz.ListingKitAuthorizer, orgresource.ConsumerChargePort, *storeapp.OfficialApplicationRegistry) (kernelmodule.Module, error) {
					return currentStoreRouteFixture{routes: currentStoreTestRoutes(t)}, nil
				},
				buildLocalTrial: func(_ context.Context, got *gorm.DB, _ *authz.ListingKitAuthorizer, _ routeAuthDependencies) (kernelmodule.Module, error) {
					calls++
					require.Same(t, trial, got)
					return issue36TrialModule{routes: trialRouteDescriptors()}, nil
				},
			}
			options := []CurrentApplicationOption{WithCommercialOwnerDatabase(owner), WithStoreCenter(stores)}
			switch mode {
			case "missing-store":
				options = options[:1]
			case "nil-trial":
				trial = nil
			case "shared-store":
				trial = stores
			case "shared-owner":
				trial = owner
			case "missing-builder":
				factories.buildLocalTrial = nil
			}
			options = append(options, WithIssue36Trial(trial))
			if mode == "duplicate" {
				options = append(options, WithIssue36Trial(trial))
			}
			server, err := buildCurrentApplication(context.Background(), source, currentApplicationTestConfig(), logrus.New(), factories, options...)
			if mode == "enabled" {
				require.NoError(t, err)
				require.NotNil(t, server)
				require.Equal(t, 1, calls)
			} else {
				require.Error(t, err)
				require.Nil(t, server)
				require.Zero(t, calls)
			}
		})
	}
}
