package httpapi

import (
	"context"
	"errors"
	"net/url"
	"time"

	"gorm.io/gorm"
	accountallocation "task-processor/internal/accountallocation"
	allocationhttp "task-processor/internal/accountallocation/httpapi"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	allocationstore "task-processor/internal/integration/persistence/accountallocation"
	"task-processor/internal/integration/zitadel/membership"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listingsubscription"
)

type commercialTokenQuotaReader struct {
	repository *listingsubscription.GormRepository
}

func (r commercialTokenQuotaReader) ReadTokenQuota(ctx context.Context, organizationID string) (accountallocation.Quota, error) {
	if r.repository == nil {
		return accountallocation.Quota{}, accountallocation.ErrQuotaUnavailable
	}
	return r.repository.ReadTokenQuota(ctx, organizationID, time.Now().UTC())
}

func buildAccountResourceAllocationModule(ctx context.Context, cfg *config.Config, sourceDB, commercialDB *gorm.DB, deps MembershipDependencies, authorizer *authz.ListingKitAuthorizer, auth routeAuthDependencies) (kernelmodule.Module, error) {
	if ctx == nil || cfg == nil || sourceDB == nil || commercialDB == nil || authorizer == nil || auth.authorizer != authorizer || deps.ReceiptDB == nil {
		return nil, accountallocation.ErrUnavailable
	}
	origin, err := url.Parse(deps.ProviderOrigin)
	if err != nil || origin.Host == "" || (origin.Scheme != "http" && origin.Scheme != "https") || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || deps.ReadToken == "" {
		return nil, accountallocation.ErrUnavailable
	}
	directory, err := membership.NewClient(deps.ProviderOrigin, deps.ReadToken, cfg.ListingKit.Zitadel.ProjectID, nil)
	if err != nil {
		return nil, err
	}
	// Allocation, entitlement and usage are one commercial consistency boundary.
	repository, err := allocationstore.New(commercialDB)
	if err != nil {
		return nil, err
	}
	service, err := accountallocation.NewService(commercialTokenQuotaReader{repository: listingsubscription.NewGormRepository(commercialDB)}, repository)
	if err != nil {
		return nil, err
	}
	handler, err := allocationhttp.NewHandler(service, directory)
	if err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, errors.New("account resource allocation startup canceled")
	}
	return allocationhttp.NewModule(handler), nil
}
