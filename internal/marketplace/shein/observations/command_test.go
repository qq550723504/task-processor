package observations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type commandRepository struct {
	Repository
	value Command
}

func (r *commandRepository) CommandByKey(_ context.Context, scope Scope, key string) (Command, error) {
	if r.value.ID == "" || r.value.Owner != scope || r.value.Key != key {
		return Command{}, ErrNotFound
	}
	return r.value, nil
}
func (r *commandRepository) Begin(_ context.Context, scope Scope, key, hash string, in BeginInput, children []Sync) (Command, error) {
	r.value = Command{ID: uuid.NewString(), Owner: scope, Key: key, Hash: hash, Input: in, Syncs: children, CreatedAt: time.Now().UTC()}
	return r.value, nil
}

type commandDirectory struct{ stores []string }

func (d *commandDirectory) ListStores(context.Context, Scope) ([]string, error) {
	return append([]string{}, d.stores...), nil
}

type commandAccess struct {
	Access
	errors map[string]error
}

func (a commandAccess) Authorize(context.Context, Scope, Kind, bool) error { return nil }
func (a commandAccess) Open(_ context.Context, scope Scope, store string, _ Kind, _ bool, _ *Binding) (Merchant, error) {
	if err := a.errors[store]; err != nil {
		return nil, err
	}
	return &serviceMerchant{binding: Binding{OrganizationID: scope.OrganizationID, StoreID: store, ApplicationID: "current"}}, nil
}

type commandStarter func()

func (f commandStarter) Ensure(context.Context, string, string) error { f(); return nil }

func TestCommandProjectsOnlyCurrentChildrenWithoutChangingReceiptOrReplay(t *testing.T) {
	ctx := context.Background()
	scope := Scope{"org-a", "actor-a", "original-member"}
	first, second, key := uuid.NewString(), uuid.NewString(), uuid.NewString()
	input := BeginInput{Kind: Products, Stores: []string{}}
	raw, err := json.Marshal(input)
	require.NoError(t, err)
	hash := sha256.Sum256(raw)
	stored := Command{ID: uuid.NewString(), Owner: scope, Key: key, Hash: hex.EncodeToString(hash[:]), Input: BeginInput{Kind: Products, Stores: []string{first, second}}}
	for _, store := range stored.Input.Stores {
		stored.Syncs = append(stored.Syncs, Sync{ID: uuid.NewString(), StoreID: store, Owner: scope, Kind: Products, Binding: Binding{OrganizationID: scope.OrganizationID, StoreID: store, ApplicationID: "current"}})
	}
	for _, tc := range []struct {
		name   string
		stores []string
		err    error
		fail   error
	}{
		{"directory-revocation", []string{second}, nil, nil},
		{"live-revocation", []string{first, second}, ErrForbidden, nil},
		{"missing-source", []string{first, second}, ErrNotFound, nil},
		{"changed-source", []string{first, second}, ErrConflict, nil},
		{"unsupported-source", []string{first, second}, ErrUnsupported, nil},
		{"temporary-dependency", []string{first, second}, ErrUnavailable, ErrUnavailable},
		{"all-revoked", []string{}, nil, ErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &commandRepository{value: stored}
			s := Service{Repository: r, Directory: &commandDirectory{tc.stores}, Access: commandAccess{errors: map[string]error{first: tc.err}}}
			for _, replay := range []bool{false, true} {
				var result Command
				var err error
				if replay {
					result, err = s.Begin(ctx, scope, key, input)
				} else {
					result, err = s.Command(ctx, scope, key)
				}
				if tc.fail != nil {
					require.ErrorIs(t, err, tc.fail)
					continue
				}
				require.NoError(t, err)
				require.Equal(t, []Sync{stored.Syncs[1]}, result.Syncs)
				require.Equal(t, []string{second}, result.Input.Stores)
				require.Equal(t, stored.ID, result.ID)
				require.Equal(t, stored.Hash, result.Hash)
			}
			require.Equal(t, stored, r.value, "redaction must not rewrite the fixed durable receipt or shared slices")
			replacement := scope
			replacement.MemberID = "replacement-member"
			_, err := s.Command(ctx, replacement, key)
			require.ErrorIs(t, err, ErrNotFound)
		})
	}
}

