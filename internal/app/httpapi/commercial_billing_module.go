package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	billing "task-processor/internal/commercial/billing"
	billinghttp "task-processor/internal/commercial/billing/httpapi"
	"task-processor/internal/core/config"
	orgresourceadapter "task-processor/internal/integration/orgresource"
	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	moneystore "task-processor/internal/integration/persistence/money"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/listingsubscription"
)

func buildCommercialBillingModule(ctx context.Context, commercialDB, moneyDB *gorm.DB, authorizer *authz.ListingKitAuthorizer, cfg *config.Config) (kernelmodule.Module, error) {
	if ctx == nil || commercialDB == nil || moneyDB == nil || authorizer == nil || cfg == nil {
		return nil, errors.New("commercial billing or canonical money database unavailable")
	}
	wallet, err := moneystore.New(moneyDB)
	if err != nil {
		return nil, err
	}
	commercial, err := commercialstore.New(commercialDB)
	if err != nil {
		return nil, err
	}
	resourceRepository, err := orgresourceadapter.NewGormRepository(commercialDB, orgresourceadapter.TransactionConfig{})
	if err != nil {
		return nil, err
	}
	grantService, err := orgresource.NewPurchasedResourceGrantService(resourceRepository, orgresourceadapter.TrustedCommercialGrantAuthorizer{})
	if err != nil {
		return nil, err
	}
	service, err := billing.NewService(commercial, commercial, commercial, commercial, wallet, grantService)
	if err != nil {
		return nil, err
	}
	module := commercialBillingModule{handler: billinghttp.NewHandler(service)}
	zitadel := cfg.ListingKit.Zitadel
	if strings.TrimSpace(zitadel.TenantDirectoryToken) != "" && strings.TrimSpace(zitadel.AuthorizationAPIURL) != "" && strings.TrimSpace(zitadel.ProjectID) != "" {
		subscriptionOwner, ownerErr := listingsubscription.NewRuntimeService(listingsubscription.NewGormRepository(commercialDB))
		if ownerErr != nil {
			return nil, ownerErr
		}
		recoveryAuthorizer := subscriptionPurchaseRecoveryAuthorizer{reader: zitadelruntime.NewAuthorizationClient(zitadel.AuthorizationAPIURL, &http.Client{Timeout: 5 * time.Second}), serviceToken: zitadel.TenantDirectoryToken, projectID: zitadel.ProjectID, authorizer: authorizer}
		if enableErr := service.EnableSubscriptionPurchases(purchasedSubscriptionOwnerAdapter{owner: subscriptionOwner}, recoveryAuthorizer); enableErr != nil {
			return nil, enableErr
		}
		module.reconcileSubscriptions = func(run context.Context) error { return service.ReconcileRecoverableSubscriptionOrders(run, 50) }
	}
	return module, nil
}

type commercialBillingModule struct {
	handler                *billinghttp.Handler
	reconcileSubscriptions func(context.Context) error
}

func (commercialBillingModule) Name() string { return "commercial-billing" }

func (m commercialBillingModule) Enabled(cfg *config.Config) bool {
	return m.handler != nil && cfg != nil && cfg.Workbench.Enabled
}

func (m commercialBillingModule) Register(registry *kernelmodule.Registry) error {
	routes, err := billinghttp.Routes(m.handler)
	if err != nil {
		return err
	}
	registry.AddRoutes(routes...)
	return nil
}
