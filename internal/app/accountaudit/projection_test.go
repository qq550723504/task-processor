package accountaudit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/ledger/orgresource"
	registry "task-processor/internal/sourceaccountregistry"
)

type historyStub struct {
	page    registry.HistoryPage
	request registry.HistoryRequest
	calls   int
}

type additionalHistoryStub struct{ page AdditionalAuditPage }
type usageHistoryStub struct {
	events []UsageAuditEvent
	calls  int
}
type pagingSourceHistory struct{ events []registry.CommittedOperation }
type failSecondHistory struct {
	first registry.HistoryPage
	calls int
}

func (s *failSecondHistory) List(_ context.Context, _ registry.HistoryRequest) (registry.HistoryPage, error) {
	s.calls++
	if s.calls == 1 {
		return s.first, nil
	}
	return registry.HistoryPage{}, registry.ErrUnavailable
}

func (s pagingSourceHistory) List(_ context.Context, request registry.HistoryRequest) (registry.HistoryPage, error) {
	page := registry.HistoryPage{Items: []registry.CommittedOperation{}}
	for _, item := range s.events {
		if request.After != nil && (item.OccurredAt.After(request.After.OccurredAt) || item.OccurredAt.Equal(request.After.OccurredAt) && item.AccountID >= request.After.AccountID) {
			continue
		}
		if len(page.Items) == request.Limit {
			p := page.Items[len(page.Items)-1].Position()
			page.Next = &p
			break
		}
		page.Items = append(page.Items, item)
	}
	return page, nil
}

func TestProjectionInterleavesSourceAndUsageAcrossCursorsWithoutLoss(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	source := pagingSourceHistory{events: []registry.CommittedOperation{
		{OrganizationID: "B", AccountID: "0198d4f0-0000-7000-8000-000000000004", ActorSubject: "actor", Kind: registry.OperationDisable, Version: 2, OccurredAt: now},
		{OrganizationID: "B", AccountID: "0198d4f0-0000-7000-8000-000000000002", ActorSubject: "actor", Kind: registry.OperationDisable, Version: 2, OccurredAt: now.Add(-2 * time.Second)},
	}}
	usage := &usageHistoryStub{events: []UsageAuditEvent{
		{OrganizationID: "B", EventID: "e-3", MemberID: "m", InvocationID: "inv-3", Quantity: 7, Time: now.Add(-time.Second)},
		{OrganizationID: "B", EventID: "e-1", MemberID: "m", InvocationID: "inv-1", Quantity: 8, Time: now.Add(-3 * time.Second)},
	}}
	query, _ := NewCurrentAuditSources(source, nil, nil, usage, nil, nil)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "u1", TenantID: "B", EffectiveOrganizationID: "B", TokenExpiresAt: now.Add(time.Hour)})
	cursor, seen := "", []string{}
	for i := 0; i < 4; i++ {
		page, err := query.Read(ctx, 1, cursor)
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("page %d = %+v err=%v", i, page, err)
		}
		seen = append(seen, page.Items[0].Relation.Reference)
		if i < 3 {
			if page.NextCursor == nil {
				t.Fatalf("missing cursor at %d", i)
			}
			cursor = *page.NextCursor
		} else if page.NextCursor != nil {
			t.Fatalf("extra cursor: %s", *page.NextCursor)
		}
	}
	want := "0198d4f0-0000-7000-8000-000000000004,e-3,0198d4f0-0000-7000-8000-000000000002,e-1"
	if strings.Join(seen, ",") != want {
		t.Fatalf("seen %v, want %s", seen, want)
	}
}

