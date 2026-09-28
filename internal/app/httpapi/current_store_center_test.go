package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	storehttp "task-processor/internal/storecenter/httpapi"
)

func currentStoreTestRoutes(t *testing.T) []httproute.Descriptor {
	t.Helper()
	reg := kernelmodule.NewRegistry()
	require.NoError(t, storehttp.NewModule(&storehttp.Handler{}).Register(reg))
	return reg.Routes()
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
	for _, mode := range []string{"disabled", "enabled", "nil-records", "nil-quota", "shared", "missing-owner", "duplicate", "missing-factory"} {
		t.Run(mode, func(t *testing.T) {
			deps := newRouteAuthDependencies()
			calls := 0
			source, read, owner, records, quota := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
			factories := currentApplicationFactories{
				buildWorkbench: func(*config.Config, *logrus.Logger) (workbenchContextBuildResult, error) {
					return workbenchContextBuildResult{module: currentApplicationTestModule{name: "base", routes: currentWorkbenchApplicationRoutes}, authDependencies: &deps}, nil
				},
				buildSourceAccount: func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) { return nil, nil },
				buildCommercial:    func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) { return nil, nil },
				buildStoreCenter: func(_ context.Context, r, q *gorm.DB) (kernelmodule.Module, error) {
					calls++
					require.Same(t, records, r)
					require.Same(t, quota, q)
					return storehttp.NewModule(&storehttp.Handler{}), nil
				},
			}
			options := []CurrentApplicationOption{WithCommercialOwnerDatabase(owner)}
			switch mode {
			case "nil-records":
				records = nil
			case "nil-quota":
				quota = nil
			case "shared":
				quota = records
			case "missing-owner":
				options = nil
			case "missing-factory":
				factories.buildStoreCenter = nil
			}
			if mode != "disabled" {
				options = append(options, WithStoreCenter(records, quota))
			}
			if mode == "duplicate" {
				options = append(options, WithStoreCenter(records, quota))
			}
			server, err := buildCurrentApplication(context.Background(), source, read, currentApplicationTestConfig(), logrus.New(), factories, options...)
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
