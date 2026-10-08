package preparation

import (
	"context"
	"errors"
	"sort"
	"task-processor/internal/authidentity"
	"task-processor/internal/product/collection"
	"time"
)

type Service struct {
	repository Repository
	selections CollectionSelections
	auth       Authorizer
}

func NewService(repository Repository, selections CollectionSelections, auth Authorizer) (*Service, error) {
	if repository == nil || selections == nil || auth == nil {
		return nil, ErrUnavailable
	}
	return &Service{repository, selections, auth}, nil
}

func (s *Service) authorize(ctx context.Context, purpose string) (Scope, error) {
	if ctx == nil || ctx.Err() != nil {
		return Scope{}, ErrForbidden
	}
	scope, err := s.auth.Authorize(ctx, purpose)
	if err != nil {
		return Scope{}, err
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || scope.Validate() != nil || identity.TenantID != scope.OrganizationID || identity.EffectiveOrganizationID != scope.OrganizationID || identity.UserID != scope.ActorID || identity.EffectiveMemberID != scope.MemberID || !time.Now().Before(identity.TokenExpiresAt) || ctx.Err() != nil {
		return Scope{}, ErrForbidden
	}
	return scope, nil
}

func (s *Service) Transfer(ctx context.Context, key string, input TransferInput) (TransferReceipt, error) {
	if ctx == nil {
		return TransferReceipt{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, PermissionManage)
	if err != nil {
		return TransferReceipt{}, err
	}
	if !collection.ValidID(key) || input.Validate() != nil {
		return TransferReceipt{}, ErrInvalid
	}
	input.ItemIDs = append([]string(nil), input.ItemIDs...)
	sort.Strings(input.ItemIDs)
	existing, err := s.repository.FindTransfer(ctx, scope, key, input)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return TransferReceipt{}, err
	}
	proof, err := s.selections.SelectBatch(ctx, input)
	if err != nil {
		return TransferReceipt{}, err
	}
	selectedScope, _, err := proof.Read(ctx)
	if err != nil || selectedScope != scope {
		return TransferReceipt{}, ErrForbidden
	}
	return s.repository.Transfer(ctx, TransferCommit{scope: scope, key: key, input: input, proof: proof})
}

func (s *Service) ReadByKey(ctx context.Context, key string) (TransferReceipt, error) {
	if ctx == nil {
		return TransferReceipt{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, PermissionRead)
	if err != nil {
		return TransferReceipt{}, err
	}
	if !collection.ValidID(key) {
		return TransferReceipt{}, ErrInvalid
	}
	return s.repository.ReadByKey(ctx, scope, key)
}

func (s *Service) List(ctx context.Context, query Query) (collection.Page[Preparation], error) {
	if ctx == nil {
		return collection.Page[Preparation]{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, PermissionRead)
	if err != nil {
		return collection.Page[Preparation]{}, err
	}
	if query.Validate() != nil {
		return collection.Page[Preparation]{}, ErrInvalid
	}
	return s.repository.List(ctx, scope, query)
}

func (s *Service) ListSources(ctx context.Context, id string, query Query) (collection.Page[SourceItem], error) {
	if ctx == nil {
		return collection.Page[SourceItem]{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, PermissionRead)
	if err != nil {
		return collection.Page[SourceItem]{}, err
	}
	if !collection.ValidID(id) || query.Validate() != nil {
		return collection.Page[SourceItem]{}, ErrInvalid
	}
	return s.repository.ListSources(ctx, scope, id, query)
}

func (s *Service) Read(ctx context.Context, id string) (Preparation, error) {
	if ctx == nil || !collection.ValidID(id) {
		return Preparation{}, ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, PermissionRead)
	if err != nil {
		return Preparation{}, err
	}
	return s.repository.Read(ctx, scope, id)
}
