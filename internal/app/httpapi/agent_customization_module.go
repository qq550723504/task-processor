package httpapi

import (
	"context"
	"errors"
	"gorm.io/gorm"
	d "task-processor/internal/agentcustomization"
	customhttp "task-processor/internal/agentcustomization/httpapi"
	"task-processor/internal/core/config"
	store "task-processor/internal/integration/persistence/agentcustomization"
	kernelmodule "task-processor/internal/kernel/module"
)

func WithAgentCustomization(db *gorm.DB) CurrentApplicationOption {
	return func(o *currentApplicationOptions) { o.agentCustomizations++; o.agentCustomizationDB = db }
}

func validateAgentCustomizationPool(o currentApplicationOptions, source *gorm.DB) error {
	if o.agentCustomizations == 0 {
		return nil
	}
	if o.agentCustomizations != 1 || o.agentCustomizationDB == nil {
		return errors.New("agent customization requires one existing owner pool")
	}
	others := []*gorm.DB{source, o.commercialOwnerDB, o.moneyOwnerDB, o.referralDB, o.productAcquisitionDB, o.imageAgentDB, o.accountAuditImageDB, o.accountAuditProductDB, o.storeCenterDB, o.localTrialDB, o.agentConfigurationDB, o.notificationDB}
	if o.toolMarket != nil {
		others = append(others, o.toolMarket.DB)
	}
	if o.ecoservices != nil {
		others = append(others, o.ecoservices.DB)
	}
	if o.supplyChain != nil {
		others = append(others, o.supplyChain.AssetDB)
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
		if db == o.agentCustomizationDB {
			return errors.New("agent customization requires an independent owner pool")
		}
	}
	return nil
}

type agentCustomizationModule struct{ handler *customhttp.Handler }

func (agentCustomizationModule) Name() string                { return "agent-customization" }
func (agentCustomizationModule) Enabled(*config.Config) bool { return true }
func (m agentCustomizationModule) Register(reg *kernelmodule.Registry) error {
	reg.AddRoutes(customhttp.Routes(m.handler)...)
	return nil
}
func buildAgentCustomizationModule(ctx context.Context, db *gorm.DB) (kernelmodule.Module, error) {
	if db == nil {
		return nil, d.ErrUnavailable
	}
	raw, err := db.DB()
	if err != nil {
		return nil, d.ErrUnavailable
	}
	if err = store.VerifySchema(ctx, raw); err != nil {
		return nil, err
	}
	repository, err := store.New(raw)
	if err != nil {
		return nil, err
	}
	service, err := d.NewService(repository)
	if err != nil {
		return nil, err
	}
	handler, err := customhttp.NewHandler(service)
	if err != nil {
		return nil, err
	}
	return agentCustomizationModule{handler: handler}, nil
}