func TestFilteredProjectionScansSparseUsageAcrossBatchesWithoutFalseEmptyPage(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	items := make([]UsageAuditEvent, 0, 156)
	for index := 0; index < 156; index++ {
		member := "other"
		if index == 112 || index == 154 {
			member = "target"
		}
		items = append(items, UsageAuditEvent{OrganizationID: "B", EventID: strconv.Itoa(200 - index), MemberID: member, InvocationID: "inv-" + strconv.Itoa(index), Quantity: 7, Time: now.Add(-time.Duration(index+1) * time.Second)})
	}
	source := pagingSourceHistory{events: []registry.CommittedOperation{{OrganizationID: "B", AccountID: "0198d4f0-0000-7000-8000-000000000004", ActorSubject: "target", Kind: registry.OperationDisable, Version: 2, OccurredAt: now}}}
	usage := &usageHistoryStub{events: items}
	query, err := NewCurrentAuditSources(source, &additionalHistoryStub{}, &additionalHistoryStub{}, usage, pointHistoryStub{}, &additionalHistoryStub{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "u1", TenantID: "B", EffectiveOrganizationID: "B", TokenExpiresAt: now.Add(time.Hour)})
	filter := Filter{Content: "模型实际用量", MemberID: "target", Period: "all"}
	first, err := query.ReadFiltered(ctx, 1, "", filter)
	if err != nil || len(first.Items) != 1 || first.Items[0].ObjectReference != "inv-112" || first.NextCursor == nil {
		t.Fatalf("first sparse page=%+v err=%v", first, err)
	}
	if _, err := query.ReadFiltered(ctx, 1, *first.NextCursor, Filter{Content: "模型实际用量", MemberID: "other", Period: "all"}); !errors.Is(err, registry.ErrInvalid) {
		t.Fatalf("changed member accepted old cursor: %v", err)
	}
	if _, err := query.ReadFiltered(ctx, 1, *first.NextCursor, Filter{Content: "模型实际用量", MemberID: "target", Period: "7d"}); !errors.Is(err, registry.ErrInvalid) {
		t.Fatalf("changed period accepted old cursor: %v", err)
	}
	second, err := query.ReadFiltered(ctx, 1, *first.NextCursor, filter)
	if err != nil || len(second.Items) != 1 || second.Items[0].ObjectReference != "inv-154" || second.NextCursor != nil {
		t.Fatalf("second sparse page=%+v err=%v", second, err)
	}
	actorOnly, err := query.ReadFiltered(ctx, 20, "", Filter{Content: "停用源账号", MemberID: "target", Period: "all"})
	if err != nil || len(actorOnly.Items) != 0 || actorOnly.NextCursor != nil {
		t.Fatalf("actor misread as affected member: %+v err=%v", actorOnly, err)
	}
}

func TestFilteredProjectionUsesCurrentOwnerTimeAndDisplayedResourceContent(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	resource := &additionalHistoryStub{page: AdditionalAuditPage{Items: []AdditionalAuditEvent{{EventType: "account_member_resource.changed", ObjectType: "member_resource", ObjectReference: "member-1", Actor: "actor", Operation: "allocate_member_resource", Version: 1, Key: "1", RelationReference: "op-1", Time: now.Add(-time.Hour), Resource: &ResourceDetail{Type: "store_renewal_period", Quantity: "8"}}}}}
	profile := &additionalHistoryStub{page: AdditionalAuditPage{Items: []AdditionalAuditEvent{{EventType: "account_business_profile.updated", ObjectType: "account_business_profile", ObjectReference: "member-1", Actor: "actor", Operation: "update", Version: 1, Key: "1", Time: now.Add(-8 * 24 * time.Hour)}}}}
	query, err := NewCurrentAuditSources(&historyStub{}, profile, &additionalHistoryStub{}, &usageHistoryStub{}, pointHistoryStub{}, resource)
	if err != nil {
		t.Fatal(err)
	}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "u1", TenantID: "B", EffectiveOrganizationID: "B", TokenExpiresAt: now.Add(time.Hour)})
	recent, err := query.ReadFiltered(ctx, 20, "", Filter{MemberID: "member-1", Content: "分配续费期数：8 期", Period: "7d"})
	if err != nil || len(recent.Items) != 1 || recent.Items[0].ObjectType != "member_resource" {
		t.Fatalf("recent=%+v err=%v", recent, err)
	}
	old, err := query.ReadFiltered(ctx, 20, "", Filter{MemberID: "member-1", Content: "更新账户资料", Period: "7d"})
	if err != nil || len(old.Items) != 0 {
		t.Fatalf("old=%+v err=%v", old, err)
	}
	all, err := query.ReadFiltered(ctx, 20, "", Filter{MemberID: "member-1", Content: "更新账户资料", Period: "all"})
	if err != nil || len(all.Items) != 1 || all.Items[0].ObjectType != "account_business_profile" {
		t.Fatalf("all=%+v err=%v", all, err)
	}
}

