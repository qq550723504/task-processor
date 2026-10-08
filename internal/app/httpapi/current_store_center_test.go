package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	storeapp "task-processor/internal/app/storecenter"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/ledger/orgresource"
	storehttp "task-processor/internal/storecenter/httpapi"
)

func currentStoreTestRoutes(t *testing.T) []httproute.Descriptor {
	t.Helper()
	var result []httproute.Descriptor
	for _, r := range currentStoreCenterRoutes {
		result = append(result, httproute.Descriptor{Method: r.method, Path: r.path, Module: storehttp.ModuleName, Permission: r.permission, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, Handler: func(*gin.Context) {}})
	}
	return result
}

func TestCurrentStoreRouteAdmissionRejectsDrift(t *testing.T) {
	var base []httproute.Descriptor
	for _, r := range currentWorkbenchApplicationRoutes {
		base = append(base, httproute.Descriptor{Method: r.Method, Path: r.Path})
	}
	stores := currentStoreTestRoutes(t)
	all := append(base, stores...)
	validate := func(routes []httproute.Descriptor, enabled bool) error {
		return validateCurrentApplicationRoutesInternal(routes, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{StoreCenter: enabled})
	}
	require.NoError(t, validate(base, false))
	require.Error(t, validate(base, true))
	require.Error(t, validate(all, false))
	require.NoError(t, validate(all, true))
	for i := range stores {
		for _, mutate := range []func(*httproute.Descriptor){
			func(d *httproute.Descriptor) { d.AuthPolicy = httproute.AuthPolicyVerifiedIdentity },
			func(d *httproute.Descriptor) {
				d.OrganizationAccessPolicy = httproute.OrganizationAccessPolicyCachedRead
			},
			func(d *httproute.Descriptor) { d.Permission = authz.PermissionWorkbenchStoreRead + "-drift" },
			func(d *httproute.Descriptor) { d.Handler = nil },
		} {
			changed := append([]httproute.Descriptor(nil), all...)
			mutate(&changed[len(base)+i])
			require.Error(t, validate(changed, true))
		}
	}
	for _, action := range []string{"activate", "renew", "reactivate"} {
		require.Error(t, validate(append(append([]httproute.Descriptor(nil), all...), httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/workbench/stores/:store_id/" + action, Module: storehttp.ModuleName, Handler: func(*gin.Context) {}}), true))
	}
}

func TestCurrentStoreOptionalAssembly(t *testing.T) {
	for _, mode := range []string{"disabled", "enabled", "nil-records", "shared", "missing-owner", "duplicate", "missing-factory"} {
		t.Run(mode, func(t *testing.T) {
			deps := newRouteAuthDependencies()
			calls := 0
			source, owner, records := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
			factories := currentApplicationFactories{
				buildWorkbench: func(*config.Config, *logrus.Logger) (workbenchContextBuildResult, error) {
					return workbenchContextBuildResult{module: currentApplicationTestModule{name: "base", routes: currentWorkbenchApplicationRoutes}, authDependencies: &deps}, nil
				},
				buildSourceAccount: func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) { return nil, nil },
				buildCommercial:    func(*gorm.DB, *gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) { return nil, nil },
				buildResourceCharges: func(context.Context, *gorm.DB, *gorm.DB, *gorm.DB, *authz.ListingKitAuthorizer) (*orgresource.ConsumerChargeService, error) {
					return orgresource.NewConsumerChargeService(currentStoreChargeFixture{}, map[orgresource.ResourceConsumer]orgresource.ConsumerChargeOwner{orgresource.ConsumerStoreService: currentStoreChargeFixture{}})
				},
				buildStoreCenter: func(_ context.Context, r *gorm.DB, _ *authz.ListingKitAuthorizer, charges orgresource.ConsumerChargePort, _ *storeapp.OfficialApplicationRegistry) (kernelmodule.Module, error) {
					calls++
					require.Same(t, records, r)
					require.NotNil(t, charges)
					return currentStoreRouteFixture{routes: currentStoreTestRoutes(t)}, nil
				},
			}
			options := []CurrentApplicationOption{WithCommercialOwnerDatabase(owner)}
			switch mode {
			case "nil-records":
				records = nil
			case "shared":
				records = source
			case "missing-owner":
				options = nil
			case "missing-factory":
				factories.buildStoreCenter = nil
			}
			if mode != "disabled" {
				options = append(options, WithStoreCenter(records))
			}
			if mode == "duplicate" {
				options = append(options, WithStoreCenter(records))
			}
			server, err := buildCurrentApplication(context.Background(), source, currentApplicationTestConfig(), logrus.New(), factories, options...)
			if mode == "enabled" || mode == "disabled" {
				require.NoError(t, err)
				require.NotNil(t, server)
			} else {
				require.Error(t, err)
				require.Nil(t, server)
			}
			if mode == "enabled" {
				require.Equal(t, 1, calls)
			} else {
				require.Zero(t, calls)
			}
		})
	}
}

type currentStoreRouteFixture struct{ routes []httproute.Descriptor }

func (currentStoreRouteFixture) Name() string                { return storehttp.ModuleName }
func (currentStoreRouteFixture) Enabled(*config.Config) bool { return true }
func (m currentStoreRouteFixture) Register(reg *kernelmodule.Registry) error {
	reg.AddRoutes(m.routes...)
	return nil
}

type currentStoreChargeFixture struct{}

func (currentStoreChargeFixture) Reserve(context.Context, orgresource.ConsumerChargeIntent) (orgresource.ConsumerChargeReceipt, error) {
	return orgresource.ConsumerChargeReceipt{}, orgresource.ErrInvalidInput
}
func (currentStoreChargeFixture) Read(context.Context, orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeReceipt, error) {
	return orgresource.ConsumerChargeReceipt{}, orgresource.ErrInvalidInput
}
func (currentStoreChargeFixture) Settle(context.Context, orgresource.ConsumerChargeReceipt, orgresource.ConsumerChargeProof) (orgresource.ConsumerChargeReceipt, error) {
	return orgresource.ConsumerChargeReceipt{}, orgresource.ErrInvalidInput
}
func (currentStoreChargeFixture) ClaimDue(context.Context, []orgresource.ResourceConsumer) ([]orgresource.ConsumerChargeIdentity, error) {
	return nil, nil
}
func (currentStoreChargeFixture) ReadChargeIntent(context.Context, orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeIntent, error) {
	return orgresource.ConsumerChargeIntent{}, orgresource.ErrInvalidInput
}
func (currentStoreChargeFixture) ReadChargeProof(context.Context, orgresource.ConsumerChargeReceipt) (orgresource.ConsumerChargeProof, error) {
	return orgresource.ConsumerChargeProof{}, orgresource.ErrInvalidInput
}
