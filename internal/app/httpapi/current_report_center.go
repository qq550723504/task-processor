package httpapi

import (
	"context"
	"errors"
	"gorm.io/gorm"
	reportapp "task-processor/internal/app/reportcenter"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	reporthttp "task-processor/internal/reportcenter/httpapi"
)

func WithReportCenter(db *gorm.DB) CurrentApplicationOption {
	return func(o *currentApplicationOptions) { o.reportCenters++; o.reportCenterDB = db }
}

func validateReportCenterPool(o currentApplicationOptions, source *gorm.DB) error {
	if o.reportCenters == 0 {
		return nil
	}
	if o.reportCenters != 1 || o.reportCenterDB == nil {
		return errors.New("report center requires one existing owner pool")
	}
	others := []*gorm.DB{source, o.commercialOwnerDB, o.moneyOwnerDB, o.referralDB, o.productAcquisitionDB, o.imageAgentDB, o.accountAuditImageDB, o.accountAuditProductDB, o.storeCenterDB, o.localTrialDB, o.agentConfigurationDB, o.notificationDB, o.projectCenterDB, o.agentCustomizationDB}
	if o.toolMarket != nil {
		others = append(others, o.toolMarket.DB)
	}
	if o.ecoservices != nil {
		others = append(others, o.ecoservices.DB)
	}
	if o.supplyChain != nil {
		others = append(others, o.supplyChain.AssetDB)
	}
	if o.pod != nil {
		others = append(others, o.pod.AssetDB)
	}
	if o.productAgent != nil {
		others = append(others, o.productAgent.RunDB, o.productAgent.ReviewDB, o.productAgent.AssetDB)
	}
	if o.aiWorkbench != nil {
		others = append(others, o.aiWorkbench.DB)
	}
	if o.membership != nil {
		others = append(others, o.membership.ReceiptDB)
	}
	for _, db := range others {
		if db == o.reportCenterDB {
			return errors.New("report center requires an independent owner pool")
		}
	}
	return nil
}

type reportCenterModule struct{ routes []httproute.Descriptor }

func (reportCenterModule) Name() string                  { return reporthttp.ModuleName }
func (reportCenterModule) Enabled(c *config.Config) bool { return c != nil && c.Workbench.Enabled }
func (m reportCenterModule) Register(r *kernelmodule.Registry) error {
	r.AddRoutes(m.routes...)
	return nil
}

// These are the already mounted current-request readers. Missing owners disable
// only new capture; reading saved reports never depends on an Agent or model.
func buildReportCenter(ctx context.Context, db *gorm.DB, deps routeAuthDependencies, policy *authz.ListingKitAuthorizer, product *productAgentApplication) (kernelmodule.Module, error) {
	var reviews reportapp.ReviewReader
	if product != nil {
		reviews = product.reviews
	}
	// The native SupplyChain owns TargetRecord, not the offline SHEIN Record
	// contract. The latter is absent from normal composition; never substitute
	// an incompatible owner or silently mount the isolated trial application.
	h, err := reportapp.New(ctx, reportapp.Dependencies{DB: db, Resolver: deps.organizationResolver, Policy: policy, Reviews: reviews})
	if err != nil {
		return nil, err
	}
	// Review's existing live authorizer consumes this capability, not a fixed
	// execution scope. Report authorization keeps the values while refreshing id.
	bind := h.Bind
	h.Bind = func(ctx context.Context, header string) (context.Context, error) {
		bound, err := (productReviewCapabilityBinder{}).Bind(ctx, header)
		if err != nil {
			return nil, err
		}
		return bind(bound, header)
	}
	routes, err := reporthttp.BuildRoutes(h)
	if err != nil {
		return nil, err
	}
	return reportCenterModule{routes: routes}, nil
}