func TestFilteredProjectionCursorFitsLongestSupportedFilterAndSixSourcePositions(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	filter := Filter{ActorSubject: strings.Repeat("a", 128), Content: strings.Repeat("q", 80), MemberID: strings.Repeat("m", 128), Period: "all", AsOf: now.Format(time.RFC3339Nano)}
	state := cursorState{
		source:     &registry.HistoryPosition{OccurredAt: now, AccountID: "0198d4f0-0000-7000-8000-000000000004", Version: 9223372036854775807},
		profile:    &AuditPosition{CreatedAt: now, Key: "9223372036854775807"},
		membership: &AuditPosition{CreatedAt: now, Key: strings.Repeat("x", 550)},
		usage:      &AuditPosition{CreatedAt: now, Key: strings.Repeat("u", 160)},
		points:     &orgresource.ImagePointAuditPosition{CreatedAt: now, EventID: strings.Repeat("p", 128)},
		resources:  &AuditPosition{CreatedAt: now, Key: "9223372036854775807"},
	}
	cursor, err := encodeCursor(state, strings.Repeat("o", 128), filter)
	if err != nil || len(cursor) > 3072 {
		t.Fatalf("cursor len=%d err=%v", len(cursor), err)
	}
	if _, err := parseCursor(cursor, strings.Repeat("o", 128), filter); err != nil {
		t.Fatal(err)
	}
}

func TestFilteredProjectionDoesNotReturnPartialMatchesAfterLaterSourceFailure(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	const targetID = "0198d4f0-0000-7000-8000-000000000001"
	const otherID = "0198d4f0-0000-7000-8000-000000000002"
	items := make([]registry.CommittedOperation, 100)
	for index := range items {
		id := otherID
		if index == 0 {
			id = targetID
		}
		items[index] = registry.CommittedOperation{OrganizationID: "B", AccountID: id, ActorSubject: "actor", Kind: registry.OperationDisable, Version: 2, OccurredAt: now.Add(-time.Duration(index+1) * time.Second)}
	}
	position := items[99].Position()
	history := &failSecondHistory{first: registry.HistoryPage{Items: items, Next: &position}}
	query, err := NewCurrentAuditSources(history, &additionalHistoryStub{}, &additionalHistoryStub{}, &usageHistoryStub{}, pointHistoryStub{}, &additionalHistoryStub{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "u1", TenantID: "B", EffectiveOrganizationID: "B", TokenExpiresAt: now.Add(time.Hour)})
	page, err := query.ReadFiltered(ctx, 1, "", Filter{Content: targetID, Period: "all"})
	if !errors.Is(err, registry.ErrUnavailable) || page.SchemaVersion != "" || len(page.Items) != 0 || history.calls != 2 {
		t.Fatalf("partial result escaped: page=%+v err=%v calls=%d", page, err, history.calls)
	}
}

func TestFilteredProjectionRejectsIncompleteOwnerSetInsteadOfClaimingNoMatches(t *testing.T) {
	query, err := New(&historyStub{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "u1", TenantID: "B", EffectiveOrganizationID: "B", TokenExpiresAt: time.Now().Add(time.Hour)})
	page, err := query.ReadFiltered(ctx, 20, "", Filter{Content: "成员", Period: "all"})
	if !errors.Is(err, registry.ErrUnavailable) || page.SchemaVersion != "" {
		t.Fatalf("incomplete source set became success: %+v err=%v", page, err)
	}
}

