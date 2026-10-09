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

func TestOptimizationPinsTemplateRevisionAndConfirmedQuote(t *testing.T) {
	input := OperationInput{PreparationID: uuid.NewString(), StoreID: uuid.NewString(), ExpectedRevision: 1, Action: OperationOptimize, TitleTemplateID: uuid.NewString()}
	require.ErrorIs(t, input.Validate(), ErrInvalid)
	input.TitleTemplateRevision = "1"
	input.TitleQuoteHash = collection.Digest("quote")
	require.NoError(t, input.Validate())
	input.TitleTemplateRevision = "01"
	require.ErrorIs(t, input.Validate(), ErrInvalid)
	input.TitleTemplateRevision = "1"
	input.TitleQuoteHash = "unknown"
	require.ErrorIs(t, input.Validate(), ErrInvalid)
}

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

type unavailableExecution struct{}

func (unavailableExecution) AuthorizeOperationExecution(context.Context, Operation) error {
	return collection.ErrUnavailable
}

func (unavailableExecution) AuthorizeExecution(context.Context, collection.Scope, string) error {
	return collection.ErrUnavailable
}

type agentGrantRevoked struct{}

func (a agentGrantRevoked) AuthorizeOperationExecution(ctx context.Context, op Operation) error {
	if op.Input.Action == OperationOptimize {
		return a.AuthorizeExecution(ctx, op.Owner, "workbench.agent.use")
	}
	return nil
}
func (agentGrantRevoked) AuthorizeExecution(_ context.Context, _ collection.Scope, purpose string) error {
	if purpose == "workbench.agent.use" {
		return collection.ErrForbidden
	}
	return nil
}
func TestOperationOptimizeChecksItsAgentModuleGrant(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	proof := OperationAccess{service: &OperationService{execution: agentGrantRevoked{}}, operation: Operation{Owner: Scope{"org-a", "actor-a", "member-a"}, Input: OperationInput{Action: OperationOptimize}}, worker: true, expiresAt: time.Now().Add(time.Second)}
	_, err := proof.Read(ctx)
	require.ErrorIs(t, err, ErrForbidden, "Supply grants cannot substitute for the operation's Agent grant")
}
func TestOperationWorkerRetainsAuthorizationDependencyFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	proof := OperationAccess{service: &OperationService{execution: unavailableExecution{}}, operation: Operation{Owner: Scope{"org-a", "actor-a", "member-a"}}, worker: true, expiresAt: time.Now().Add(time.Second)}
	_, err := proof.Read(ctx)
	require.ErrorIs(t, err, ErrUnavailable, "unavailable IAM must never authorize permanent denied settlement")
}

type stopExecutionAuth struct{ err error }

func (a *stopExecutionAuth) AuthorizeOperationExecution(context.Context, Operation) error {
	return a.err
}

func (a *stopExecutionAuth) AuthorizeExecution(context.Context, collection.Scope, string) error {
	return a.err
}

type stopOperationRepo struct {
	OperationRepository
	op     Operation
	writes int
}

func (r *stopOperationRepo) ReadExecutionOperation(context.Context, string, string) (Operation, error) {
	return r.op, nil
}
func (r *stopOperationRepo) StopOperation(ctx context.Context, proof OperationStopAccess, _ string) (OperationItem, error) {
	op, err := proof.Read(ctx)
	if err != nil {
		return OperationItem{}, err
	}
	if op.Owner != r.op.Owner || op.ID != r.op.ID {
		return OperationItem{}, ErrConflict
	}
	r.writes++
	return OperationItem{}, nil
}
func TestOperationStopRequiresWorkerAndFreshAuthoritativeDenial(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	repo := &stopOperationRepo{op: Operation{ID: uuid.NewString(), Owner: Scope{"org-a", "actor-a", "member-a"}, Input: OperationInput{PreparationID: uuid.NewString(), ExpectedRevision: 1, StoreID: uuid.NewString(), Action: OperationAdapt}}}
	auth := &stopExecutionAuth{err: collection.ErrUnavailable}
	service := &OperationService{repository: repo, execution: auth}
	_, err := service.StopExecution(ctx, "org-a", repo.op.ID, "")
	require.ErrorIs(t, err, ErrUnavailable)
	require.Zero(t, repo.writes)
	auth.err = nil
	_, err = service.StopExecution(ctx, "org-a", repo.op.ID, "")
	require.ErrorIs(t, err, ErrConflict, "a currently allowed worker cannot mint a denial proof")
	auth.err = collection.ErrForbidden
	request := authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{UserID: "actor-a"})
	_, err = service.StopExecution(request, "org-a", repo.op.ID, "")
	require.ErrorIs(t, err, ErrForbidden, "HTTP request authority cannot consume the worker stop path")
	_, err = (OperationStopAccess{}).Read(ctx)
	require.ErrorIs(t, err, ErrForbidden)
	_, err = service.StopExecution(ctx, "org-a", repo.op.ID, "")
	require.NoError(t, err)
	require.Equal(t, 1, repo.writes)
}
