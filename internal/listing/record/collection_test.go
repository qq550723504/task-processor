package record

import (
	"context"
	"testing"
	"time"

	listingtask "task-processor/internal/listing/task"

	"github.com/stretchr/testify/require"
)

type collectionReaderFixture struct {
	calls int
	actor listingtask.Actor
	page  Page
	err   error
}

func (f *collectionReaderFixture) List(_ context.Context, actor listingtask.Actor, request PageRequest) (Page, error) {
	f.calls++
	f.actor = actor
	return f.page, f.err
}

func TestCollectionServiceAuthorizesBeforeReader(t *testing.T) {
	for _, fixture := range []struct {
		name string
		ctx  context.Context
		auth authFixture
	}{
		{name: "missing identity", ctx: context.Background(), auth: authFixture{read: true}},
		{name: "missing read", ctx: fixtureContext(), auth: authFixture{read: false}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			reader := &collectionReaderFixture{}
			service, err := NewCollectionService(reader, fixture.auth)
			require.NoError(t, err)
			_, err = service.List(fixture.ctx, PageRequest{Limit: 20})
			require.ErrorIs(t, err, ErrForbidden)
			require.Zero(t, reader.calls)
		})
	}
}

func TestCollectionServicePassesVerifiedEffectiveActor(t *testing.T) {
	created := time.Date(2026, 9, 6, 1, 2, 3, 456000000, time.UTC)
	reader := &collectionReaderFixture{page: Page{Items: []CollectionItem{{ID: "12345678-1234-4234-8234-123456789abc", Input: fixtureInput, CreatedAt: created}}}}
	service, err := NewCollectionService(reader, authFixture{read: true})
	require.NoError(t, err)
	page, err := service.List(fixtureContext(), PageRequest{Limit: 20})
	require.NoError(t, err)
	require.Equal(t, reader.page, page)
	require.Equal(t, listingtask.Actor{TenantID: "B", UserID: "actor", Roles: []string{"role"}}, reader.actor)
}

func TestPageRequestRejectsInvalidBoundsAndCursor(t *testing.T) {
	valid := PageCursor{ID: "12345678-1234-4234-8234-123456789abc", CreatedAt: time.Now().UTC()}
	for _, request := range []PageRequest{
		{Limit: 0}, {Limit: 101}, {Limit: 20, Cursor: &PageCursor{}},
		{Limit: 20, Cursor: &PageCursor{ID: "not-a-uuid", CreatedAt: valid.CreatedAt}},
		{Limit: 20, Cursor: &PageCursor{ID: valid.ID, CreatedAt: time.Time{}}},
	} {
		require.ErrorIs(t, request.Validate(), ErrInvalid)
	}
	require.NoError(t, (PageRequest{Limit: 20, Cursor: &valid}).Validate())
}
