package reportcenterapp

import (
	"context"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	rc "task-processor/internal/reportcenter"
	"task-processor/internal/workbenchcontext"
	"time"
)

type OrganizationResolver interface {
	Resolve(context.Context, httproute.OrganizationAccessPolicy, workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error)
}
type authorization struct {
	resolver OrganizationResolver
	policy   authz.StaticAuthorizer
}
type boundRequest struct{ input workbenchcontext.ResolveInput }
type requestKey struct{}

func (a *authorization) Bind(ctx context.Context, header string) (context.Context, error) {
	id, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || !authidentity.IsBoundedIdentifier(id.EffectiveOrganizationID) || !strings.HasPrefix(header, "Bearer ") || header[7:] == "" || strings.TrimSpace(header[7:]) != header[7:] {
		return nil, rc.ErrForbidden
	}
	original := authidentity.AuthenticatedIdentity{UserID: id.UserID, HomeOrganizationID: id.HomeOrganizationID, TokenExpiresAt: id.TokenExpiresAt}
	return context.WithValue(ctx, requestKey{}, boundRequest{workbenchcontext.ResolveInput{Identity: original, BearerToken: header[7:], RequestedOrganizationID: id.EffectiveOrganizationID}}), nil
}
func (a *authorization) Authorize(ctx context.Context, scope rc.Scope, manage bool) (context.Context, error) {
	bound, ok := ctx.Value(requestKey{}).(boundRequest)
	if !ok || a == nil || a.resolver == nil || a.policy == nil || ctx.Err() != nil || bound.input.Identity.UserID != scope.ActorID || bound.input.RequestedOrganizationID != scope.OrganizationID || !bound.input.Identity.TokenExpiresAt.After(time.Now()) {
		return nil, rc.ErrForbidden
	}
	access := httproute.OrganizationAccessPolicyCachedRead
	permission := rc.ReadPermission
	if manage {
		access = httproute.OrganizationAccessPolicyLiveWrite
		permission = rc.ManagePermission
	}
	id, e := a.resolver.Resolve(ctx, access, bound.input)
	if e != nil {
		return nil, rc.ErrForbidden
	}
	if id.UserID != scope.ActorID || id.TenantID != scope.OrganizationID || id.EffectiveOrganizationID != scope.OrganizationID || !id.TokenExpiresAt.Equal(bound.input.Identity.TokenExpiresAt) || !id.TokenExpiresAt.After(time.Now()) {
		return nil, rc.ErrForbidden
	}
	fresh := authidentity.WithAuthenticatedIdentity(ctx, id)
	if !authz.AllowedOrganization(fresh, a.policy, "", scope.OrganizationID, id.Roles, permission) {
		return nil, rc.ErrForbidden
	}
	return fresh, nil
}
