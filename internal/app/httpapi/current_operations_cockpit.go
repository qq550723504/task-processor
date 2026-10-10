package httpapi

import (
	"context"
	"gorm.io/gorm"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	"task-processor/internal/integration/operationscockpitauth"
	"task-processor/internal/integration/operationscockpitobservations"
	persistence "task-processor/internal/integration/persistence/operationscockpit"
	kernelmodule "task-processor/internal/kernel/module"
	o "task-processor/internal/marketplace/shein/observations"
	c "task-processor/internal/operationscockpit"
	cockpithttp "task-processor/internal/operationscockpit/httpapi"
	"task-processor/internal/storecenter"
	"time"
)

func WithOperationsCockpit() CurrentApplicationOption {
	return func(o *currentApplicationOptions) { o.operationsCockpits++ }
}

type operationsCockpitModule struct{ routes []httproute.Descriptor }

func (operationsCockpitModule) Name() string { return cockpithttp.ModuleName }
func (operationsCockpitModule) Enabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Workbench.Enabled
}
func (m operationsCockpitModule) Register(r *kernelmodule.Registry) error {
	r.AddRoutes(m.routes...)
	return nil
}
func buildOperationsCockpit(ctx context.Context, db *gorm.DB, deps routeAuthDependencies, permissions *authz.ListingKitAuthorizer, capabilities storecenter.RuntimeCapabilities, observations *o.Service) (kernelmodule.Module, error) {
	if !capabilities.OperationsCockpit {
		return nil, c.ErrUnavailable
	}
	if err := storecenter.VerifyCurrentSchema(ctx, db); err != nil {
		return nil, err
	}
	if err := storecenter.VerifyRuntimePermissionsForCapabilities(ctx, db, capabilities); err != nil {
		return nil, err
	}
	boundary, err := operationscockpitauth.New(deps.organizationResolver, permissions, db)
	if err != nil {
		return nil, err
	}
	repo, err := persistence.New(ctx, db, boundary, time.Now)
	if err != nil {
		return nil, err
	}
	h := &cockpithttp.Handler{Repository: repo, Bind: boundary.Bind, Current: boundary.Current, Directory: boundary.ListStores, Revalidate: boundary.ReadStores, Observations: (operationscockpitobservations.Orders{Service: observations}).Read, Now: time.Now, Scope: func(ctx context.Context) (c.Scope, error) {
		id, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
		s := c.Scope{OrganizationID: id.EffectiveOrganizationID, ActorID: id.UserID}
		if !ok || !s.Valid() || id.TenantID != s.OrganizationID || !id.TokenExpiresAt.After(time.Now()) {
			return c.Scope{}, c.ErrForbidden
		}
		return s, nil
	}}
	routes, err := cockpithttp.BuildRoutes(h)
	if err != nil {
		return nil, err
	}
	return operationsCockpitModule{routes: routes}, nil
}
