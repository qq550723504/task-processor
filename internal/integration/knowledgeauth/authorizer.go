// Package knowledgeauth adapts existing Workbench live grants and Casbin to
// Knowledge's content admission port. It owns no identity or permission policy.
package knowledgeauth

import (
	"context"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	"task-processor/internal/integration/commercetoolauth"
	k "task-processor/internal/knowledge"
	"task-processor/internal/workbenchcontext"
)

type OrganizationResolver interface {
	Resolve(context.Context, httproute.OrganizationAccessPolicy, workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error)
}
type Authorizer struct {
	fresh  *commercetoolauth.FreshWorkbenchPrincipalResolver
	policy *authz.ListingKitAuthorizer
}

func NewAuthorizer(resolver OrganizationResolver, policy *authz.ListingKitAuthorizer) (*Authorizer, error) {
	if resolver == nil || policy == nil {
		return nil, k.ErrUnavailable
	}
	fresh, err := commercetoolauth.NewFreshWorkbenchPrincipalResolver(commercetoolauth.FreshOrganizationResolverFunc(func(ctx context.Context, r commercetoolauth.OrganizationRequest) (authidentity.AuthenticatedIdentity, error) {
		return resolver.Resolve(ctx, httproute.OrganizationAccessPolicyLiveWrite, workbenchcontext.ResolveInput{Identity: r.Identity, BearerToken: r.BearerToken, RequestedOrganizationID: r.RequestedOrganizationID})
	}), nil)
	if err != nil {
		return nil, k.ErrUnavailable
	}
	return &Authorizer{fresh: fresh, policy: policy}, nil
}
func (a *Authorizer) AuthorizeKnowledge(ctx context.Context) (k.Scope, error) {
	p, err := a.fresh.ResolveFreshPrincipal(ctx)
	if err != nil || !authz.AllowedOrganization(ctx, a.policy, p.UserID, p.TenantID, p.Roles, authz.PermissionWorkbenchKnowledgeRead) {
		return k.Scope{}, k.ErrForbidden
	}
	return k.Scope{OrganizationID: p.TenantID, ActorID: p.UserID}, nil
}
