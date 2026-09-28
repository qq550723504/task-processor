package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listingsubscription"
	"task-processor/internal/storecenter"
	storehttp "task-processor/internal/storecenter/httpapi"
)

// WithStoreCenter borrows separately owned record and quota pools. The runtime
// retains responsibility for closing them; this builder never opens cfg.Database.
func WithStoreCenter(records, quota *gorm.DB) CurrentApplicationOption {
	return func(o *currentApplicationOptions) {
		o.storeCenters++
		o.storeCenterDB = records
		o.storeQuotaDB = quota
	}
}

func buildCurrentStoreCenterModule(ctx context.Context, records, quota *gorm.DB) (kernelmodule.Module, error) {
	if records == nil || quota == nil || records == quota {
		return nil, errors.New("store center requires independent record and quota pools")
	}
	if err := storecenter.VerifyCurrentSchema(ctx, records); err != nil {
		return nil, fmt.Errorf("verify store center schema: %w", err)
	}
	if err := storecenter.VerifyRuntimePermissions(ctx, records); err != nil {
		return nil, err
	}
	if err := listingsubscription.VerifyStoreQuotaRuntime(ctx, quota); err != nil {
		return nil, err
	}
	repo, err := storecenter.NewGormStoreRepository(records)
	if err != nil {
		return nil, err
	}
	audit, err := storecenter.NewGormAuditRepository(records)
	if err != nil {
		return nil, err
	}
	ledger := listingsubscription.NewGormStoreQuotaLedger(listingsubscription.NewGormRepository(quota))
	service, err := storecenter.NewService(repo, ledger, audit, unavailableConnectionStatusProvider{}, time.Now)
	if err != nil {
		return nil, err
	}
	handler, err := storehttp.NewHandler(service)
	if err != nil {
		return nil, err
	}
	return storehttp.NewModule(handler), nil
}

var currentStoreCenterRoutes = []struct{ method, path, permission string }{
	{http.MethodGet, "/api/v1/workbench/stores", authz.PermissionWorkbenchStoreRead},
	{http.MethodPost, "/api/v1/workbench/stores", authz.PermissionWorkbenchStoreCreate},
	{http.MethodPost, "/api/v1/workbench/stores/:store_id/resume", authz.PermissionWorkbenchStoreCreate},
	{http.MethodGet, "/api/v1/workbench/stores/:store_id", authz.PermissionWorkbenchStoreRead},
	{http.MethodPut, "/api/v1/workbench/stores/:store_id", authz.PermissionWorkbenchStoreUpdate},
	{http.MethodPost, "/api/v1/workbench/stores/:store_id/disable", authz.PermissionWorkbenchStoreLifecycle},
	{http.MethodPost, "/api/v1/workbench/stores/:store_id/enable", authz.PermissionWorkbenchStoreLifecycle},
	{http.MethodDelete, "/api/v1/workbench/stores/:store_id", authz.PermissionWorkbenchStoreDelete},
}

func validateCurrentStoreDescriptor(d httproute.Descriptor) error {
	for _, expected := range currentStoreCenterRoutes {
		if expected.method == d.Method && expected.path == d.Path && d.Module == storehttp.ModuleName && d.AuthPolicy == httproute.AuthPolicyCurrentIdentity && d.OrganizationAccessPolicy == httproute.OrganizationAccessPolicyLiveWrite && d.OrganizationTargetResolver == nil && d.Permission == expected.permission && d.Handler != nil {
			return nil
		}
	}
	return errors.New("store center route loses current identity, live permission or CRUD scope")
}