func (s *usageHistoryStub) ListObservedAIUsageAudit(_ context.Context, org string, limit int, after *AuditPosition) (UsageAuditPage, error) {
	s.calls++
	page := UsageAuditPage{Items: []UsageAuditEvent{}}
	for _, item := range s.events {
		if item.OrganizationID != org {
			continue
		}
		if after != nil && (item.Time.After(after.CreatedAt) || item.Time.Equal(after.CreatedAt) && item.EventID >= after.Key) {
			continue
		}
		if len(page.Items) == limit {
			p := AuditPosition{CreatedAt: page.Items[len(page.Items)-1].Time, Key: page.Items[len(page.Items)-1].EventID}
			page.Next = &p
			break
		}
		page.Items = append(page.Items, item)
	}
	return page, nil
}

func TestProjectionPagesCommittedAIUsageWithoutActorImpersonation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	usage := &usageHistoryStub{events: []UsageAuditEvent{
		{OrganizationID: "B", EventID: "e-3", MemberID: "m-1", InvocationID: "inv-3", Quantity: 7, Time: now},
		{OrganizationID: "B", EventID: "e-2", MemberID: "m-1", InvocationID: "inv-2", Quantity: 8, Time: now},
		{OrganizationID: "B", EventID: "e-1", MemberID: "m-2", InvocationID: "inv-1", Quantity: 9, Time: now.Add(-time.Second)},
	}}
	query, _ := New(&historyStub{})
	query.usage = usage
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "u1", TenantID: "B", EffectiveOrganizationID: "B", TokenExpiresAt: now.Add(time.Hour)})
	seen := []string{}
	cursor := ""
	for i := 0; i < 3; i++ {
		page, err := query.Read(ctx, 1, cursor)
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("page %d: %+v %v", i, page, err)
		}
		item := page.Items[0]
		if item.Actor != "" || item.Usage == nil || item.Usage.Quantity != int64(7+i) || item.EventType != "ai_invocation.usage_observed" {
			t.Fatalf("item=%+v", item)
		}
		seen = append(seen, item.Relation.Reference)
		if i < 2 {
			if page.NextCursor == nil {
				t.Fatal("missing cursor")
			}
			cursor = *page.NextCursor
		} else if page.NextCursor != nil {
			t.Fatalf("unexpected cursor %s", *page.NextCursor)
		}
	}
	if strings.Join(seen, ",") != "e-3,e-2,e-1" {
		t.Fatal(seen)
	}
	filtered, err := query.ReadFiltered(ctx, 1, "", Filter{ActorSubject: "actor"})
	if err != nil || len(filtered.Items) != 0 || usage.calls != 3 {
		t.Fatalf("actor filter page=%+v err=%v calls=%d", filtered, err, usage.calls)
	}
	page, _ := query.Read(ctx, 1, "")
	other := authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{UserID: "u1", TenantID: "A", EffectiveOrganizationID: "A", TokenExpiresAt: now.Add(time.Hour)})
	if _, err := query.Read(other, 1, *page.NextCursor); !errors.Is(err, registry.ErrInvalid) {
		t.Fatalf("foreign cursor: %v", err)
	}
}

type pagingAdditionalHistoryStub struct {
	pages []AdditionalAuditPage
	calls int
}

func (s *additionalHistoryStub) ListRecentAudit(context.Context, string, int, string, string, *AuditPosition) (AdditionalAuditPage, error) {
	return s.page, nil
}

