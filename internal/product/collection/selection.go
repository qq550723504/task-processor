package collection

import (
	"context"
	"task-processor/internal/authidentity"
	"task-processor/internal/product/catalog"
	"time"
)

// SelectionInput binds a member reference, never a caller-provided ProductKey.
type SelectionInput struct {
	ItemID                string
	ExpectedRevision      int64
	OriginalPublicationID string
	OriginalVersion       uint64
}

// AuthorizedSelection is a request-local capability. Its private state cannot
// be serialized into a workflow or recreated from a saved receipt.
type AuthorizedSelection struct {
	scope     Scope
	detail    ItemDetail
	expiresAt time.Time
}

func (s *Service) Select(ctx context.Context, input SelectionInput) (AuthorizedSelection, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, PermissionRead)
	if err != nil {
		return AuthorizedSelection{}, err
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.TenantID != scope.OrganizationID || identity.EffectiveOrganizationID != scope.OrganizationID || identity.UserID != scope.ActorID || identity.EffectiveMemberID != scope.MemberID || !time.Now().Before(identity.TokenExpiresAt) {
		return AuthorizedSelection{}, ErrForbidden
	}
	if !ValidID(input.ItemID) || input.ExpectedRevision < 1 || input.OriginalPublicationID == "" || input.OriginalVersion == 0 {
		return AuthorizedSelection{}, ErrInvalid
	}
	detail, err := s.readItem(ctx, scope, input.ItemID)
	if err != nil {
		return AuthorizedSelection{}, err
	}
	if detail.Item.Revision != input.ExpectedRevision || detail.Item.Source.PublicationID != input.OriginalPublicationID || detail.Item.Source.Version != input.OriginalVersion {
		return AuthorizedSelection{}, ErrConflict
	}
	cloned, err := catalog.CloneProductSnapshot(detail.Product)
	if err != nil {
		return AuthorizedSelection{}, ErrUnavailable
	}
	detail.Product = cloned
	expires := time.Now().Add(5 * time.Second)
	if identity.TokenExpiresAt.Before(expires) {
		expires = identity.TokenExpiresAt
	}
	if deadline, ok := ctx.Deadline(); ok && deadline.Before(expires) {
		expires = deadline
	}
	return AuthorizedSelection{scope: scope, detail: detail, expiresAt: expires}, nil
}
func (s AuthorizedSelection) Scope(ctx context.Context) (Scope, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if ctx == nil || ctx.Err() != nil || !ok || !time.Now().Before(s.expiresAt) || !time.Now().Before(identity.TokenExpiresAt) || identity.TenantID != s.scope.OrganizationID || identity.EffectiveOrganizationID != s.scope.OrganizationID || identity.UserID != s.scope.ActorID || identity.EffectiveMemberID != s.scope.MemberID || s.scope.Validate() != nil {
		return Scope{}, ErrForbidden
	}
	return s.scope, nil
}
func (s AuthorizedSelection) Read(ctx context.Context) (ItemDetail, error) {
	if _, err := s.Scope(ctx); err != nil {
		return ItemDetail{}, err
	}
	result := s.detail
	cloned, err := catalog.CloneProductSnapshot(result.Product)
	if err != nil {
		return ItemDetail{}, ErrUnavailable
	}
	result.Product = cloned
	return result, nil
}
