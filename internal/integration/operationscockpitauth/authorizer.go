// Package operationscockpitauth consumes current Organization, Casbin and
// member-scoped Store contracts; it owns no grants or Store facts.
package operationscockpitauth

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
	"task-processor/internal/httproute"
	"task-processor/internal/integration/commercetoolauth"
	c "task-processor/internal/operationscockpit"
	"task-processor/internal/storecenter"
	"task-processor/internal/workbenchcontext"
	"time"
)

type OrganizationResolver interface {
	Resolve(context.Context, httproute.OrganizationAccessPolicy, workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error)
}
type Authorizer struct {
	fresh   *commercetoolauth.FreshWorkbenchPrincipalResolver
	policy  *authz.ListingKitAuthorizer
	records *gorm.DB
}

func New(resolver OrganizationResolver, policy *authz.ListingKitAuthorizer, records *gorm.DB) (*Authorizer, error) {
	if resolver == nil || policy == nil || records == nil {
		return nil, c.ErrUnavailable
	}
	fresh, err := commercetoolauth.NewFreshWorkbenchPrincipalResolver(commercetoolauth.FreshOrganizationResolverFunc(func(ctx context.Context, r commercetoolauth.OrganizationRequest) (authidentity.AuthenticatedIdentity, error) {
		original, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
		if !ok || !authidentity.IsBoundedIdentifier(original.EffectiveMemberID) {
			return authidentity.AuthenticatedIdentity{}, c.ErrForbidden
		}
		resolved, err := resolver.Resolve(ctx, httproute.OrganizationAccessPolicyLiveWrite, workbenchcontext.ResolveInput{Identity: r.Identity, BearerToken: r.BearerToken, RequestedOrganizationID: r.RequestedOrganizationID})
		if err != nil {
			return resolved, err
		}
		if resolved.EffectiveMemberID != original.EffectiveMemberID {
			return resolved, c.ErrForbidden
		}
		return resolved, nil
	}), nil)
	if err != nil {
		return nil, c.ErrUnavailable
	}
	return &Authorizer{fresh: fresh, policy: policy, records: records}, nil
}

func (a *Authorizer) Bind(ctx context.Context, header string) (context.Context, error) {
	id, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || !(c.Scope{OrganizationID: id.EffectiveOrganizationID, ActorID: id.UserID}).Valid() {
		return nil, c.ErrForbidden
	}
	if id.TenantID != id.EffectiveOrganizationID || !authidentity.IsBoundedIdentifier(id.EffectiveMemberID) || !id.TokenExpiresAt.After(time.Now()) || !strings.HasPrefix(header, "Bearer ") || header[7:] == "" || header[7:] != strings.TrimSpace(header[7:]) {
		return nil, c.ErrForbidden
	}
	return commercetoolauth.WithOrganizationRequest(ctx, commercetoolauth.OrganizationRequest{Identity: authidentity.AuthenticatedIdentity{UserID: id.UserID, HomeOrganizationID: id.HomeOrganizationID, TokenExpiresAt: id.TokenExpiresAt}, BearerToken: header[7:], RequestedOrganizationID: id.EffectiveOrganizationID}), nil
}

func (a *Authorizer) principal(ctx context.Context, scope c.Scope) (commercetool.Principal, error) {
	id, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if a == nil || ctx == nil || !ok || !scope.Valid() || id.UserID != scope.ActorID || id.EffectiveOrganizationID != scope.OrganizationID || id.TenantID != scope.OrganizationID {
		return commercetool.Principal{}, c.ErrForbidden
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return commercetool.Principal{}, c.ErrForbidden
	}
	p, err := a.fresh.ResolveFreshPrincipal(ctx)
	if errors.Is(err, workbenchcontext.ErrAuthorizationDependencyUnavailable) || ctx.Err() != nil {
		return p, c.ErrUnavailable
	}
	if err != nil || p.UserID != scope.ActorID || p.TenantID != scope.OrganizationID {
		return p, c.ErrForbidden
	}
	return p, nil
}
func (a *Authorizer) Current(ctx context.Context, scope c.Scope) (c.Access, error) {
	p, err := a.principal(ctx, scope)
	if err != nil {
		return c.Access{}, err
	}
	var access c.Access
	for _, item := range []struct {
		permission string
		value      *bool
	}{
		{authz.PermissionCockpitGoalsRead, &access.GoalsRead}, {authz.PermissionCockpitGoalsCreate, &access.GoalsCreate}, {authz.PermissionCockpitGoalsManage, &access.GoalsManage}, {authz.PermissionCockpitStoresRead, &access.StoresRead}, {authz.PermissionCockpitFactsWrite, &access.FactsWrite}, {authz.PermissionCockpitAlertsRead, &access.AlertsRead}, {authz.PermissionCockpitAdviceRead, &access.AdviceRead},
	} {
		allowed, err := authz.AuthorizeOrganization(ctx, a.policy, p.UserID, p.TenantID, p.Roles, item.permission)
		if err != nil {
			return c.Access{}, c.ErrUnavailable
		}
		*item.value = allowed
	}
	return access, nil
}

