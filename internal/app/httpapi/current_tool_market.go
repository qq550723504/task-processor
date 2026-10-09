package httpapi

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	tmstore "task-processor/internal/integration/persistence/toolmarket"
	"task-processor/internal/integration/toolmarketauth"
	kernelmodule "task-processor/internal/kernel/module"
	tm "task-processor/internal/toolmarket"
	tmhttp "task-processor/internal/toolmarket/httpapi"
)

type ToolMarketDependencies struct {
	DB      *gorm.DB
	Package *tm.PackageConfig
}

func WithToolMarket(d ToolMarketDependencies) CurrentApplicationOption {
	return func(o *currentApplicationOptions) { o.toolMarketConfigs++; o.toolMarket = &d }
}

type toolMarketModule struct{ routes []httproute.Descriptor }

func (toolMarketModule) Name() string                    { return tmhttp.ModuleName }
func (m toolMarketModule) Enabled(c *config.Config) bool { return c != nil && c.Workbench.Enabled }
func (m toolMarketModule) Register(r *kernelmodule.Registry) error {
	r.AddRoutes(m.routes...)
	return nil
}

func buildToolMarket(ctx context.Context, d ToolMarketDependencies, deps routeAuthDependencies, a *authz.ListingKitAuthorizer, readiness tm.Readiness) (kernelmodule.Module, error) {
	if d.DB == nil {
		return nil, tm.ErrUnavailable
	}
	var safe bool
	if err := d.DB.WithContext(ctx).Raw(`SELECT current_user='tool_market_runtime' AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolbypassrls AND NOT pg_has_role(current_user,'tool_market_owner','MEMBER') AND NOT has_schema_privilege(current_user,'tool_market','CREATE') FROM pg_roles WHERE rolname=current_user`).Scan(&safe).Error; err != nil || !safe {
		return nil, errors.New("tool market requires its restricted serving role")
	}
	if err := tmstore.VerifySchema(ctx, d.DB); err != nil {
		return nil, err
	}
	repo, err := tmstore.New(d.DB)
	if err != nil {
		return nil, err
	}
	admission, err := toolmarketauth.New(deps.organizationResolver, a)
	if err != nil {
		return nil, err
	}
	h := &tmhttp.Handler{Repository: repo, Bind: admission.Bind, Authorize: admission.Authorize, Readiness: readiness}
	if d.Package != nil {
		_ = h.ConfigurePackage(*d.Package)
	}
	routes, err := tmhttp.BuildRoutes(h)
	if err != nil {
		return nil, err
	}
	return toolMarketModule{routes: routes}, nil
}
