package httpapi

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"task-processor/internal/authz"
	billing "task-processor/internal/commercial/billing"
	billinghttp "task-processor/internal/commercial/billing/httpapi"
	"task-processor/internal/core/config"
	orgresourceadapter "task-processor/internal/integration/orgresource"
	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	moneystore "task-processor/internal/integration/persistence/money"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/ledger/orgresource"
)

func buildCommercialBillingModule(ctx context.Context, commercialDB, moneyDB *gorm.DB, _ *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
	if ctx == nil || commercialDB == nil || moneyDB == nil {
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
	return commercialBillingModule{handler: billinghttp.NewHandler(service)}, nil
}

type commercialBillingModule struct{ handler *billinghttp.Handler }

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
