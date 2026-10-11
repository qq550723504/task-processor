package collection

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type readBatchStore struct {
	Repository
	scope Scope
	after func()
}

func (s *readBatchStore) ReadBatch(_ context.Context, scope Scope, id string) (Batch, error) {
	s.scope = scope
	if s.after != nil {
		s.after()
	}
	return Batch{ID: id, Name: "当前分组", Revision: 1}, nil
}

func TestReadBatchUsesOwnerScopeAndRejectsChangedAuthority(t *testing.T) {
	for _, kind := range []string{"read", "revoked", "member-changed", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			scope := Scope{"org-a", "actor-a", "member-a"}
			auth := &testAuth{scope: scope}
			store := &readBatchStore{}
			service, err := NewService(store, auth, testSources{})
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			store.after = func() {
				switch kind {
				case "revoked":
					auth.err = ErrForbidden
				case "member-changed":
					auth.scope.MemberID = "member-b"
				case "cancelled":
					cancel()
				}
			}
			got, err := service.ReadBatch(ctx, uuid.NewString())
			require.Equal(t, scope, store.scope)
			if kind == "read" {
				require.NoError(t, err)
				require.Equal(t, "当前分组", got.Name)
			} else {
				require.Error(t, err)
				require.Empty(t, got)
			}
		})
	}
}