func (s *pagingAdditionalHistoryStub) ListRecentAudit(context.Context, string, int, string, string, *AuditPosition) (AdditionalAuditPage, error) {
	page := s.pages[s.calls]
	s.calls++
	return page, nil
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
	resources := &additionalHistoryStub{page: AdditionalAuditPage{Items: []AdditionalAuditEvent{
		{EventType: "account_member_ai_point_limit.changed", ObjectType: "member_ai_point_limit", ObjectReference: "m-1", Actor: "actor", Operation: "set_member_ai_point_limit", Version: 1, Key: "1", RelationReference: "op-1", Time: now.Add(-2 * time.Microsecond), Resource: &ResourceDetail{Type: "ai_point", Quantity: "1000"}},
		{EventType: "account_member_ai_point_limit.changed", ObjectType: "member_ai_point_limit", ObjectReference: "m-2", Actor: "actor", Operation: "set_member_ai_point_limit", Version: 2, Key: "2", RelationReference: "op-2", Time: now.Add(-3 * time.Microsecond), Resource: &ResourceDetail{Type: "ai_point", Quantity: "0"}},
	}}}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "u1", TenantID: "B", EffectiveOrganizationID: "B", TokenExpiresAt: now.Add(time.Hour)})
	query, err := NewCurrentAuditSources(history, nil, nil, nil, nil, resources)
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

func TestProjectionIncludesProfileAndMembershipAuditFacts(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	history := &historyStub{}
	profile := &additionalHistoryStub{page: AdditionalAuditPage{Items: []AdditionalAuditEvent{{EventType: "account_business_profile.updated", Actor: "actor", Time: now, ObjectType: "account_business_profile", ObjectReference: "u1", Operation: "update", Version: 1, Key: "1"}}}}
	membership := &additionalHistoryStub{page: AdditionalAuditPage{Items: []AdditionalAuditEvent{{EventType: "organization_membership.changed", Actor: "actor", Time: now.Add(-time.Microsecond), ObjectType: "organization_member", ObjectReference: "member-1", Operation: "role", Version: 2, Key: "00000000-0000-4000-8000-000000000001", RelationReference: "00000000-0000-4000-8000-000000000001"}}}}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "u1", TenantID: "B", EffectiveOrganizationID: "B", TokenExpiresAt: now.Add(time.Hour)})
	query, err := NewCurrentAuditSources(history, profile, membership, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	page, err := query.Read(ctx, 20, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.Items[0].ObjectType != "account_business_profile" || page.Items[1].ObjectType != "organization_member" {
		t.Fatalf("audit page = %#v", page.Items)
	}
	if page.Source != "source_account_committed_operations+account_business_profile_audit+organization_member_audit" {
		t.Fatalf("audit source = %q", page.Source)
	}
	if got := page.Items[1].Relation; got.Type != "organization_membership_operation" || got.Reference != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("membership relation = %#v", got)
	}
}

func TestProjectionOrdersFixedWidthProfileKeysNumerically(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	history := &historyStub{}
	profile := &pagingAdditionalHistoryStub{pages: []AdditionalAuditPage{
		{Items: []AdditionalAuditEvent{{EventType: "account_business_profile.updated", Actor: "actor", Time: now, ObjectType: "account_business_profile", ObjectReference: "u10", Operation: "update", Version: 10, Key: "00000000000000000010"}}, Next: &AuditPosition{CreatedAt: now, Key: "00000000000000000010"}},
		{Items: []AdditionalAuditEvent{{EventType: "account_business_profile.updated", Actor: "actor", Time: now, ObjectType: "account_business_profile", ObjectReference: "u9", Operation: "update", Version: 9, Key: "00000000000000000009"}}},
	}}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "u1", TenantID: "B", EffectiveOrganizationID: "B", TokenExpiresAt: now.Add(time.Hour)})
	query, err := NewCurrentAuditSources(history, profile, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	page, err := query.Read(ctx, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ObjectReference != "u10" || page.NextCursor == nil {
		t.Fatalf("first page = %#v", page)
	}
	page, err = query.Read(ctx, 1, *page.NextCursor)
	if err != nil || len(page.Items) != 1 || page.Items[0].ObjectReference != "u9" || page.NextCursor != nil {
		t.Fatalf("second page = %#v, err=%v", page, err)
	}
}
