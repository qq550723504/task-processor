package projectcenter

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
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
	projects   []Project
	gets       int
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
	r.gets++
	return r.p, r.refs, nil
}
func (r *repositorySpy) List(context.Context, Scope, Query) ([]Project, string, error) {
	return r.projects, "", nil
}

func TestListBoundsSourceCallsAndNeverResolvesDetailOnlyReferences(t *testing.T) {
	scope := Scope{"org-a", "user-a"}
	id := "a1111111-1111-4111-8111-111111111111"
	repo := &repositorySpy{p: Project{ID: id, Scope: scope}}
	for i := 0; i < 20; i++ {
		repo.projects = append(repo.projects, repo.p)
	}
	for i := 0; i < 100; i++ {
		kind := "CONVERSATION"
		if i < 20 {
			kind = "BUSINESS_TASK"
		}
		repo.refs = append(repo.refs, Reference{SlotID: id, Kind: kind, TargetID: id})
	}
	reader := &cardReader{}
	checks := 0
	service := Service{Store: repo, Reader: reader, Authorize: func(context.Context, Scope, bool) error { checks++; return nil }}
	page, e := service.List(context.Background(), scope, Query{Mode: "active"})
	require.NoError(t, e)
	require.Len(t, page.Projects, 20)
	require.LessOrEqual(t, reader.calls, 20)
	require.Zero(t, reader.detailCalls)
	require.Equal(t, 1, checks)
	require.Equal(t, 20, page.Projects[0].TaskTotal)
	require.True(t, page.Projects[0].TaskSummaryAvailable)
	require.False(t, page.Projects[19].TaskSummaryAvailable)
	require.Zero(t, page.Projects[19].TaskCompleted)
	require.Empty(t, page.Projects[0].References)
}

type cardReader struct{ calls, detailCalls int }

func (r *cardReader) Resolve(_ context.Context, _ Scope, ref Reference) (ReferenceView, error) {
	r.calls++
	if ref.Kind != "BUSINESS_TASK" && ref.Kind != "STORE" {
		r.detailCalls++
	}
	return ReferenceView{Kind: ref.Kind, TargetID: ref.TargetID, Available: true, Title: "task", Href: "/workbench/ai/tasks/" + ref.TargetID, TaskState: "COMPLETED"}, nil
}

func TestListSourceDeadlineKeepsTheLocalCardsReadable(t *testing.T) {
	scope := Scope{"org-a", "user-a"}
	id := "a1111111-1111-4111-8111-111111111111"
	repo := &repositorySpy{p: Project{ID: id, Scope: scope}, refs: []Reference{{Kind: "BUSINESS_TASK", TargetID: id}}}
	for i := 0; i < 20; i++ {
		repo.projects = append(repo.projects, repo.p)
	}
	reader := &blockedCardReader{}
	service := Service{Store: repo, Reader: reader, Authorize: func(context.Context, Scope, bool) error { return nil }}
	started := time.Now()
	page, e := service.List(context.Background(), scope, Query{Mode: "active"})
	require.NoError(t, e)
	require.Less(t, time.Since(started), 3*time.Second)
	require.Equal(t, 1, reader.calls)
	require.Len(t, page.Projects, 20)
	for _, card := range page.Projects {
		require.False(t, card.TaskSummaryAvailable)
	}
}

type blockedCardReader struct{ calls int }

func (r *blockedCardReader) Resolve(ctx context.Context, _ Scope, _ Reference) (ReferenceView, error) {
	r.calls++
	<-ctx.Done()
	return ReferenceView{}, ctx.Err()
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
