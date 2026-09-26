package accountallocation

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeQuotaReader struct {
	quota Quota
	err   error
}

func (f fakeQuotaReader) ReadTokenQuota(context.Context, string) (Quota, error) {
	return f.quota, f.err
}

type fakeRepo struct {
	set      Quota
	input    SetTargetInput
	snapshot Quota
	consume  Quota
}

func (f *fakeRepo) Snapshot(_ context.Context, q Quota) (Snapshot, error) {
	f.snapshot = q
	return Snapshot{OrganizationID: q.OrganizationID, Metric: q.Metric}, nil
}
func (f *fakeRepo) SetTarget(_ context.Context, q Quota, input SetTargetInput) (Allocation, error) {
	f.set, f.input = q, input
	return Allocation{Allocated: input.Target}, nil
}
func (f *fakeRepo) Consume(_ context.Context, q Quota, _ ConsumeInput) error {
	f.consume = q
	return nil
}

func quota() Quota {
	return Quota{OrganizationID: "org-1", Metric: MetricToken, Total: 100, WindowStart: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), WindowEnd: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
}

func TestServicePassesCommercialQuotaToAllocationOwner(t *testing.T) {
	repo := &fakeRepo{}
	service, err := NewService(fakeQuotaReader{quota: quota()}, repo)
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.SetTarget(context.Background(), SetTargetInput{OrganizationID: "org-1", MemberID: "member-1", Target: 20, ExpectedVersion: 0, IdempotencyKey: "op-1", ActorID: "actor-1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Allocated != 20 || repo.set.Total != 100 || repo.set.WindowEnd.IsZero() {
		t.Fatalf("got allocation=%+v quota=%+v", got, repo.set)
	}
}

func TestServiceRejectsInvalidMutationBeforeOwnerCall(t *testing.T) {
	repo := &fakeRepo{}
	service, _ := NewService(fakeQuotaReader{quota: quota()}, repo)
	_, err := service.SetTarget(context.Background(), SetTargetInput{OrganizationID: "org-1", MemberID: "member-1", Target: -1, IdempotencyKey: "op-1", ActorID: "actor-1"})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("err=%v", err)
	}
	if !repo.set.WindowStart.IsZero() {
		t.Fatal("repository called for invalid target")
	}
}