type memberAuthorization struct{ access storecenter.StoreMemberAccess }

func (m memberAuthorization) AuthorizeStoreMember(ctx context.Context, org string) (storecenter.StoreMemberAccess, error) {
	if ctx.Err() != nil || org != m.access.OrganizationID {
		return storecenter.StoreMemberAccess{}, storecenter.ErrNotFound
	}
	return m.access, nil
}
func (a *Authorizer) stores(ctx context.Context, scope c.Scope, db *gorm.DB) (*storecenter.MemberScopedStoreRepository, error) {
	p, err := a.principal(ctx, scope)
	if err != nil {
		return nil, err
	}
	allowed, err := authz.AuthorizeOrganization(ctx, a.policy, p.UserID, p.TenantID, p.Roles, authz.PermissionWorkbenchStoreRead)
	if err != nil {
		return nil, c.ErrUnavailable
	}
	if !allowed {
		return nil, c.ErrForbidden
	}
	id, _ := authidentity.AuthenticatedIdentityFromContext(ctx)
	r, err := storecenter.NewMemberScopedStoreRepository(db, memberAuthorization{storecenter.StoreMemberAccess{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: id.EffectiveMemberID, Administrator: a.policy.IsTenantAdmin(p.UserID, p.Roles)}})
	return r, storeError(err)
}
func storeError(err error) error {
	if errors.Is(err, storecenter.ErrNotFound) {
		return c.ErrForbidden
	}
	if err != nil {
		return c.ErrUnavailable
	}
	return nil
}
func (a *Authorizer) ReadStores(ctx context.Context, scope c.Scope, ids []string) error {
	if len(ids) > 500 {
		return c.ErrInvalid
	}
	r, err := a.stores(ctx, scope, a.records)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if !c.UUID(id) {
			return c.ErrInvalid
		}
		if _, err := r.Get(ctx, scope.OrganizationID, id); err != nil {
			return storeError(err)
		}
	}
	return nil
}
func (a *Authorizer) LockStores(ctx context.Context, tx *gorm.DB, scope c.Scope, ids []string) error {
	r, err := a.stores(ctx, scope, tx)
	if err != nil {
		return err
	}
	return storeError(r.LockRead(ctx, scope.OrganizationID, ids))
}
func (a *Authorizer) ListStores(ctx context.Context, scope c.Scope) ([]c.StoreReference, error) {
	r, err := a.stores(ctx, scope, a.records)
	if err != nil {
		return nil, err
	}
	result := []c.StoreReference{}
	for page := 1; page <= 5; page++ {
		list, err := r.List(ctx, scope.OrganizationID, storecenter.StoreListQuery{Page: page, PageSize: 100})
		if err != nil {
			return nil, storeError(err)
		}
		if list.Total > 500 || list.Total < 0 {
			return nil, c.ErrInvalid
		}
		for _, s := range list.Stores {
			result = append(result, c.StoreReference{ID: s.ID(), Name: s.Name(), Platform: string(s.Platform()), Region: s.Region(), Status: string(s.RecordStatus())})
		}
		if int64(page*100) >= list.Total {
			break
		}
	}
	ids := make([]string, len(result))
	for i, s := range result {
		ids[i] = s.ID
	}
	if err := a.ReadStores(ctx, scope, ids); err != nil {
		return nil, err
	}
	return result, nil
}
