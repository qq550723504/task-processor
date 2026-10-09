package imageagentapp

import (
	"context"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
)

type ImageScopeAuthorizer interface {
	AuthorizeImageExecution(context.Context, collection.Scope) error
}

type ScopedImageAuthorizer struct{ Live ImageScopeAuthorizer }

func (a ScopedImageAuthorizer) AuthorizeExecution(ctx context.Context, id imageagent.ExecutionIdentity) error {
	if ctx == nil || a.Live == nil || id.ScopeProtocol != imageagent.OrganizationScopeProtocol {
		return imageagent.ErrIdentityRequired
	}
	verified, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || verified.TenantID != id.TenantID || verified.EffectiveOrganizationID != id.TenantID || verified.UserID != id.UserID || verified.EffectiveMemberID != id.MemberID {
		return imageagent.ErrIdentityRequired
	}
	return a.Live.AuthorizeImageExecution(ctx, collection.Scope{OrganizationID: id.TenantID, ActorID: id.UserID, MemberID: id.MemberID})
}

type ImagePublicationScopeAuthorizer struct{ Live ImageScopeAuthorizer }

func (a ImagePublicationScopeAuthorizer) Authorize(ctx context.Context) (sourcing.PublicationScope, error) {
	id, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if ctx == nil || !ok || a.Live == nil || id.TenantID != id.EffectiveOrganizationID {
		return sourcing.PublicationScope{}, sourcing.ErrPublicationForbidden
	}
	scope := collection.Scope{OrganizationID: id.TenantID, ActorID: id.UserID, MemberID: id.EffectiveMemberID}
	if err := a.Live.AuthorizeImageExecution(ctx, scope); err != nil {
		return sourcing.PublicationScope{}, err
	}
	return sourcing.PublicationScope{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID}, nil
}
