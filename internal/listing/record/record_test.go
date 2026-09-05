package record

import (
	"context"
	"errors"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	listingtask "task-processor/internal/listing/task"
	"task-processor/internal/product/catalog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type authFixture struct{ read, write bool }

func (a authFixture) Authorize(_ string, _ []string, p string) bool {
	if p == authz.PermissionListingKitAdminRead {
		return a.read
	}
	if p == authz.PermissionListingKitAdminWrite {
		return a.write
	}
	return false
}
func (authFixture) IsTenantAdmin(string, []string) bool { return false }

type sourceFixture struct {
	calls  int
	change bool
	cancel context.CancelFunc
}

func (s *sourceFixture) GetSnapshot(_ context.Context, id catalog.SnapshotIdentity, version uint64) (catalog.PublishedSnapshot, error) {
	s.calls++
	if s.change {
		id.TenantID = "foreign"
	}
	if s.cancel != nil {
		s.cancel()
	}
	return catalog.PublishedSnapshot{Identity: id, Version: version, Snapshot: catalog.ProductSnapshot{Title: "draft"}}, nil
}

type storeFixture struct {
	saved  Record
	writes int
	cancel context.CancelFunc
}

func (s *storeFixture) FindOperation(context.Context, listingtask.Actor, string) (Record, error) {
	if s.cancel != nil {
		s.cancel()
	}
	if s.saved.ID == "" {
		return Record{}, ErrNotFound
	}
	return s.saved.Clone(), nil
}
func (s *storeFixture) Insert(_ context.Context, p Prepared) (Record, error) {
	s.writes++
	s.saved = p.Record()
	return s.saved.Clone(), nil
}

type builderFixture struct {
	calls  int
	fail   error
	cancel context.CancelFunc
}

func (b *builderFixture) Build(context.Context, catalog.ProductSnapshot, Input) ([]byte, error) {
	b.calls++
	if b.cancel != nil {
		b.cancel()
	}
	return []byte(`{"spu_name":"draft"}`), b.fail
}
func fixtureContext() context.Context {
	return authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "B", EffectiveOrganizationID: "B", HomeOrganizationID: "A", UserID: "actor", Roles: []string{"role"}, TokenExpiresAt: time.Now().Add(time.Minute)})
}

var fixtureInput = Input{ProductKey: "product", SnapshotVersion: 1, Country: "US", Language: "en"}

func TestCreateChecksBothPermissionsBeforeAnyProductRead(t *testing.T) {
	for _, a := range []authFixture{{false, true}, {true, false}, {false, false}} {
		t.Run(string(rune('0'+btoi(a.read)*2+btoi(a.write))), func(t *testing.T) {
			source := &sourceFixture{}
			store := &storeFixture{}
			builder := &builderFixture{}
			service, err := NewService(source, store, builder, a)
			require.NoError(t, err)
			_, err = service.Create(fixtureContext(), "op", fixtureInput)
			require.ErrorIs(t, err, ErrForbidden)
			require.Zero(t, source.calls)
			require.Zero(t, store.writes)
		})
	}
}
func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
func TestCreateSourceIntegrityAndCancellationNeverWrite(t *testing.T) {
	for _, phase := range []string{"source identity", "source cancel", "builder cancel", "encoding failure", "already cancelled"} {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithCancel(fixtureContext())
			defer cancel()
			source := &sourceFixture{}
			store := &storeFixture{}
			builder := &builderFixture{}
			switch phase {
			case "source identity":
				source.change = true
			case "source cancel":
				source.cancel = cancel
			case "builder cancel":
				builder.cancel = cancel
			case "encoding failure":
				builder.fail = errors.New("cannot encode")
			case "already cancelled":
				cancel()
			}
			service, err := NewService(source, store, builder, authFixture{true, true})
			require.NoError(t, err)
			_, err = service.Create(ctx, "op", fixtureInput)
			require.Error(t, err)
			require.Zero(t, store.writes)
		})
	}
}
func TestCreateReplayReturnsPersistedCopyWithoutRebuilding(t *testing.T) {
	source := &sourceFixture{}
	store := &storeFixture{}
	builder := &builderFixture{}
	service, err := NewService(source, store, builder, authFixture{true, true})
	require.NoError(t, err)
	first, err := service.Create(fixtureContext(), "op", fixtureInput)
	require.NoError(t, err)
	builder.fail = errors.New("new builder would fail")
	second, err := service.Create(fixtureContext(), "op", fixtureInput)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, 1, builder.calls)
	require.Equal(t, 2, source.calls)
	require.Equal(t, 1, store.writes)
	ctx, cancel := context.WithCancel(fixtureContext())
	store.cancel = cancel
	_, err = service.Create(ctx, "op", fixtureInput)
	require.ErrorIs(t, err, context.Canceled)
}
