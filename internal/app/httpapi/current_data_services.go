package httpapi

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"net/http"
	dataservicesapp "task-processor/internal/app/dataservices"
	dataservicesruntime "task-processor/internal/app/runtime/dataservices"
	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	datahttp "task-processor/internal/dataservice/httpapi"
	"task-processor/internal/httproute"
	"task-processor/internal/integration/dataserviceauth"
	acquisitionstore "task-processor/internal/integration/persistence/product/acquisition"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/product/dataacquisition"
	"task-processor/internal/storecenter"
	"task-processor/internal/workbenchcontext"
	"time"
)

type DataServicesDependencies struct {
	ProductDB         *gorm.DB
	Provider          dataservicesapp.RuntimeProvider
	Starter           dataacquisition.ExecutionStarter
	NewWorker         func(dataservicesruntime.JobRunner) (dataservicesruntime.Worker, error)
	Worker            *dataservicesruntime.Worker
	TrustedProxyCIDRs []string
}

func WithDataServices(d DataServicesDependencies) CurrentApplicationOption {
	return func(o *currentApplicationOptions) { o.dataServicesConfigs++; o.dataServices = &d }
}

func validateDataServicesDependencies(o currentApplicationOptions, source *gorm.DB, cfg *config.Config) error {
	if o.dataServicesConfigs == 0 {
		return nil
	}
	d := o.dataServices
	if o.dataServicesConfigs != 1 || d == nil || d.ProductDB == nil || d.Provider == nil || d.Starter == nil || d.NewWorker == nil || d.Worker == nil || o.productAcquisitionDB == nil || o.productCollections != 1 || o.commercialOwnerDB == nil || cfg.ListingKit.Zitadel.TenantDirectoryToken == "" {
		return errors.New("data services require Product, collections, Resource, exact IAM, provider and worker lifecycle")
	}
	// Identity of the physical database is also verified against the live pool.
	for _, other := range []*gorm.DB{source, o.productAcquisitionDB, o.commercialOwnerDB, o.moneyOwnerDB} {
		if d.ProductDB == other {
			return errors.New("data services require their independently bounded serving pool")
		}
	}
	return datahttp.ValidateProxyCIDRs(d.TrustedProxyCIDRs)
}

type dataServicesModule struct{ routes []httproute.Descriptor }

func (dataServicesModule) Name() string                  { return "data-services" }
func (dataServicesModule) Enabled(c *config.Config) bool { return c != nil && c.Workbench.Enabled }
func (m dataServicesModule) Register(r *kernelmodule.Registry) error {
	r.AddRoutes(m.routes...)
	return nil
}

func buildDataServices(ctx context.Context, o currentApplicationOptions, deps routeAuthDependencies, a *authz.ListingKitAuthorizer, cfg *config.Config, capabilities storecenter.RuntimeCapabilities) (dataServicesModule, *orgresource.ConsumerChargeService, error) {
	d := o.dataServices
	resolver, ok := deps.organizationResolver.(*workbenchcontext.Resolver)
	if !ok || resolver == nil || a == nil {
		return dataServicesModule{}, nil, errors.New("data services require formal organization authority")
	}
	if err := acquisitionstore.VerifyDataServicesRuntimePermissions(ctx, d.ProductDB); err != nil {
		return dataServicesModule{}, nil, err
	}
	if err := verifySameProductDatabase(ctx, o.productAcquisitionDB, d.ProductDB); err != nil {
		return dataServicesModule{}, nil, err
	}
	httpClient := &http.Client{Timeout: 5 * time.Second}
	users, err := dataserviceauth.NewActiveUserClient(cfg.ListingKit.Zitadel.AuthorizationAPIURL, httpClient)
	if err != nil {
		return dataServicesModule{}, nil, err
	}
	authority, err := dataserviceauth.NewAuthorizer(zitadel.NewAuthorizationClient(cfg.ListingKit.Zitadel.AuthorizationAPIURL, httpClient), users, func(context.Context) (string, error) { return cfg.ListingKit.Zitadel.TenantDirectoryToken, nil }, cfg.ListingKit.Zitadel.ProjectID, a, resolver.BusinessStatusChecker())
	if err != nil {
		return dataServicesModule{}, nil, err
	}
	var charges *orgresource.ConsumerChargeService
	module, err := dataservicesapp.NewModule(ctx, dataservicesapp.Dependencies{
		ProductDB: d.ProductDB, Access: authority, Live: authority, Specialist: authority, Funding: authority, Provider: d.Provider, Starter: d.Starter, TrustedProxyCIDRs: d.TrustedProxyCIDRs,
		Charges: func(owner orgresource.ConsumerChargeOwner) (dataacquisition.Charges, error) {
			var e error
			charges, e = buildCurrentResourceChargesWithOwners(ctx, o.productAcquisitionDB, o.storeCenterDB, o.commercialOwnerDB, a, capabilities, map[orgresource.ResourceConsumer]orgresource.ConsumerChargeOwner{orgresource.ConsumerAmazonData: owner})
			return charges, e
		},
	})
	if err != nil {
		return dataServicesModule{}, nil, err
	}
	w, err := d.NewWorker(module.Runner())
	if err != nil || w == nil {
		return dataServicesModule{}, nil, errors.New("data services worker construction failed")
	}
	*d.Worker = w
	return dataServicesModule{routes: module.BuildRoutes()}, charges, nil
}

func verifySameProductDatabase(ctx context.Context, source, data *gorm.DB) error {
	// current_database alone cannot distinguish databases on different servers.
	const query = `SELECT current_database() AS database, (SELECT oid FROM pg_database WHERE datname=current_database()) AS oid, inet_server_addr()::text AS host, inet_server_port() AS port`
	type target struct {
		Database string
		OID      uint32 `gorm:"column:oid"`
		Host     string
		Port     int
	}
	var left, right target
	if err := source.WithContext(ctx).Raw(query).Scan(&left).Error; err != nil {
		return err
	}
	if err := data.WithContext(ctx).Raw(query).Scan(&right).Error; err != nil {
		return err
	}
	if left != right || left.Database == "" {
		return errors.New("data services must share the physical Product database")
	}
	return nil
}

func validateDataServicesDescriptor(r httproute.Descriptor) error {
	for _, expected := range datahttp.BuildRoutes(datahttp.Dependencies{}) {
		if r.Method == expected.Method && r.Path == expected.Path {
			if r.Module == expected.Module && r.Permission == expected.Permission && r.AuthPolicy == expected.AuthPolicy && r.OrganizationAccessPolicy == expected.OrganizationAccessPolicy && r.RequestTimeout == expected.RequestTimeout && r.RejectUnreadRequestBody == expected.RejectUnreadRequestBody {
				return nil
			}
			break
		}
	}
	return errors.New("data services route loses its admitted identity, scope, permission or deadline")
}
