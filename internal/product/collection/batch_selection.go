package collection

import (
	"context"
	"sort"
	"task-processor/internal/authidentity"
	"time"
)

// An empty ItemIDs means all current members, not the current UI page.
type BatchSelectionInput struct {
	BatchID          string   `json:"batchId"`
	ExpectedRevision int64    `json:"expectedRevision"`
	ItemIDs          []string `json:"itemIds,omitempty"`
}

func (i BatchSelectionInput) Validate() error {
	if !ValidID(i.BatchID) || i.ExpectedRevision < 1 || len(i.ItemIDs) > 1000 {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, id := range i.ItemIDs {
		if !ValidID(id) || seen[id] {
			return ErrInvalid
		}
		seen[id] = true
	}
	return nil
}

type AuthorizedBatchSelection struct {
	scope     Scope
	input     BatchSelectionInput
	expiresAt time.Time
}

func (s *Service) SelectBatch(ctx context.Context, input BatchSelectionInput) (AuthorizedBatchSelection, error) {
	if ctx == nil {
		return AuthorizedBatchSelection{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, PermissionRead)
	if err != nil {
		return AuthorizedBatchSelection{}, err
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || !selectionIdentityMatches(identity, scope) || !time.Now().Before(identity.TokenExpiresAt) {
		return AuthorizedBatchSelection{}, ErrForbidden
	}
	if input.Validate() != nil {
		return AuthorizedBatchSelection{}, ErrInvalid
	}
	batch, err := s.store.ReadBatch(ctx, scope, input.BatchID)
	if err != nil {
		return AuthorizedBatchSelection{}, err
	}
	if batch.ID != input.BatchID || batch.Revision != input.ExpectedRevision {
		return AuthorizedBatchSelection{}, ErrConflict
	}
	input.ItemIDs = append([]string(nil), input.ItemIDs...)
	sort.Strings(input.ItemIDs)
	expires := time.Now().Add(5 * time.Second)
	if identity.TokenExpiresAt.Before(expires) {
		expires = identity.TokenExpiresAt
	}
	if deadline, ok := ctx.Deadline(); ok && deadline.Before(expires) {
		expires = deadline
	}
	return AuthorizedBatchSelection{scope: scope, input: input, expiresAt: expires}, nil
}

func selectionIdentityMatches(identity authidentity.AuthenticatedIdentity, scope Scope) bool {
	return scope.Validate() == nil && identity.TenantID == scope.OrganizationID && identity.EffectiveOrganizationID == scope.OrganizationID && identity.UserID == scope.ActorID && identity.EffectiveMemberID == scope.MemberID
}

func (s AuthorizedBatchSelection) Read(ctx context.Context) (Scope, BatchSelectionInput, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if ctx == nil || ctx.Err() != nil || !ok || !selectionIdentityMatches(identity, s.scope) || !time.Now().Before(identity.TokenExpiresAt) || !time.Now().Before(s.expiresAt) {
		return Scope{}, BatchSelectionInput{}, ErrForbidden
	}
	input := s.input
	input.ItemIDs = append([]string(nil), input.ItemIDs...)
	return s.scope, input, nil
}

// SelectionReader is injected on the caller's Product transaction. It must
// lock the batch and recheck its revision before yielding the complete fixed
// membership in bounded pages. No Catalog bodies or approval facts are copied.
type SelectionReader interface {
	VisitSelection(context.Context, AuthorizedBatchSelection, func(Item) error) (int64, error)
}
