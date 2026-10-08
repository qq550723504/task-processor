package notificationcenter

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

type testSource struct {
	items   []Item
	err     error
	readErr error
	calls   int
}

func (s *testSource) Name() string   { return "workbench-task" }
func (*testSource) Category() string { return Business }
func (*testSource) Personal() bool   { return false }
func (s *testSource) ListVisible(_ context.Context, scope Scope, after string, limit int) (SourcePage, error) {
	s.calls++
	if s.err != nil {
		return SourcePage{}, s.err
	}
	start := 0
	if after != "" {
		for i, v := range s.items {
			if v.Ref.EntityID == after {
				start = i + 1
			}
		}
	}
	end := start + limit
	if end > len(s.items) {
		end = len(s.items)
	}
	page := SourcePage{Items: s.items[start:end]}
	if end < len(s.items) {
		page.Next = s.items[end-1].Ref.EntityID
	}
	return page, nil
}
func (s *testSource) ReadVisible(_ context.Context, _ Scope, ref Ref) (Item, error) {
	if s.readErr != nil {
		return Item{}, s.readErr
	}
	if s.err != nil {
		return Item{}, s.err
	}
	for _, item := range s.items {
		if item.Ref.EntityID == ref.EntityID {
			return item, nil
		}
	}
	return Item{}, ErrNotFound
}

