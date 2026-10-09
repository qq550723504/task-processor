package projectcenter

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFieldsAcceptBusinessGoalAndRejectInvalidCalendarAndInventedType(t *testing.T) {
	f := Fields{Title: "夏季女装开发", Goal: "整理已保存商品并确认开发方向", Kind: "PRODUCT_DEVELOPMENT", DueDate: "2026-12-10"}
	require.NoError(t, ValidateFields(f))
	f.DueDate = "2026-02-30"
	require.ErrorIs(t, ValidateFields(f), ErrInvalid)
	f.DueDate = ""
	f.Kind = "AUTONOMOUS_WORKFLOW"
	require.ErrorIs(t, ValidateFields(f), ErrInvalid)
}

type repositorySpy struct {
	replay     bool
	validation error
	commits    int
	p          Project
	refs       []Reference
}

func (r *repositorySpy) Replay(context.Context, Scope, string, Command) (Receipt, bool, error) {
	return Receipt{ID: r.p.ID, Revision: 1, Replayed: r.replay}, r.replay, nil
}
func (r *repositorySpy) Commit(_ context.Context, _ Scope, _ string, _ Command, e error) (Receipt, error) {
	r.commits++
	r.validation = e
	return Receipt{}, e
}
func (r *repositorySpy) Get(context.Context, Scope, string) (Project, []Reference, error) {
	return r.p, r.refs, nil
}
func (r *repositorySpy) List(context.Context, Scope, Query) ([]Project, string, error) {
	return nil, "", nil
}
func (r *repositorySpy) Templates(context.Context, Scope, string) ([]Template, string, error) {
	return nil, "", nil
}

type deniedReferences struct{ calls int }

func (r *deniedReferences) Resolve(context.Context, Scope, Reference) (ReferenceView, error) {
	r.calls++
	return ReferenceView{Available: true, Title: "protected title", TargetID: "protected-id", Href: "/workbench/secret"}, ErrForbidden
}
func TestReplayPrecedesRevokedReferenceAndProjectsDoNotLeakTargetMetadata(t *testing.T) {
	scope := Scope{"org-a", "user-a"}
	id := "a1111111-1111-4111-8111-111111111111"
	target := "b1111111-1111-4111-8111-111111111111"
	repo := &repositorySpy{replay: true, p: Project{ID: id, Scope: scope, StoreID: target}}
	reader := &deniedReferences{}
	service := Service{Store: repo, Reader: reader, Authorize: func(context.Context, Scope, bool) error { return nil }}
	receipt, e := service.Execute(context.Background(), scope, target, Command{Operation: "add", ID: id, Expected: 1, Reference: &Reference{Kind: "CONVERSATION", TargetID: target}})
	require.NoError(t, e)
	require.True(t, receipt.Replayed)
	require.Zero(t, reader.calls)
	require.Zero(t, repo.commits)
	repo.refs = []Reference{{SlotID: id, Kind: "CONVERSATION", TargetID: target}}
	view, e := service.Get(context.Background(), scope, id)
	require.NoError(t, e)
	require.False(t, view.Store.Available)
	require.Empty(t, view.Store.TargetID)
	require.Empty(t, view.Store.Title)
	require.Equal(t, id, view.References[0].SlotID)
	require.Empty(t, view.References[0].Href)
	require.Empty(t, view.References[0].TargetID)
	repo.p.Scope.ActorID = "another-user"
	_, e = service.Get(context.Background(), scope, id)
	require.ErrorIs(t, e, ErrUnavailable)
}
func TestMissingReaderIsNotFalseSuccessAndDeniedProjectNeverCallsStore(t *testing.T) {
	scope := Scope{"org-a", "user-a"}
	id := "a1111111-1111-4111-8111-111111111111"
	repo := &repositorySpy{}
	s := Service{Store: repo, Authorize: func(context.Context, Scope, bool) error { return nil }}
	_, e := s.Execute(context.Background(), scope, id, Command{Operation: "add", ID: id, Expected: 1, Reference: &Reference{Kind: "BUSINESS_TASK", TargetID: id}})
	require.ErrorIs(t, e, ErrUnavailable)
	require.Equal(t, 1, repo.commits)
	s.Authorize = func(context.Context, Scope, bool) error { return ErrForbidden }
	_, e = s.Get(context.Background(), scope, id)
	require.ErrorIs(t, e, ErrForbidden)
}
