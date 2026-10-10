package httpapi

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"net/http"
	observationruntime "task-processor/internal/app/runtime/storeobservations"
	storeapp "task-processor/internal/app/storecenter"
	observationapp "task-processor/internal/app/storeobservations"
	observationhttp "task-processor/internal/app/storeobservations/httpapi"
	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	observationstore "task-processor/internal/integration/persistence/sheinobservations"
	kernelmodule "task-processor/internal/kernel/module"
	o "task-processor/internal/marketplace/shein/observations"
	"task-processor/internal/storecenter"
	"task-processor/internal/workbenchcontext"
	"time"
)

type StoreObservationsDependencies struct {
	Starter   o.Starter
	NewWorker func(*o.Service) (observationruntime.Worker, error)
	Lifecycle *observationruntime.Lifecycle
}

func WithStoreObservations(d StoreObservationsDependencies) CurrentApplicationOption {
	return func(options *currentApplicationOptions) {
		options.storeObservations++
		options.storeObservationDependencies = &d
	}
}

type storeObservationModule struct {
	application *observationapp.Application
	routes      []httproute.Descriptor
}

func (storeObservationModule) Name() string                  { return "store-observations" }
func (storeObservationModule) Enabled(c *config.Config) bool { return c != nil && c.Workbench.Enabled }
func (m storeObservationModule) Register(r *kernelmodule.Registry) error {
	r.AddRoutes(m.routes...)
	return nil
}

func buildCurrentStoreObservations(ctx context.Context, db *gorm.DB, d StoreObservationsDependencies, deps routeAuthDependencies, permissions *authz.ListingKitAuthorizer, applications *storeapp.OfficialApplicationRegistry, cfg *config.Config, capabilities storecenter.RuntimeCapabilities) (storeObservationModule, error) {
	var empty storeObservationModule
	resolver, ok := deps.organizationResolver.(*workbenchcontext.Resolver)
	if !ok || resolver == nil || permissions == nil || applications == nil || db == nil || d.Starter == nil || d.NewWorker == nil || d.Lifecycle == nil || cfg == nil || cfg.ListingKit.Zitadel.TenantDirectoryToken == "" {
		return empty, o.ErrUnavailable
	}
	if err := storecenter.VerifyCurrentSchema(ctx, db); err != nil {
		return empty, err
	}
	if !capabilities.Observations {
		return empty, o.ErrUnavailable
	}
	if err := storecenter.VerifyRuntimePermissionsForCapabilities(ctx, db, capabilities); err != nil {
		return empty, err
	}
	repo, err := observationstore.NewRepository(ctx, db)
	if err != nil {
		return empty, err
	}
	stores, err := storecenter.NewMemberScopedStoreRepository(db, currentStoreMemberAuthorizer{authorizer: permissions})
	if err != nil {
		return empty, err
	}
	authorization := observationapp.Authorization{Client: zitadel.NewAuthorizationClient(cfg.ListingKit.Zitadel.AuthorizationAPIURL, &http.Client{Timeout: 5 * time.Second}), ServiceToken: func(context.Context) (string, error) { return cfg.ListingKit.Zitadel.TenantDirectoryToken, nil }, ProjectID: cfg.ListingKit.Zitadel.ProjectID, Permissions: permissions, OrganizationStatus: resolver.BusinessStatusChecker()}
	official, err := storeapp.NewOfficialObservationAccess(stores, authorization, applications)
	if err != nil {
		return empty, err
	}
	service := &o.Service{Repository: repo, Access: observationapp.Access{Authorization: authorization, Official: official}, Directory: observationapp.Directory{Stores: stores}, Starter: d.Starter}
	currentWorker, err := d.NewWorker(service)
	if err != nil || currentWorker == nil {
		return empty, o.ErrUnavailable
	}
	d.Lifecycle.Worker = currentWorker
	application := &observationapp.Application{Service: service, Ready: d.Lifecycle.Ready}
	binder := productReviewCapabilityBinder{now: time.Now}
	return storeObservationModule{application: application, routes: observationhttp.Routes(application, binder.Bind)}, nil
}

func validateCurrentObservationDescriptor(d httproute.Descriptor) error {
	for _, expected := range observationhttp.Routes(nil, nil) {
		if d.Method == expected.Method && d.Path == expected.Path {
			if d.Module == expected.Module && d.Permission == expected.Permission && d.AuthPolicy == expected.AuthPolicy && d.OrganizationAccessPolicy == expected.OrganizationAccessPolicy && d.OrganizationTargetResolver == nil && d.RequestTimeout == expected.RequestTimeout && d.RejectUnreadRequestBody == expected.RejectUnreadRequestBody && d.Handler != nil {
				return nil
			}
			return errors.New("Store observation route loses original member, live permission or bounded read boundary")
		}
	}
	return errors.New("Store observation route not admitted")
}
