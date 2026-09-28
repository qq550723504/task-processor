package inviteflow

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"sync"
	"sync/atomic"
	"task-processor/internal/authidentity"
	"testing"
	"time"
)

type testStore struct {
	mu  sync.Mutex
	inv Invitation
}

func (t *testStore) Read(context.Context, string) (Invitation, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.inv, nil
}
func (t *testStore) Change(_ context.Context, _ string, rev int64, c Change) (Invitation, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if rev != t.inv.Revision {
		return t.inv, ErrConflict
	}
	inv, err := Transition(t.inv, c)
	if err == nil {
		t.inv = inv
	}
	return inv, err
}
func (t *testStore) Create(context.Context, Invitation) (Invitation, bool, error) {
	return Invitation{}, false, ErrUnavailable
}
func (t *testStore) List(context.Context, string, int, int) (Page, error) {
	return Page{}, ErrUnavailable
}
func (t *testStore) ClaimDelivery(context.Context, string, string, time.Time) (Invitation, error) {
	return Invitation{}, ErrUnavailable
}
func (t *testStore) FinishDelivery(context.Context, string, string, string, time.Time) (Invitation, error) {
	return Invitation{}, ErrUnavailable
}
func fixture() (*Service, context.Context, *testStore, *atomic.Int32) {
	now := time.Now().UTC()
	t := &testStore{inv: Invitation{ID: uuid.NewString(), ProjectID: "p", OrganizationID: "org", CreatorID: "admin", Contact: "recipient@example.test", Role: "listingkit_viewer", State: Pending, Revision: 1, ExpiresAt: now.Add(time.Hour)}}
	calls := &atomic.Int32{}
	verified := true
	email := "recipient@example.test"
	s := New("p", Dependencies{Store: t, RoleAllowed: func(string) bool { return true }, ReadSelf: func(context.Context) (authidentity.SelfProfile, error) {
		return authidentity.SelfProfile{UserID: "recipient", Email: &email, EmailVerified: &verified}, nil
	}, Authorize: func(string, []string, string) bool { return true }, Manager: func(context.Context, string) (authidentity.AuthenticatedIdentity, error) {
		return authidentity.AuthenticatedIdentity{UserID: "admin", EffectiveOrganizationID: "org"}, nil
	}, ReadGrant: func(_ context.Context, _ string, user string) (Grant, error) {
		if user == "admin" {
			return Grant{Found: true, State: "active", Roles: []string{"listingkit_admin"}}, nil
		}
		if calls.Load() > 0 {
			return Grant{Found: true, State: "active", ID: "grant", Roles: []string{"listingkit_viewer"}}, nil
		}
		return Grant{}, nil
	}, WriteGrant: func(context.Context, Invitation) error { calls.Add(1); return errors.New("ACK lost") }})
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "recipient", TokenExpiresAt: now.Add(time.Hour)})
	return s, ctx, t, calls
}
func TestConcurrentAcceptanceAndLostACKOnlyOneWrite(t *testing.T) {
	s, ctx, store, calls := fixture()
	id := store.inv.ID
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() { _, _ = s.Accept(ctx, id) })
	}
	wg.Wait()
	inv, err := s.ReadRecipient(ctx, store.inv.ID)
	if err != nil || inv.State != Accepted || calls.Load() != 1 {
		t.Fatalf("state=%s calls=%d err=%v", inv.State, calls.Load(), err)
	}
}
func TestUnknownAcceptanceNeverRedispatches(t *testing.T) {
	s, ctx, store, calls := fixture()
	s.ReadGrant = func(_ context.Context, _ string, u string) (Grant, error) {
		if u == "admin" {
			return Grant{Found: true, State: "active"}, nil
		}
		return Grant{}, nil
	}
	for range 3 {
		inv, err := s.Accept(ctx, store.inv.ID)
		if err != nil || inv.State != Accepting {
			t.Fatalf("%s %v", inv.State, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatal(calls.Load())
	}
}
func TestAcceptanceRequiresVerifiedContactAndActiveCreator(t *testing.T) {
	for _, kind := range []string{"email", "unverified", "revoked", "existing"} {
		t.Run(kind, func(t *testing.T) {
			s, ctx, store, calls := fixture()
			switch kind {
			case "email", "unverified":
				email := "foreign@example.test"
				v := kind == "email"
				s.ReadSelf = func(context.Context) (authidentity.SelfProfile, error) {
					return authidentity.SelfProfile{UserID: "recipient", Email: &email, EmailVerified: &v}, nil
				}
			case "revoked":
				s.ReadGrant = func(context.Context, string, string) (Grant, error) {
					return Grant{Found: true, State: "inactive"}, nil
				}
			case "existing":
				s.ReadGrant = func(context.Context, string, string) (Grant, error) { return Grant{Found: true, State: "active"}, nil }
			}
			if _, err := s.Accept(ctx, store.inv.ID); err == nil {
				t.Fatal("accepted")
			}
			if calls.Load() != 0 || store.inv.State != Pending {
				t.Fatal("unexpected effect")
			}
		})
	}
}
func TestCancelReplayKeepsOriginal(t *testing.T) {
	s, ctx, store, _ := fixture()
	first, err := s.Cancel(ctx, store.inv.ID)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := s.Cancel(ctx, store.inv.ID)
	if err != nil || replay.Revision != first.Revision {
		t.Fatalf("replay %v %v", replay, err)
	}
}
