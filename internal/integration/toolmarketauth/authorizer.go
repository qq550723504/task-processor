// Package toolmarketauth adapts current Workbench and Casbin contracts. It
// creates no grants, roles, identity facts or alternate permission framework.
package toolmarketauth

import (
	"context"
	"slices"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
	"task-processor/internal/httproute"
	"task-processor/internal/integration/commercetoolauth"
	tm "task-processor/internal/toolmarket"
	"task-processor/internal/workbenchcontext"
	"time"
)

type OrganizationResolver interface {
	Resolve(context.Context, httproute.OrganizationAccessPolicy, workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error)
}
type Authorizer struct {
	cached *commercetoolauth.WorkbenchPrincipalResolver
	fresh  *commercetoolauth.FreshWorkbenchPrincipalResolver
	policy authz.StaticAuthorizer
}

func New(resolver OrganizationResolver, policy authz.StaticAuthorizer) (*Authorizer, error) {
	if resolver == nil || policy == nil {
		return nil, tm.ErrUnavailable
	}
	cached, e := commercetoolauth.NewWorkbenchPrincipalResolver(commercetoolauth.CachedReadOrganizationResolverFunc(func(ctx context.Context, r commercetoolauth.OrganizationRequest) (authidentity.AuthenticatedIdentity, error) {
		if !authidentity.IsBoundedIdentifier(r.Identity.UserID) || !authidentity.IsBoundedIdentifier(r.RequestedOrganizationID) || r.BearerToken == "" || !r.Identity.TokenExpiresAt.After(time.Now()) {
			return authidentity.AuthenticatedIdentity{}, tm.ErrForbidden
		}
		id, err := resolver.Resolve(ctx, httproute.OrganizationAccessPolicyCachedRead, workbenchcontext.ResolveInput{Identity: r.Identity, BearerToken: r.BearerToken, RequestedOrganizationID: r.RequestedOrganizationID})
		if err != nil {
			return authidentity.AuthenticatedIdentity{}, err
		}
		if ctx.Err() != nil || id.UserID != r.Identity.UserID || !id.TokenExpiresAt.Equal(r.Identity.TokenExpiresAt) || id.TenantID != r.RequestedOrganizationID || id.EffectiveOrganizationID != r.RequestedOrganizationID {
			return authidentity.AuthenticatedIdentity{}, tm.ErrForbidden
		}
		return id, nil
	}), nil)
	if e != nil {
		return nil, tm.ErrUnavailable
	}
	fresh, e := commercetoolauth.NewFreshWorkbenchPrincipalResolver(commercetoolauth.FreshOrganizationResolverFunc(func(ctx context.Context, r commercetoolauth.OrganizationRequest) (authidentity.AuthenticatedIdentity, error) {
		return resolver.Resolve(ctx, httproute.OrganizationAccessPolicyLiveWrite, workbenchcontext.ResolveInput{Identity: r.Identity, BearerToken: r.BearerToken, RequestedOrganizationID: r.RequestedOrganizationID})
	}), nil)
	if e != nil {
		return nil, tm.ErrUnavailable
	}
	return &Authorizer{cached: cached, fresh: fresh, policy: policy}, nil
}

// Bind receives only the authenticated request's bearer header after current
// identity middleware. Fresh resolver reuses the original actor/token expiry.
func (a *Authorizer) Bind(ctx context.Context, header string) (context.Context, error) {
	id, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || !authidentity.IsBoundedIdentifier(id.EffectiveOrganizationID) || !strings.HasPrefix(header, "Bearer ") || strings.TrimSpace(header[7:]) != header[7:] || header[7:] == "" {
		return nil, tm.ErrForbidden
	}
	return commercetoolauth.WithOrganizationRequest(ctx, commercetoolauth.OrganizationRequest{Identity: authidentity.AuthenticatedIdentity{UserID: id.UserID, HomeOrganizationID: id.HomeOrganizationID, TokenExpiresAt: id.TokenExpiresAt}, BearerToken: header[7:], RequestedOrganizationID: id.EffectiveOrganizationID}), nil
}
func (a *Authorizer) Authorize(ctx context.Context, permission string, platform bool) (tm.Scope, error) {
	return a.authorize(ctx, permission, platform, false)
}

// AuthorizeRead follows the existing CachedRead policy, including the display
// of management/customization actions. Mutations use Authorize and LiveWrite.
func (a *Authorizer) AuthorizeRead(ctx context.Context, permission string, platform bool) (tm.Scope, error) {
	return a.authorize(ctx, permission, platform, true)
}

func (a *Authorizer) authorize(ctx context.Context, permission string, platform, read bool) (tm.Scope, error) {
	if a == nil || a.policy == nil || ctx == nil || ctx.Err() != nil {
		return tm.Scope{}, tm.ErrForbidden
	}
	if platform {
		id, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
		if !ok || permission != authz.PermissionListingKitPlatformAdm || !id.TokenExpiresAt.After(time.Now()) || !a.policy.Authorize(id.UserID, id.Roles, authz.PermissionListingKitPlatformAdm) {
			return tm.Scope{}, tm.ErrForbidden
		}
		return tm.Scope{ActorID: id.UserID}, nil
	}
	if permission != authz.PermissionWorkbenchToolsRead && permission != authz.PermissionWorkbenchToolsManage && permission != authz.PermissionWorkbenchToolsCustomize {
		return tm.Scope{}, tm.ErrForbidden
	}
	var p commercetool.Principal
	var e error
	if read {
		p, e = a.cached.ResolvePrincipal(ctx)
	} else {
		p, e = a.fresh.ResolveFreshPrincipal(ctx)
	}
	if e != nil {
		return tm.Scope{}, tm.ErrForbidden
	}
	if permission == authz.PermissionWorkbenchToolsManage && !slices.Contains(p.Roles, "listingkit_admin") {
		return tm.Scope{}, tm.ErrForbidden
	}
	if !authz.AllowedOrganization(ctx, a.policy, p.UserID, p.TenantID, p.Roles, permission) {
		return tm.Scope{}, tm.ErrForbidden
	}
	return tm.Scope{ActorID: p.UserID, OrganizationID: p.TenantID}, nil
}
