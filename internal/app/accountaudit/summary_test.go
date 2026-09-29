package accountaudit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/ledger/orgresource"
	registry "task-processor/internal/sourceaccountregistry"
)

type summaryAdditional struct {
	events     []AdditionalAuditEvent
	err        error
	calls      int
	failOnCall int
}

func (s *summaryAdditional) ListRecentAudit(ctx context.Context, org string, limit int, actor, operation string, after *AuditPosition) (AdditionalAuditPage, error) {
	s.calls++
	if s.err != nil && (s.failOnCall == 0 || s.calls >= s.failOnCall) {
		return AdditionalAuditPage{}, s.err
	}
	if err := ctx.Err(); err != nil {
		return AdditionalAuditPage{}, err
	}
	page := AdditionalAuditPage{Items: []AdditionalAuditEvent{}}
	for _, e := range s.events {
		if after != nil && (e.Time.After(after.CreatedAt) || e.Time.Equal(after.CreatedAt) && e.Key >= after.Key) {
			continue
		}
		if len(page.Items) == limit {
			last := page.Items[len(page.Items)-1]
			page.Next = &AuditPosition{CreatedAt: last.Time, Key: last.Key}
			break
		}
		page.Items = append(page.Items, e)
	}
	return page, nil
}

type summaryPoints struct{ events []orgresource.ImagePointDebit }

func (s summaryPoints) ListImagePointDebits(context.Context, string, string, int, *orgresource.ImagePointAuditPosition) (orgresource.ImagePointAuditPage, error) {
	return orgresource.ImagePointAuditPage{Items: s.events}, nil
}
func TestSummaryDeduplicatesAcrossPagesAndRejectsPartialRead(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Microsecond)
	profile := &summaryAdditional{}
	for i := 105; i > 0; i-- {
		e := profileSummaryEvent(at.Add(-time.Second), i)
		e.Version = 1
		profile.events = append(profile.events, e)
	}
	q := summaryQuery(t, profile, &summaryAdditional{}, &summaryAdditional{})
	got, err := q.readSummary(summaryContext(), at)
	if err != nil || got.Counts.Operations != "1" || profile.calls < 2 {
		t.Fatalf("dedup across pages %+v %v calls=%d", got, err, profile.calls)
	}
	profile.calls = 0
	profile.failOnCall = 2
	profile.err = context.DeadlineExceeded
	got, err = q.readSummary(summaryContext(), at)
	if !errors.Is(err, context.DeadlineExceeded) || got.SchemaVersion != "" {
		t.Fatalf("partial published %+v %v", got, err)
	}
}
func TestSummaryCountsCommittedImageDebitWithoutSummingPointQuantity(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Microsecond)
	q := summaryQuery(t, &summaryAdditional{}, &summaryAdditional{}, &summaryAdditional{})
	q.points = summaryPoints{events: []orgresource.ImagePointDebit{{OrganizationID: "B", EventID: "event-1", ActorID: "actor", MemberID: "member", RunID: "run", IntentID: "intent", PriceVersion: "price", Points: 1200, CreatedAt: at.Add(-time.Second)}}}
	got, err := q.readSummary(summaryContext(), at)
	if err != nil || got.Counts.Operations != "1" || got.Counts.Resources != "1" {
		t.Fatalf("image debit %+v %v", got, err)
	}
}

