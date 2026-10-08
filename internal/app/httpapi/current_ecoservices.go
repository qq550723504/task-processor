package httpapi

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"net/http"
	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	b "task-processor/internal/commercial/billing"
	"task-processor/internal/core/config"
	e "task-processor/internal/ecoservices"
	ehttp "task-processor/internal/ecoservices/httpapi"
	"task-processor/internal/httproute"
	"task-processor/internal/integration/ecoservicesbilling"
	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	estore "task-processor/internal/integration/persistence/ecoservices"
	moneystore "task-processor/internal/integration/persistence/money"
	"task-processor/internal/integration/servicepayments"
	kernelmodule "task-processor/internal/kernel/module"
	"time"
)

type EcoservicesDependencies struct {
	DB                 *gorm.DB
	Objects            e.PrivateObjectStore
	Channel            b.ServicePurchaseProvider
	Protection         b.ServicePayloadProtection
	MerchantProtection e.MerchantProtection
}

// The application adapts its process config; the domain exposes only routes.
type ecoservicesRouteModule struct{ handler *ehttp.Handler }

func (ecoservicesRouteModule) Name() string { return ehttp.ModuleName }
func (m ecoservicesRouteModule) Enabled(c *config.Config) bool {
	return m.handler != nil && c != nil && c.Workbench.Enabled
}
func (m ecoservicesRouteModule) Register(registry *kernelmodule.Registry) error {
	registry.AddRoutes(ehttp.Routes(m.handler)...)
	return nil
}

func WithEcoservices(deps EcoservicesDependencies) CurrentApplicationOption {
	return func(o *currentApplicationOptions) { o.ecoservicesConfigs++; o.ecoservices = &deps }
}

type ecoservicesCheckoutAuthorizer struct{ financialRecoveryAuthorizer }
type ecoservicesMerchantAuthorizer struct{ financialRecoveryAuthorizer }

func (a ecoservicesMerchantAuthorizer) AuthorizeMerchantApplication(ctx context.Context, org, actor string) error {
	if a.reader == nil || a.authorizer == nil || a.serviceToken == "" || a.projectID == "" {
		return e.ErrUnavailable
	}
	grant, err := a.reader.ReadExactServiceProjectAuthorization(ctx, a.serviceToken, actor, a.projectID, org)
	if err != nil || !grant.Found || grant.State != "STATE_ACTIVE" || !authz.AllowedOrganization(ctx, a.authorizer, actor, org, grant.Roles, authz.PermissionWorkbenchEcoservicesJoin) {
		return e.ErrForbidden
	}
	return nil
}

func (a ecoservicesCheckoutAuthorizer) AuthorizeServicePurchase(ctx context.Context, org, actor string) error {
	if a.reader == nil || a.authorizer == nil || a.serviceToken == "" || a.projectID == "" {
		return b.ErrFeatureUnavailable
	}
	grant, err := a.reader.ReadExactServiceProjectAuthorization(ctx, a.serviceToken, actor, a.projectID, org)
	if err != nil {
		return b.ErrAuthorizationRevoked
	}
	if !grant.Found || grant.State != "STATE_ACTIVE" || !authz.AllowedOrganization(ctx, a.authorizer, actor, org, grant.Roles, authz.PermissionWorkbenchEcoservicesPurchase) {
		return b.ErrAuthorizationRevoked
	}
	return nil
}
func buildEcoservices(ctx context.Context, d EcoservicesDependencies, commercialDB, moneyDB *gorm.DB, a *authz.ListingKitAuthorizer, cfg *config.Config) (*ehttp.Handler, func(context.Context) error, error) {
	if d.DB == nil || d.Objects == nil || d.Channel == nil || d.Protection == nil || d.MerchantProtection == nil || a == nil || cfg == nil || commercialDB == nil || moneyDB == nil {
		return nil, nil, e.ErrUnavailable
	}
	if err := commercialstore.VerifyServicePurchasesRuntime(ctx, commercialDB); err != nil {
		return nil, nil, err
	}
	if err := moneystore.VerifyProviderTopUpRuntime(ctx, moneyDB); err != nil {
		return nil, nil, err
	}
	repo, err := estore.NewRepository(ctx, d.DB)
	if err != nil {
		return nil, nil, err
	}
	commercial, err := commercialstore.New(commercialDB)
	if err != nil {
		return nil, nil, err
	}
	funds, err := moneystore.New(moneyDB)
	if err != nil {
		return nil, nil, err
	}
	z := cfg.ListingKit.Zitadel
	read := financialRecoveryAuthorizer{reader: zitadelruntime.NewAuthorizationClient(z.AuthorizationAPIURL, &http.Client{Timeout: 5 * time.Second}), serviceToken: z.TenantDirectoryToken, projectID: z.ProjectID, authorizer: a}
	source := ecoservicesbilling.Source{Repository: repo, Authorizer: ecoservicesCheckoutAuthorizer{read}}
	purchases, err := b.NewServicePurchases(commercial, funds, d.Channel, source, d.Protection)
	if err != nil {
		return nil, nil, err
	}
	service, err := e.NewService(repo, ecoservicesbilling.Trading{Purchases: purchases}, d.Channel.Profile().FreezeDays)
	if err != nil {
		return nil, nil, err
	}
	files, err := e.NewFileService(repo, d.Objects)
	if err != nil {
		return nil, nil, err
	}
	handler, err := ehttp.NewHandler(service, files, purchases)
	if err != nil {
		return nil, nil, err
	}
	verifier, ok := d.Channel.(servicepayments.NotificationVerifier)
	if !ok {
		return nil, nil, e.ErrUnavailable
	}
	handler.SetNotifications(servicepayments.NotificationIngress{Verifier: verifier, Inbox: commercial, Wake: repo})
	merchantChannel, ok := d.Channel.(e.MerchantOnboardingPort)
	if !ok {
		return nil, nil, e.ErrUnavailable
	}
	merchant, err := e.NewMerchantOnboarding(repo, merchantChannel, files, d.MerchantProtection, ecoservicesMerchantAuthorizer{read})
	if err != nil {
		return nil, nil, err
	}
	handler.SetMerchants(merchant)
	handler.SetFinancial(purchases)
	return handler, service.Recover, err
}
func validateEcoservicesDescriptor(d httproute.Descriptor) error {
	for _, expected := range ehttp.Routes(nil) {
		if d.Method == expected.Method && d.Path == expected.Path && d.Module == expected.Module && d.AuthPolicy == expected.AuthPolicy && d.OrganizationAccessPolicy == expected.OrganizationAccessPolicy && d.OrganizationTargetResolver == nil && d.Permission == expected.Permission && d.RequestTimeout == expected.RequestTimeout && d.RejectUnreadRequestBody == expected.RejectUnreadRequestBody && d.Handler != nil {
			return nil
		}
	}
	return errors.New("ecoservices route loses scoped permission or bounded request contract")
}
