package reportcenter

import (
	"context"
	"errors"
	"testing"
)

const sourceID = "f895350e-bd80-4b24-876e-30b8aa6f7505"
const commandID = "f895350e-bd80-4b24-876e-30b8aa6f7506"

var scope = Scope{"org-a", "actor-a"}

type sourceFunc func(context.Context, Scope, string, string) (Snapshot, error)

func (f sourceFunc) Read(c context.Context, s Scope, k, id string) (Snapshot, error) {
	return f(c, s, k, id)
}

type repoFake struct {
	Repository
	ref         SourceRef
	calls       int
	replay      bool
	fingerprint string
}

func (r *repoFake) Lookup(_ context.Context, _ Scope, _, fp string) (string, error) {
	if !r.replay {
		return "", nil
	}
	if r.fingerprint != fp {
		return "", ErrConflict
	}
	return sourceID, nil
}
func (r *repoFake) Read(context.Context, Scope, string) (Report, error) {
	return Report{ReportSummary: ReportSummary{ID: sourceID}}, nil
}
func (r *repoFake) Save(ctx context.Context, _ Scope, _, _ string, s Snapshot, check func(context.Context) error) (Result, error) {
	if e := check(ctx); e != nil {
		return Result{}, e
	}
	r.calls++
	r.ref = s.Ref
	return Result{Report: Report{ReportSummary: ReportSummary{ID: sourceID}}}, nil
}
func fixture() Snapshot {
	return Snapshot{SourceInfo: SourceInfo{Ref: SourceRef{"TITLE_REVIEW", sourceID, "1:accepted"}, Title: "标题审核结果", ProductKey: "product-a"}, Content: Document{SchemaVersion: 1, Sections: []Section{{Title: "历史标题", Fields: []Field{{"建议标题", "valid title"}}}}}}
}
func TestSaveExactVersionAndFreshAuthorization(t *testing.T) {
	r := &repoFake{}
	checks := 0
	snap := fixture()
	s := Service{Repository: r, Authorize: func(c context.Context, got Scope, manage bool) (context.Context, error) {
		checks++
		if got != scope || !manage {
			return nil, ErrForbidden
		}
		return c, nil
	}, Sources: sourceFunc(func(c context.Context, got Scope, k, id string) (Snapshot, error) { return snap, nil })}
	if _, e := s.Save(context.Background(), scope, commandID, SourceRef{"TITLE_REVIEW", sourceID, "1:pending"}); !errors.Is(e, ErrConflict) || r.calls != 0 {
		t.Fatalf("stale version saved: %v", e)
	}
	if _, e := s.Save(context.Background(), scope, commandID, snap.Ref); e != nil || r.calls != 1 || checks != 3 {
		t.Fatalf("save/commit authorization: %v calls=%d checks=%d", e, r.calls, checks)
	}
	snap.Ref.Version = "1:applied"
	if _, e := s.Save(context.Background(), scope, commandID, snap.Ref); e != nil || r.ref.Version != "1:applied" {
		t.Fatalf("Apply with unchanged revision: %v", e)
	}
}
func TestReplayDoesNotRecaptureButReauthorizes(t *testing.T) {
	ref := fixture().Ref
	r := &repoFake{replay: true, fingerprint: Fingerprint("save", ref)}
	checks := 0
	s := Service{Repository: r, Authorize: func(c context.Context, _ Scope, _ bool) (context.Context, error) { checks++; return c, nil }, Sources: sourceFunc(func(context.Context, Scope, string, string) (Snapshot, error) {
		t.Fatal("replay recaptured source")
		return Snapshot{}, nil
	})}
	got, e := s.Save(context.Background(), scope, commandID, ref)
	if e != nil || !got.Replayed || checks != 1 {
		t.Fatalf("replay: %+v %v checks=%d", got, e, checks)
	}
	ref.Version = "2:accepted"
	if _, e = s.Save(context.Background(), scope, commandID, ref); !errors.Is(e, ErrConflict) {
		t.Fatalf("different payload: %v", e)
	}
	s.Authorize = func(context.Context, Scope, bool) (context.Context, error) { return nil, ErrForbidden }
	if _, e = s.Save(context.Background(), scope, commandID, fixture().Ref); !errors.Is(e, ErrForbidden) {
		t.Fatalf("revoked replay: %v", e)
	}
}
func TestSnapshotRejectsInvalidOrOversizedContent(t *testing.T) {
	s := fixture()
	s.Content.Sections[0].Fields[0].Value = string(make([]byte, 17000))
	if _, _, e := s.Bytes(); e == nil {
		t.Fatal("oversized accepted")
	}
	s = fixture()
	s.Ref.Version = "01:accepted"
	if _, _, e := s.Bytes(); e == nil {
		t.Fatal("noncanonical accepted")
	}
}