func summaryContext() context.Context {
	return authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "u", TenantID: "B", EffectiveOrganizationID: "B", TokenExpiresAt: time.Now().Add(time.Hour)})
}
func summaryQuery(t *testing.T, profile, members, resources *summaryAdditional) *Query {
	t.Helper()
	q, err := NewCurrentAuditSources(&historyStub{}, profile, members, &usageHistoryStub{}, summaryPoints{}, resources)
	if err != nil {
		t.Fatal(err)
	}
	return q
}
func profileSummaryEvent(at time.Time, id int) AdditionalAuditEvent {
	return AdditionalAuditEvent{EventType: "account_business_profile.updated", Actor: "actor", Time: at, ObjectType: "account_business_profile", ObjectReference: "u", Operation: "update", Version: int64(id), Key: fmt.Sprintf("%020d", id)}
}
func TestSummaryCountsEntireWindowAcrossPagesAndClassifiesOverlaps(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Microsecond)
	profile := &summaryAdditional{}
	for i := 105; i > 0; i-- {
		profile.events = append(profile.events, profileSummaryEvent(at.Add(-time.Minute), i))
	}
	profile.events = append(profile.events, profileSummaryEvent(at.Add(-30*24*time.Hour), 106), profileSummaryEvent(at.Add(-30*24*time.Hour-time.Microsecond), 107))
	members := &summaryAdditional{}
	for i, op := range []string{"invite", "role", "remove"} {
		members.events = append(members.events, AdditionalAuditEvent{EventType: "organization_membership.changed", Actor: "actor", Time: at.Add(-time.Second), ObjectType: "organization_member", ObjectReference: "member", Operation: op, Version: 1, Key: fmt.Sprintf("member-%d", 3-i), RelationReference: fmt.Sprintf("op-%d", i)})
	}
	resources := &summaryAdditional{events: []AdditionalAuditEvent{
		{EventType: "account_member_resource.changed", Actor: "actor", Time: at.Add(-time.Second), ObjectType: "member_resource", ObjectReference: "member", Operation: "allocate_member_resource", Version: 1, Key: "00000000000000000002", RelationReference: "allocation", Resource: &ResourceDetail{Type: orgresource.ResourceDataRow, Quantity: "100"}},
		{EventType: "account_member_ai_point_limit.changed", Actor: "actor", Time: at.Add(-time.Second), ObjectType: "member_ai_point_limit", ObjectReference: "member", Operation: "set_member_ai_point_limit", Version: 2, Key: "00000000000000000001", RelationReference: "limit", Resource: &ResourceDetail{Type: orgresource.ResourceAIPoint, Quantity: "0"}},
	}}
	q := summaryQuery(t, profile, members, resources)
	usage := &usageHistoryStub{events: []UsageAuditEvent{{OrganizationID: "B", EventID: "observed", MemberID: "member", InvocationID: "inv", Quantity: 10, Time: at.Add(-time.Second)}}}
	q.usage = usage
	got, err := q.readSummary(summaryContext(), at)
	if err != nil {
		t.Fatal(err)
	}
	if got.Counts.Operations != "111" || got.Counts.Members != "3" || got.Counts.Permissions != "1" || got.Counts.Resources != "2" {
		t.Fatalf("counts %+v", got)
	}
	if profile.calls < 2 || usage.calls != 0 {
		t.Fatalf("incomplete scan or observed counted: %d/%d", profile.calls, usage.calls)
	}
	if !got.Window.From.Equal(at.Add(-30*24*time.Hour)) || !got.Window.AsOf.Equal(at) {
		t.Fatalf("window %+v", got.Window)
	}
}
func TestSummaryBoundaryZeroAndOwnerFailures(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Microsecond)
	p := &summaryAdditional{events: []AdditionalAuditEvent{profileSummaryEvent(at.Add(time.Microsecond), 3), profileSummaryEvent(at, 2)}}
	q := summaryQuery(t, p, &summaryAdditional{}, &summaryAdditional{})
	got, err := q.readSummary(summaryContext(), at)
	if err != nil || got.Counts.Operations != "0" {
		t.Fatalf("zero %+v %v", got, err)
	}
	p.err = errors.New("private owner failure")
	if got, err = q.readSummary(summaryContext(), at); err == nil || got.SchemaVersion != "" {
		t.Fatalf("failure published counts %+v %v", got, err)
	}
	q.profile = nil
	if _, err = q.ReadSummary(summaryContext()); !errors.Is(err, ErrSummaryNotConfigured) {
		t.Fatalf("missing owner %v", err)
	}
	q.profile = (*summaryAdditional)(nil)
	if _, err = q.ReadSummary(summaryContext()); !errors.Is(err, ErrSummaryNotConfigured) {
		t.Fatalf("typed nil %v", err)
	}
	q.profile = &summaryAdditional{}
	ctx, cancel := context.WithCancel(summaryContext())
	cancel()
	if _, err = q.ReadSummary(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation %v", err)
	}
	if _, err = q.ReadSummary(context.Background()); !errors.Is(err, registry.ErrAuthenticationRequired) {
		t.Fatalf("identity %v", err)
	}
}
func TestSummaryOriginalIdentityDedupAndConflictingPayload(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Microsecond)
	a := profileSummaryEvent(at.Add(-time.Second), 1)
	b := a
	b.Key = "00000000000000000002"
	q := summaryQuery(t, &summaryAdditional{events: []AdditionalAuditEvent{b, a}}, &summaryAdditional{}, &summaryAdditional{})
	got, err := q.readSummary(summaryContext(), at)
	if err != nil || got.Counts.Operations != "1" {
		t.Fatalf("dedup %+v %v", got, err)
	}
	b.Actor = "different"
	q.profile = &summaryAdditional{events: []AdditionalAuditEvent{b, a}}
	if _, err = q.readSummary(summaryContext(), at); !errors.Is(err, registry.ErrUnavailable) {
		t.Fatalf("conflicting operation %v", err)
	}
}

type numericSourceHistory struct{ events []registry.CommittedOperation }

func (s numericSourceHistory) List(_ context.Context, r registry.HistoryRequest) (registry.HistoryPage, error) {
	page := registry.HistoryPage{}
	for _, e := range s.events {
		if r.After != nil && !e.Position().Before(*r.After) {
			continue
		}
		if len(page.Items) == r.Limit {
			p := page.Items[len(page.Items)-1].Position()
			page.Next = &p
			break
		}
		page.Items = append(page.Items, e)
	}
	return page, nil
}
func TestProjectionKeepsNumericSourceVersionOrderAcrossMergeBoundary(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Microsecond)
	id := "0198d4f0-0000-7000-8000-000000000001"
	h := numericSourceHistory{events: []registry.CommittedOperation{
		{OrganizationID: "B", AccountID: id, ActorSubject: "actor", Kind: registry.OperationDisable, Version: 10, OccurredAt: at},
		{OrganizationID: "B", AccountID: id, ActorSubject: "actor", Kind: registry.OperationDisable, Version: 2, OccurredAt: at},
	}}
	q, _ := NewCurrentAuditSources(h, &summaryAdditional{events: []AdditionalAuditEvent{profileSummaryEvent(at, 1)}}, nil, nil, nil, nil)
	versions := []string{}
	cursor := ""
	for i := 0; i < 3; i++ {
		page, err := q.Read(summaryContext(), 2, cursor)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range page.Items {
			if e.ObjectType == "source_account" {
				versions = append(versions, e.Relation.Version)
			}
		}
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	if strings.Join(versions, ",") != "10,2" {
		t.Fatalf("lost numeric owner order: %v", versions)
	}
}
