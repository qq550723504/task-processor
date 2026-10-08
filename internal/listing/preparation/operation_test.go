package preparation

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
)

type operationAuth struct {
	scope  Scope
	denied bool
}

func (a *operationAuth) Authorize(context.Context, string) (Scope, error) {
	if a.denied {
		return Scope{}, ErrForbidden
	}
	return a.scope, nil
}

type operationCollections struct{ collection.Repository }
type operationSources struct {
	sourcing.PublishedAcquisitionReader
}
type operationRepo struct {
	OperationRepository
	values map[string]OperationReceipt
	hashes map[string]string
	writes int
}

func (r *operationRepo) FindOperation(_ context.Context, _ Scope, key, hash string) (OperationReceipt, error) {
	prior, ok := r.values[key]
	if !ok {
		return prior, ErrNotFound
	}
	if r.hashes[key] != hash {
		return OperationReceipt{}, ErrConflict
	}
	prior.Replayed = true
	return prior, nil
}
func (r *operationRepo) PrepareOperation(ctx context.Context, proof OperationCommit) (OperationReceipt, error) {
	scope, key, input, err := proof.Read(ctx)
	if err != nil {
		return OperationReceipt{}, err
	}
	r.writes++
	v := OperationReceipt{Operation: Operation{ID: OperationCommandID(scope, key), Owner: scope, Input: input, Count: 205, Status: OperationPending, CreatedAt: time.Now().UTC()}}
	r.values[key], r.hashes[key] = v, collection.Digest(input)
	return v, nil
}
func TestPreparationOperationRequiresCurrentOwnerAndReplaysItsExactFixedSelection(t *testing.T) {
	scope := Scope{"org-a", "actor-a", "member-a"}
	auth := &operationAuth{scope: scope}
	collections, err := collection.NewService(operationCollections{}, auth, operationSources{})
	require.NoError(t, err)
	base, err := NewService(nil, nil, nil)
	require.ErrorIs(t, err, ErrUnavailable)
	base, err = NewService(&operationBaseRepo{}, collections, auth)
	require.NoError(t, err)
	repo := &operationRepo{values: map[string]OperationReceipt{}, hashes: map[string]string{}}
	service, err := NewOperationService(base, collections, repo)
	require.NoError(t, err)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, UserID: scope.ActorID, EffectiveMemberID: scope.MemberID, TokenExpiresAt: time.Now().Add(time.Minute)})
	key := uuid.NewString()
	input := OperationInput{PreparationID: uuid.NewString(), ExpectedRevision: 1, Action: OperationAdapt, StoreID: uuid.NewString()}
	result, err := service.Create(ctx, key, input)
	require.NoError(t, err)
	require.EqualValues(t, 205, result.Operation.Count)
	require.Equal(t, OperationPending, result.Operation.Status)
	replay, err := service.Create(ctx, key, input)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, 1, repo.writes)
	input.SourceIDs = []string{uuid.NewString()}
	_, err = service.Create(ctx, key, input)
	require.ErrorIs(t, err, ErrConflict)
	auth.denied = true
	_, err = service.Create(ctx, key, input)
	require.ErrorIs(t, err, ErrForbidden)
	_, _, _, err = (OperationCommit{}).Read(ctx)
	require.ErrorIs(t, err, ErrForbidden)
}

type operationBaseRepo struct{ Repository }

func TestOperationInputRejectsDuplicateOrAmbiguousSelections(t *testing.T) {
	input := OperationInput{PreparationID: uuid.NewString(), ExpectedRevision: 1, Action: OperationUpload, StoreID: uuid.NewString()}
	require.NoError(t, input.Validate())
	id := uuid.NewString()
	input.SourceIDs = []string{id, id}
	require.ErrorIs(t, input.Validate(), ErrInvalid)
	input.SourceIDs = nil
	input.TitleTemplateID = "ignored-template"
	require.ErrorIs(t, input.Validate(), ErrInvalid, "upload cannot smuggle an optimization selection")
}