func TestBeginProjectsLiveChildrenAfterCommitAndWorkflowStartup(t *testing.T) {
	directory := &commandDirectory{stores: []string{uuid.NewString(), uuid.NewString()}}
	remaining := directory.stores[1]
	repo := &commandRepository{}
	var revoke sync.Once
	s := Service{Repository: repo, Directory: directory, Access: commandAccess{}, Starter: commandStarter(func() { revoke.Do(func() { directory.stores = []string{remaining} }) })}
	result, err := s.Begin(context.Background(), Scope{"org-a", "actor-a", "original-member"}, uuid.NewString(), BeginInput{Kind: Products})
	require.NoError(t, err)
	require.Equal(t, []string{remaining}, result.Input.Stores)
	require.Len(t, result.Syncs, 1)
	require.Equal(t, remaining, result.Syncs[0].StoreID)
	require.Len(t, repo.value.Syncs, 2, "the durable command must keep the original selected set")
}

func TestAllStoreCommandProjectsCurrentWindowWithoutChangingDurableReceipt(t *testing.T) {
	scope := Scope{"org-a", "actor-a", "original-member"}
	stored := Command{ID: uuid.NewString(), Owner: scope, Key: uuid.NewString(), Input: BeginInput{Kind: Orders}, CreatedAt: time.Now().UTC()}
	directory := &commandDirectory{}
	for i := 0; i < 500; i++ {
		store := uuid.NewString()
		directory.stores = append(directory.stores, store)
		stored.Input.Stores = append(stored.Input.Stores, store)
		child := Sync{ID: uuid.NewString(), StoreID: store, Owner: scope, Kind: Orders, Binding: Binding{OrganizationID: scope.OrganizationID, StoreID: store, ApplicationID: "current"}, Status: "running", CreatedAt: stored.CreatedAt, Progress: Checkpoint{Page: 1}}
		for j := 0; j < 512; j++ {
			child.Progress.Windows = append(child.Progress.Windows, Window{stored.CreatedAt.Add(-time.Hour), stored.CreatedAt})
		}
		stored.Syncs = append(stored.Syncs, child)
	}
	repo := &commandRepository{value: stored}
	s := Service{Repository: repo, Directory: directory, Access: commandAccess{}}
	result, err := s.Command(context.Background(), scope, stored.Key)
	require.NoError(t, err)
	raw, err := json.Marshal(result)
	require.NoError(t, err)
	require.LessOrEqual(t, len(raw), 2<<20)
	require.Len(t, result.Syncs, 500)
	require.Len(t, result.Syncs[0].Progress.Windows, 1)
	require.Equal(t, stored, repo.value)
}

func TestBeginTransientStoreOpenNeverCommitsTerminalCommandAndSameKeyCanRetry(t *testing.T) {
	ctx := context.Background()
	scope := Scope{"org-a", "actor-a", "original-member"}
	store, key := uuid.NewString(), uuid.NewString()
	repo := &commandRepository{}
	access := commandAccess{errors: map[string]error{store: ErrUnavailable}}
	s := Service{Repository: repo, Directory: &commandDirectory{stores: []string{store}}, Access: access}
	input := BeginInput{Kind: Products, Stores: []string{store}}
	_, err := s.Begin(ctx, scope, key, input)
	require.ErrorIs(t, err, ErrUnavailable)
	require.Empty(t, repo.value.ID, "transient Store/IAM failure must not commit a terminal child or parent")
	delete(access.errors, store)
	result, err := s.Begin(ctx, scope, key, input)
	require.NoError(t, err)
	require.Equal(t, key, result.Key)
	require.Len(t, result.Syncs, 1)
	require.Equal(t, "pending", result.Syncs[0].Status)
	require.Equal(t, key, repo.value.Key)
}