func TestMissingCurrentReaderIsIncomplete(t *testing.T) {
	s, _, _, scope := setup()
	s.Sources = []Source{ReaderSource{SourceName: "workbench-task"}}
	l, err := s.List(context.Background(), scope, ListRequest{Limit: 20})
	if err != nil || l.Exact || len(l.Coverage) != 1 || l.Coverage[0].Complete {
		t.Fatalf("missing current reader presented as complete: %+v %v", l, err)
	}
	if _, err = s.PrepareReadAll(context.Background(), scope, "89e482d7-28c6-4e38-9dfd-7a6e81b8ac7c"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
func TestListReturnsSummaryWhileDetailKeepsFullContent(t *testing.T) {
	s, src, _, scope := setup()
	src.items[0].Paragraphs = []string{"full content reserved for detail"}
	l, err := s.List(context.Background(), scope, ListRequest{Limit: 20})
	if err != nil || len(l.Items[0].Paragraphs) != 0 {
		t.Fatalf("list includes full body: %+v %v", l, err)
	}
	d, err := s.Detail(context.Background(), scope, l.Items[0].ID)
	if err != nil || len(d.Paragraphs) != 1 {
		t.Fatalf("detail lost content: %+v %v", d, err)
	}
}

func TestReadAllRechecksOriginalResourceGate(t *testing.T) {
	s, src, repo, scope := setup()
	snap, err := s.PrepareReadAll(context.Background(), scope, "dbd21057-4634-4d55-a092-60bc5107ddf0")
	if err != nil {
		t.Fatal(err)
	}
	src.readErr = ErrForbidden
	_, err = s.ReadAll(context.Background(), scope, "3968daba-600c-4523-bbee-a320f2a66790", snap.ID, snap.Fingerprint)
	if !errors.Is(err, ErrForbidden) || len(repo.reads) != 0 || len(repo.commands) != 0 {
		t.Fatalf("revoked original resource committed: %v %+v", err, repo)
	}
}
func TestListPagesStayWithinHTTPResponseBound(t *testing.T) {
	s, src, _, scope := setup()
	for i := 1; i < 100; i++ {
		item := src.items[0]
		item.Ref.EntityID = "task-" + strconv.Itoa(i)
		src.items = append(src.items, item)
	}
	for i := range src.items {
		src.items[i].Summary = strings.Repeat("a", 1024)
	}
	l, err := s.List(context.Background(), scope, ListRequest{Limit: 100})
	raw, _ := json.Marshal(l)
	if err != nil || len(raw) > 128<<10 || l.Next == "" || l.Count != 100 {
		t.Fatalf("oversized page: %d bytes, next=%q count=%d err=%v", len(raw), l.Next, l.Count, err)
	}
}
func TestTargetsUseCurrentReviewPageAndBoundedNativeIdentifiers(t *testing.T) {
	if href, err := Href(Target{Kind: "review", ID: "proposal-a"}); err != nil || href != "/workbench/ai/tasks/pending/other?proposal_id=proposal-a" {
		t.Fatalf("invalid original review entry %s %v", href, err)
	}
	for _, id := range []string{".", "..", "a/b", "a?subject=other"} {
		if _, err := Href(Target{Kind: "task", ID: id}); err == nil {
			t.Fatalf("unbounded native target accepted: %s", id)
		}
	}
}

type testRepo struct {
	reads     map[string]bool
	snapshots map[string]Snapshot
	commands  map[string]Command
	fail      bool
}

func (r *testRepo) ReadStates(_ context.Context, scope Scope, _ []Ref) (map[string]bool, error) {
	out := map[string]bool{}
	for k, v := range r.reads {
		out[k] = v
	}
	return out, nil
}
func (r *testRepo) Replay(_ context.Context, scope Scope, key, op, digest string) (Command, bool, error) {
	c, ok := r.commands[ScopeKey(scope)+key]
	if ok && (c.Operation != op || c.Fingerprint != digest) {
		return Command{}, false, ErrConflict
	}
	return c, ok, nil
}
func (r *testRepo) Command(_ context.Context, scope Scope, key string) (Command, error) {
	c, ok := r.commands[ScopeKey(scope)+key]
	if !ok {
		return c, ErrNotFound
	}
	return c, nil
}
func (r *testRepo) CommitRead(_ context.Context, scope Scope, c Command, refs []Ref, snapshot string) (Command, error) {
	if r.fail {
		return Command{}, ErrUnavailable
	}
	for _, ref := range refs {
		r.reads[RefKey(ref)] = true
	}
	r.commands[ScopeKey(scope)+c.Key] = c
	return c, nil
}
func (r *testRepo) CreateSnapshot(_ context.Context, _ Scope, c Command, s Snapshot) (Snapshot, error) {
	r.snapshots[s.ID] = s
	return s, nil
}
func (r *testRepo) Snapshot(_ context.Context, _ Scope, id string) (Snapshot, error) {
	s, ok := r.snapshots[id]
	if !ok {
		return s, ErrNotFound
	}
	return s, nil
}
func (*testRepo) Publish(context.Context, Scope, Command, AnnouncementInput) (Command, error) {
	return Command{}, ErrUnavailable
}
func (*testRepo) Withdraw(context.Context, Scope, Command, string, int64) (Command, error) {
	return Command{}, ErrUnavailable
}
func setup() (*Service, *testSource, *testRepo, Scope) {
	src := &testSource{items: []Item{{Ref: Ref{Source: "workbench-task", EntityID: "task-a", Type: "task.pending", Revision: "1", OrganizationID: "org-a"}, Category: Business, Title: "待确认", Summary: "请查看任务", Attention: true, Target: Target{Kind: "task", ID: "task-a"}}}}
	repo := &testRepo{reads: map[string]bool{}, snapshots: map[string]Snapshot{}, commands: map[string]Command{}}
	return &Service{Repository: repo, Sources: []Source{src}, Now: func() time.Time { return time.Unix(1800000000, 0) }}, src, repo, Scope{Realm: "https://identity.example", Subject: "user-a", OrganizationID: "org-a", Category: Business}
}
func TestReadIsDurableAndDoesNotResolveAttention(t *testing.T) {
	s, src, repo, scope := setup()
	l, err := s.List(context.Background(), scope, ListRequest{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Read(context.Background(), scope, "aef9db82-e326-4ee7-b134-3f50c3a93c57", l.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	l, err = s.List(context.Background(), scope, ListRequest{Limit: 20})
	if err != nil || !l.Items[0].Read || l.Pending != 1 || l.Unread != 0 {
		t.Fatalf("%+v %v", l, err)
	}
	src.items[0].Ref.Revision = "2"
	l, _ = s.List(context.Background(), scope, ListRequest{Limit: 20})
	if l.Unread != 1 {
		t.Fatal("new revision inherited read")
	}
	repo.fail = true
	_, err = s.Read(context.Background(), scope, "d662ab8f-c08b-482b-868f-5079e6087eae", l.Items[0].ID)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("commit failure reported success: %v", err)
	}
}
func TestSnapshotIncludesAllPagesAndLeavesConcurrentNewRevisionUnread(t *testing.T) {
	s, src, _, scope := setup()
	for i := 1; i < 121; i++ {
		item := src.items[0]
		item.Ref.EntityID = "task-" + strconv.Itoa(i)
		src.items = append(src.items, item)
	}
	snap, err := s.PrepareReadAll(context.Background(), scope, "fc91a60b-5267-4827-aad2-16bcaec2b074")
	if err != nil || snap.Count != 121 || src.calls < 2 {
		t.Fatalf("%+v %v", snap, err)
	}
	fresh := src.items[0]
	fresh.Ref.EntityID = "new-task"
	src.items = append(src.items, fresh)
	key := "7d231749-ad85-4749-b339-7183e9d62c29"
	if _, err = s.ReadAll(context.Background(), scope, key, snap.ID, snap.Fingerprint); err != nil {
		t.Fatal(err)
	}
	l, _ := s.List(context.Background(), scope, ListRequest{Limit: 20})
	if l.Unread != 1 || !l.Exact {
		t.Fatalf("%+v", l)
	}
	s.Now = func() time.Time { return snap.ExpiresAt.Add(time.Hour) }
	if _, err = s.ReadAll(context.Background(), scope, key, snap.ID, snap.Fingerprint); err != nil {
		t.Fatalf("committed replay must precede expiry: %v", err)
	}
}
func TestDeniedAndUnavailableSourcesCannotLeakOrReportCompleteReadAll(t *testing.T) {
	for _, e := range []error{ErrForbidden, ErrUnavailable} {
		s, src, _, scope := setup()
		src.err = e
		l, err := s.List(context.Background(), scope, ListRequest{Limit: 20})
		if err != nil || len(l.Items) != 0 {
			t.Fatalf("%+v %v", l, err)
		}
		if e == ErrUnavailable {
			if l.Exact {
				t.Fatal("unavailable source became exact zero")
			}
			if _, err = s.PrepareReadAll(context.Background(), scope, "89e482d7-28c6-4e38-9dfd-7a6e81b8ac7c"); err == nil {
				t.Fatal("incomplete snapshot accepted")
			}
		}
	}
}
func TestForeignScopeAndChangedRevisionFailBeforeReadCommit(t *testing.T) {
	s, src, repo, scope := setup()
	l, _ := s.List(context.Background(), scope, ListRequest{Limit: 20})
	scope.OrganizationID = "org-b"
	if _, err := s.Read(context.Background(), scope, "831ffded-4e21-4cbb-9417-38653eec45b8", l.Items[0].ID); err == nil {
		t.Fatal("foreign organization accepted")
	}
	scope.OrganizationID = "org-a"
	snap, _ := s.PrepareReadAll(context.Background(), scope, "dbd21057-4634-4d55-a092-60bc5107ddf0")
	src.items[0].Ref.Revision = "2"
	if _, err := s.ReadAll(context.Background(), scope, "3968daba-600c-4523-bbee-a320f2a66790", snap.ID, snap.Fingerprint); !errors.Is(err, ErrStale) || len(repo.reads) != 0 {
		t.Fatalf("stale source marked read: %v", err)
	}
}
