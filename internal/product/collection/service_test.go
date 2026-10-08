package collection

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/product/sourcing"
)

type testAuth struct {
	scope Scope
	err   error
	calls []string
}

func (a *testAuth) Authorize(_ context.Context, permission string) (Scope, error) {
	a.calls = append(a.calls, permission)
	return a.scope, a.err
}

type testStore struct {
	Repository
	commands []Command
}

func (s *testStore) Execute(_ context.Context, command Command) (Receipt, error) {
	s.commands = append(s.commands, command)
	return Receipt{OperationID: command.OperationID}, nil
}

type testSources struct {
	result sourcing.PublishedAcquisition
	err    error
}

func (s testSources) ReadPublished(context.Context, string) (sourcing.PublishedAcquisition, error) {
	return s.result, s.err
}

func TestRevokedMemberCannotMutateOrReadCachedSource(t *testing.T) {
	auth := &testAuth{err: ErrForbidden}
	store := &testStore{}
	service, err := NewService(store, auth, testSources{})
	require.NoError(t, err)
	_, err = service.Mutate(context.Background(), uuid.NewString(), Mutation{Action: "create_batch", Name: "运营批次"})
	require.ErrorIs(t, err, ErrForbidden)
	require.Empty(t, store.commands)
}

func TestCommandRejectsFieldsFromOtherActions(t *testing.T) {
	store := &testStore{}
	service, err := NewService(store, &testAuth{scope: Scope{"org", "actor", "member"}}, testSources{})
	require.NoError(t, err)
	for _, input := range []Mutation{
		{Action: "create_batch", Name: "批次", TargetBatchID: uuid.NewString()},
		{Action: "rename_batch", BatchID: uuid.NewString(), Name: "批次", ExpectedRevision: 1, Product: &OwnProduct{Title: "不应处理"}},
		{Action: "archive_batch", BatchID: uuid.NewString(), ExpectedRevision: 1, Name: "隐藏字段"},
		{Action: "move_item", ItemID: uuid.NewString(), TargetBatchID: uuid.NewString(), ExpectedRevision: 1, BatchID: uuid.NewString()},
		{Action: "archive_item", ItemID: uuid.NewString(), ExpectedRevision: 1, TargetBatchID: uuid.NewString()},
		{Action: "create_product", Product: &OwnProduct{Title: "商品"}, ExpectedRevision: 1},
	} {
		_, err := service.Mutate(context.Background(), uuid.NewString(), input)
		require.ErrorIs(t, err, ErrInvalid, input.Action)
	}
	require.Empty(t, store.commands)
}

func TestAcquisitionSelectionCannotExposeAnotherActor(t *testing.T) {
	scope := Scope{OrganizationID: "org-a", ActorID: "actor-a", MemberID: "member-a"}
	result := sourcing.PublishedAcquisition{Result: sourcing.AcquisitionResult{Operation: sourcing.AcquisitionOperation{ID: uuid.NewString(), Scope: sourcing.PublicationScope{OrganizationID: scope.OrganizationID, ActorID: "actor-b"}}}}
	store := &testStore{}
	service, err := NewService(store, &testAuth{scope: scope}, testSources{result: result})
	require.NoError(t, err)
	_, err = service.Mutate(context.Background(), uuid.NewString(), Mutation{Action: "add_acquisition", SourceOperationID: result.Result.Operation.ID})
	require.ErrorIs(t, err, ErrNotFound)
	require.Empty(t, store.commands)
}

func TestCommandReplayIdentityBindsScopeAndExactPayload(t *testing.T) {
	auth := &testAuth{scope: Scope{OrganizationID: "org-a", ActorID: "actor-a", MemberID: "member-a"}}
	store := &testStore{}
	service, err := NewService(store, auth, testSources{})
	require.NoError(t, err)
	key := uuid.NewString()
	for _, name := range []string{"一批", "一批", "二批"} {
		_, err = service.Mutate(context.Background(), key, Mutation{Action: "create_batch", Name: name})
		require.NoError(t, err)
	}
	require.Equal(t, store.commands[0].OperationID, store.commands[1].OperationID)
	require.Equal(t, store.commands[0].InputHash, store.commands[1].InputHash)
	require.NotEqual(t, store.commands[0].InputHash, store.commands[2].InputHash)
	auth.scope.ActorID = "actor-b"
	_, err = service.Mutate(context.Background(), key, Mutation{Action: "create_batch", Name: "一批"})
	require.NoError(t, err)
	require.NotEqual(t, store.commands[0].OperationID, store.commands[3].OperationID)
}

func TestOwnProductIsUserEvidenceAndKeepsImagesUnapproved(t *testing.T) {
	input := OwnProduct{Title: "衣架", Description: "用户资料", Images: []string{"https://cdn.example.test/hanger.jpg"}}
	envelope, err := OwnEnvelope(uuid.NewString(), input)
	require.NoError(t, err)
	require.Equal(t, "user_input", envelope.Identity.SourceType)
	require.Equal(t, input.Title, envelope.ProductCandidate.Title)
	require.Len(t, envelope.AssetCandidates, 1)
	require.Equal(t, input.Images[0], envelope.AssetCandidates[0].URL)
	require.NotEmpty(t, envelope.RawReference.Checksum)
	_, err = OwnEnvelope(uuid.NewString(), OwnProduct{Title: "衣架", Images: []string{"file:///etc/passwd"}})
	require.ErrorIs(t, err, ErrInvalid)
	require.False(t, errors.Is(err, ErrUnavailable))
}
