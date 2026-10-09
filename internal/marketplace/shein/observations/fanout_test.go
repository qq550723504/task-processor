package observations

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type delayedAccess struct {
	scope        Scope
	delay        time.Duration
	active, peak atomic.Int32
	badScope     atomic.Bool
	openErrors   map[string]error
	checkDenied  atomic.Bool
}

func (a *delayedAccess) wait(ctx context.Context) error {
	active := a.active.Add(1)
	defer a.active.Add(-1)
	for old := a.peak.Load(); active > old && !a.peak.CompareAndSwap(old, active); old = a.peak.Load() {
	}
	timer := time.NewTimer(a.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func (a *delayedAccess) Authorize(ctx context.Context, s Scope, _ Kind, _ bool) error {
	if s != a.scope {
		a.badScope.Store(true)
	}
	return ctx.Err()
}
func (a *delayedAccess) Open(ctx context.Context, s Scope, store string, _ Kind, _ bool, expected *Binding) (Merchant, error) {
	if s != a.scope {
		a.badScope.Store(true)
	}
	if err := a.wait(ctx); err != nil {
		return nil, err
	}
	if err := a.openErrors[store]; err != nil {
		return nil, err
	}
	binding := Binding{OrganizationID: s.OrganizationID, StoreID: store, ApplicationID: "current"}
	if expected != nil && *expected != binding {
		return nil, ErrNotFound
	}
	return &delayedMerchant{access: a, binding: binding}, nil
}

type delayedMerchant struct {
	Merchant
	access  *delayedAccess
	binding Binding
}

func (m *delayedMerchant) Binding() Binding { return m.binding }
func (m *delayedMerchant) Check(ctx context.Context) error {
	if err := m.access.wait(ctx); err != nil {
		return err
	}
	if m.access.checkDenied.Load() {
		return ErrForbidden
	}
	return nil
}

type delayedStarter struct {
	access delayedAccess
	calls  atomic.Int32
}

func (s *delayedStarter) Ensure(ctx context.Context, _, _ string) error {
	for i := 0; i < 2; i++ {
		if err := s.access.wait(ctx); err != nil {
			return err
		}
	}
	s.calls.Add(1)
	return nil
}

func TestAllStoreChecksAndStartsRespectDeadlineAndBoundedFanout(t *testing.T) {
	for _, path := range []string{"begin", "command", "list"} {
		t.Run(path, func(t *testing.T) {
			scope := Scope{"org-a", "actor-a", "original-member"}
			directory := &commandDirectory{}
			stored := Command{ID: uuid.NewString(), Owner: scope, Key: uuid.NewString(), Input: BeginInput{Kind: Products}, CreatedAt: time.Now().UTC()}
			for i := 0; i < 500; i++ {
				store := uuid.NewString()
				directory.stores = append(directory.stores, store)
				stored.Input.Stores = append(stored.Input.Stores, store)
				stored.Syncs = append(stored.Syncs, Sync{ID: uuid.NewString(), StoreID: store, Owner: scope, Kind: Products, Status: "pending", Binding: Binding{OrganizationID: scope.OrganizationID, StoreID: store, ApplicationID: "current"}})
			}
			access := &delayedAccess{scope: scope, delay: 2 * time.Millisecond}
			starter := &delayedStarter{access: delayedAccess{delay: 2 * time.Millisecond}}
			commandRepo := &commandRepository{value: stored}
			service := Service{Repository: commandRepo, Directory: directory, Access: access, Starter: starter}
			ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
			defer cancel()
			switch path {
			case "begin":
				result, err := service.Begin(ctx, scope, uuid.NewString(), BeginInput{Kind: Products})
				require.NoError(t, err)
				require.Len(t, result.Syncs, 500)
				require.Equal(t, int32(500), starter.calls.Load())
				require.LessOrEqual(t, starter.access.peak.Load(), int32(16))
			case "command":
				result, err := service.Command(ctx, scope, stored.Key)
				require.NoError(t, err)
				require.Equal(t, stored.Syncs, result.Syncs, "parallel checks preserve receipt ordering and identities")
				require.Equal(t, stored, commandRepo.value)
			case "list":
				repo := &serviceRepo{heads: stored.Syncs, latest: stored.Syncs}
				service.Repository = repo
				result, err := service.List(ctx, scope, Query{Kind: Products, Limit: 20})
				require.NoError(t, err)
				require.Equal(t, stored.Syncs, result.Syncs)
				require.Equal(t, stored.Syncs, result.Latest)
				access.checkDenied.Store(true)
				_, err = service.List(context.Background(), scope, Query{Kind: Products, Limit: 20})
				require.ErrorIs(t, err, ErrForbidden, "late per-store check still fails closed")
			}
			require.False(t, access.badScope.Load())
			require.Greater(t, access.peak.Load(), int32(1))
			require.LessOrEqual(t, access.peak.Load(), int32(16))
		})
	}
}

func TestConcurrentPreparationDependencyErrorNeverCommits(t *testing.T) {
	scope := Scope{"org-a", "actor-a", "original-member"}
	stores := []string{uuid.NewString(), uuid.NewString(), uuid.NewString()}
	repo := &commandRepository{}
	service := Service{Repository: repo, Directory: &commandDirectory{stores: stores}, Access: &delayedAccess{scope: scope, delay: time.Millisecond, openErrors: map[string]error{stores[1]: ErrUnavailable}}}
	_, err := service.Begin(context.Background(), scope, uuid.NewString(), BeginInput{Kind: Products})
	require.ErrorIs(t, err, ErrUnavailable)
	require.Empty(t, repo.value.ID)
}
