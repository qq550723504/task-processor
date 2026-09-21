package accountaudit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	allocation "task-processor/internal/accountallocation"
	"task-processor/internal/authidentity"
	registry "task-processor/internal/sourceaccountregistry"
)

type historyStub struct {
	page    registry.HistoryPage
	request registry.HistoryRequest
	calls   int
}

type allocationHistoryStub struct{ page allocation.AuditPage }

func (s *allocationHistoryStub) ListRecentAudit(context.Context, string, int, string, string, *allocation.AuditPosition) (allocation.AuditPage, error) {
	return s.page, nil
}

func (s *historyStub) List(_ context.Context, r registry.HistoryRequest) (registry.HistoryPage, error) {
	s.calls++
	s.request = r
	return s.page, nil
}
func TestProjectionPreservesMicrosecondsAndStringVersion(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	id := "0198d4f0-0000-7000-8000-000000000001"
	receipt := registry.CommittedOperation{OrganizationID: "B", AccountID: id, ActorSubject: "actor", Kind: registry.OperationDisable, Version: 9007199254740993, OccurredAt: now}
	position := receipt.Position()
	source := &historyStub{page: registry.HistoryPage{Items: []registry.CommittedOperation{receipt}, Next: &position}}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "u1", TenantID: "B", EffectiveOrganizationID: "B", TokenExpiresAt: now.Add(time.Hour)})
	query, _ := New(source)
	page, err := query.Read(ctx, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(page)
	if !strings.Contains(string(encoded), `"version":"9007199254740993"`) {
		t.Fatal(string(encoded))
	}
	if page.NextCursor == nil {
		t.Fatal("missing cursor")
	}
	source.page = registry.HistoryPage{}
	if _, err = query.Read(ctx, 1, *page.NextCursor); err != nil {
		t.Fatal(err)
	}
	if !source.request.After.Equal(position) {
		t.Fatalf("position changed: %+v", source.request.After)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(*page.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(decoded), "actor") || strings.Contains(string(decoded), "key") {
		t.Fatalf("unexpected cursor material: %s", decoded)
	}
	other := authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{UserID: "u1", TenantID: "A", EffectiveOrganizationID: "A", TokenExpiresAt: now.Add(time.Hour)})
	count := source.calls
	if _, err = query.Read(other, 1, *page.NextCursor); !errors.Is(err, registry.ErrInvalid) {
		t.Fatal(err)
	}
	if count != source.calls {
		t.Fatal("foreign cursor queried")
	}
}
func TestProjectionRejectsMalformedCursorAndUnsafeSource(t *testing.T) {
	source := &historyStub{}
	query, _ := New(source)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "u1", TenantID: "B", EffectiveOrganizationID: "B", TokenExpiresAt: time.Now().Add(time.Hour)})
	for _, cursor := range []string{"!", strings.Repeat("a", 2049), base64.RawURLEncoding.EncodeToString([]byte(`{"org":"B","extra":1}`))} {
		if _, err := query.Read(ctx, 20, cursor); !errors.Is(err, registry.ErrInvalid) {
			t.Fatal(err)
		}
	}
	if source.calls != 0 {
		t.Fatal("bad cursor queried")
	}
	source.page = registry.HistoryPage{Items: []registry.CommittedOperation{{OrganizationID: "A"}}}
	if _, err := query.Read(ctx, 20, ""); !errors.Is(err, registry.ErrUnavailable) {
		t.Fatal(err)
	}
}

func TestProjectionBindsAuditFiltersToQueryAndCursor(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	id := "0198d4f0-0000-7000-8000-000000000001"
	position := registry.HistoryPosition{OccurredAt: now, AccountID: id, Version: 2}
	source := &historyStub{page: registry.HistoryPage{Items: []registry.CommittedOperation{{OrganizationID: "B", AccountID: id, ActorSubject: "actor", Kind: registry.OperationDisable, Version: 2, OccurredAt: now}}, Next: &position}}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "u1", TenantID: "B", EffectiveOrganizationID: "B", TokenExpiresAt: now.Add(time.Hour)})
	query, _ := New(source)
	filter := Filter{ActorSubject: "actor", Kind: registry.OperationDisable}
	page, err := query.ReadFiltered(ctx, 1, "", filter)
	if err != nil || source.request.ActorSubject != "actor" || source.request.Kind != registry.OperationDisable || page.NextCursor == nil {
		t.Fatalf("filtered read = %#v, err=%v, request=%#v", page, err, source.request)
	}
	if _, err := query.ReadFiltered(ctx, 1, *page.NextCursor, Filter{}); !errors.Is(err, registry.ErrInvalid) {
		t.Fatalf("cursor reused without its filters: %v", err)
	}
}

func TestProjectionEmitsCursorWhenMergedPageTruncatesWithoutSourceCursor(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	history := &historyStub{page: registry.HistoryPage{Items: []registry.CommittedOperation{
		{OrganizationID: "B", AccountID: "0198d4f0-0000-7000-8000-000000000001", ActorSubject: "actor", Kind: registry.OperationDisable, Version: 2, OccurredAt: now},
		{OrganizationID: "B", AccountID: "0198d4f0-0000-7000-8000-000000000002", ActorSubject: "actor", Kind: registry.OperationDisable, Version: 3, OccurredAt: now.Add(-time.Microsecond)},
	}}}
	allocations := &allocationHistoryStub{page: allocation.AuditPage{Items: []allocation.AuditEvent{
		{OrganizationID: "B", ActorID: "actor", MemberID: "m-1", Operation: "set_target", Version: 1, IdempotencyKey: "k-1", CreatedAt: now.Add(-2 * time.Microsecond)},
		{OrganizationID: "B", ActorID: "actor", MemberID: "m-2", Operation: "set_target", Version: 2, IdempotencyKey: "k-2", CreatedAt: now.Add(-3 * time.Microsecond)},
	}}}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "u1", TenantID: "B", EffectiveOrganizationID: "B", TokenExpiresAt: now.Add(time.Hour)})
	query, err := NewWithAllocation(history, allocations)
	if err != nil {
		t.Fatal(err)
	}
	page, err := query.Read(ctx, 2, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.NextCursor == nil {
		t.Fatalf("merged page=%+v, want truncated cursor", page)
	}
}
