package sourceaccountregistry

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/authz"
)

type historyReaderStub struct {
	page  HistoryPage
	err   error
	calls int
	org   string
}

func (r *historyReaderStub) ListCommittedOperations(_ context.Context, org string, _ HistoryRequest) (HistoryPage, error) {
	r.calls++
	r.org = org
	return r.page, r.err
}
func TestHistoryUsesExistingReadAuthorizationAndExactOrganization(t *testing.T) {
	now := time.Now().UTC()
	reader := &historyReaderStub{}
	auth, _ := authz.NewListingKitAuthorizer(nil, nil)
	source := newTestService(t, &fakeStore{}, auth, now)
	history, err := NewHistoryService(source, reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"listingkit_viewer", "listingkit_operator", "listingkit_admin"} {
		got, err := history.List(identityContext(now, "B", "actor", role), HistoryRequest{Limit: 20})
		if err != nil || got.Items == nil || reader.org != "B" {
			t.Fatalf("role=%s page=%+v err=%v", role, got, err)
		}
	}
	before := reader.calls
	for _, ctx := range []context.Context{context.Background(), identityContext(now, "B", "actor", "no-role")} {
		if _, err := history.List(ctx, HistoryRequest{Limit: 20}); err == nil {
			t.Fatal("unauthorized history allowed")
		}
	}
	if reader.calls != before {
		t.Fatal("queried before authorization")
	}
}
func TestHistoryConfiguredSubjectAuthority(t *testing.T) {
	now := time.Now().UTC()
	reader := &historyReaderStub{}
	for _, configured := range []bool{true, false} {
		var users []string
		if configured {
			users = []string{"configured-user"}
		}
		authorizer, err := authz.NewListingKitAuthorizer(users, nil)
		if err != nil {
			t.Fatal(err)
		}
		history, err := NewHistoryService(newTestService(t, &fakeStore{}, authorizer, now), reader)
		if err != nil {
			t.Fatal(err)
		}
		before := reader.calls
		_, err = history.List(identityContext(now, "B", "configured-user", "custom_member"), HistoryRequest{Limit: 20})
		if configured {
			if err != nil || reader.org != "B" || reader.calls != before+1 {
				t.Fatalf("configured subject history: org=%s calls=%d err=%v", reader.org, reader.calls, err)
			}
		} else if !errors.Is(err, ErrForbidden) || reader.calls != before {
			t.Fatalf("removed configuration reached reader: %v", err)
		}
	}
}

func TestHistoryRejectsBoundsForeignAndUnorderedSourceFacts(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	account := validAccount(t, now)
	receipt := CommittedOperation{OrganizationID: "B", AccountID: account.ID, ActorSubject: "actor", Kind: OperationRegister, Version: 1, OccurredAt: now}
	reader := &historyReaderStub{page: HistoryPage{Items: []CommittedOperation{receipt}}}
	source := newTestService(t, &fakeStore{}, allowAllAuthorizer(), now)
	history, _ := NewHistoryService(source, reader)
	ctx := identityContext(now, "B", "actor", "listingkit_viewer")
	for _, limit := range []int{-1, 0, 101} {
		if _, err := history.List(ctx, HistoryRequest{Limit: limit}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("limit %d: %v", limit, err)
		}
	}
	if reader.calls != 0 {
		t.Fatal("invalid request queried")
	}
	if _, err := history.List(ctx, HistoryRequest{Limit: 20}); err != nil {
		t.Fatal(err)
	}
	reader.page.Items[0].OrganizationID = "A"
	if _, err := history.List(ctx, HistoryRequest{Limit: 20}); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	reader.page.Items = []CommittedOperation{receipt, receipt}
	if _, err := history.List(ctx, HistoryRequest{Limit: 20}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("duplicate accepted: %v", err)
	}
	reader.page = HistoryPage{}
	reader.err = ErrUnavailable
	if _, err := history.List(ctx, HistoryRequest{Limit: 20}); !errors.Is(err, ErrUnavailable) {
		t.Fatal("failure became empty")
	}
}
