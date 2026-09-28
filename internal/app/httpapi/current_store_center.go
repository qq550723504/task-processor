package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"gorm.io/gorm"

	storeapp "task-processor/internal/app/storecenter"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/ledger/orgresource"
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

func WithStoreOfficialConnection(provider storecenter.OfficialConnectionProvider, protection storecenter.OfficialCredentialProtection) CurrentApplicationOption {
	return func(o *currentApplicationOptions) {
		o.officialStoreConfigs++
		o.officialStoreProvider = provider
		o.officialStoreProtection = protection
	}
}
func buildCurrentStoreCenterModule(ctx context.Context, records, quota *gorm.DB, authorizer *authz.ListingKitAuthorizer, charges orgresource.ConsumerChargePort, provider storecenter.OfficialConnectionProvider, protection storecenter.OfficialCredentialProtection) (kernelmodule.Module, error) {
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
	if authorizer == nil {
		return nil, errors.New("store center authorizer unavailable")
	}
	repo, err := storecenter.NewMemberScopedStoreRepository(records, currentStoreMemberAuthorizer{authorizer: authorizer})
	if err != nil {
		return nil, err
	}
	audit, err := storecenter.NewGormAuditRepository(records)
	if err != nil {
		return nil, err
	}
	ledger := listingsubscription.NewGormStoreQuotaLedger(listingsubscription.NewGormRepository(quota))
	var connections *storeapp.OfficialConnections
	if provider == nil && protection == nil {
		connections, err = storeapp.NewUnconfiguredOfficialConnections(repo)
	} else {
		connections, err = storeapp.NewOfficialConnections(repo, provider, protection)
	}
	if err != nil {
		return nil, err
	}
	service, err := storecenter.NewService(repo, ledger, audit, connections, time.Now)
	if err != nil {
		return nil, err
	}
	executor, err := storeapp.NewServiceLifecycleExecutor(repo, charges, connections)
	if err != nil {
		return nil, err
	}
	lifecycle, err := storecenter.NewServiceLifecycleApplication(repo, executor, connections, authorizer, currentStorePeriodPolicy{}, time.Now)
	if err != nil {
		return nil, err
	}
	handler, err := storehttp.NewHandlerWithServiceLifecycle(service, lifecycle)
	if err != nil {
		return nil, err
	}
	if err := handler.SetOfficialConnections(connections); err != nil {
		return nil, err
	}
	return storehttp.NewModule(handler), nil
}

type currentStorePeriodPolicy struct{}

func (currentStorePeriodPolicy) MaxQuantity(context.Context, string, storecenter.ServiceCommand) (int64, error) {
	return 12, nil
}

// Current Store descriptors resolve live Organization access before invoking
// this capability. No member ID or administrator flag comes from the request.
type currentStoreMemberAuthorizer struct{ authorizer *authz.ListingKitAuthorizer }

func (a currentStoreMemberAuthorizer) AuthorizeStoreMember(ctx context.Context, organizationID string) (storecenter.StoreMemberAccess, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.EffectiveOrganizationID != organizationID || identity.TenantID != organizationID || !authidentity.IsBoundedIdentifier(identity.EffectiveMemberID) || !a.authorizer.Authorize(identity.UserID, identity.Roles, authz.PermissionWorkbenchStoreRead) {
		return storecenter.StoreMemberAccess{}, storecenter.ErrNotFound
	}
	granted := false
	for _, grant := range identity.OrganizationGrants {
		if grant.OrganizationID == organizationID && grant.AuthorizationID == identity.EffectiveMemberID {
			granted = true
			break
		}
	}
	if !granted {
		return storecenter.StoreMemberAccess{}, storecenter.ErrNotFound
	}
	return storecenter.StoreMemberAccess{OrganizationID: organizationID, ActorID: identity.UserID, MemberID: identity.EffectiveMemberID, Administrator: a.authorizer.IsTenantAdmin(identity.UserID, identity.Roles), CanWrite: a.authorizer.Authorize(identity.UserID, identity.Roles, authz.PermissionWorkbenchStoreUpdate)}, nil
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
	{http.MethodPost, "/api/v1/workbench/stores/:store_id/activate", authz.PermissionWorkbenchStoreLifecycle},
	{http.MethodPost, "/api/v1/workbench/stores/:store_id/renew", authz.PermissionWorkbenchStoreLifecycle},
	{http.MethodPost, "/api/v1/workbench/stores/:store_id/reactivate", authz.PermissionWorkbenchStoreLifecycle},
	{http.MethodGet, "/api/v1/workbench/stores/:store_id/connection", authz.PermissionWorkbenchStoreRead},
	{http.MethodPost, "/api/v1/workbench/stores/:store_id/connection/begin", authz.PermissionWorkbenchStoreUpdate},
	{http.MethodPost, "/api/v1/workbench/stores/:store_id/connection/complete", authz.PermissionWorkbenchStoreUpdate},
	{http.MethodPost, "/api/v1/workbench/stores/:store_id/connection/disconnect", authz.PermissionWorkbenchStoreUpdate},
}

func validateCurrentStoreDescriptor(d httproute.Descriptor) error {
	for _, expected := range currentStoreCenterRoutes {
		if expected.method == d.Method && expected.path == d.Path && d.Module == storehttp.ModuleName && d.AuthPolicy == httproute.AuthPolicyCurrentIdentity && d.OrganizationAccessPolicy == httproute.OrganizationAccessPolicyLiveWrite && d.OrganizationTargetResolver == nil && d.Permission == expected.permission && d.Handler != nil {
			return nil
		}
	}
	return errors.New("store center route loses current identity, live permission or CRUD scope")
}
