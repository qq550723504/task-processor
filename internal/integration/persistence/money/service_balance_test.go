package money

import (
	"context"
	"errors"
	m "task-processor/internal/ledger/money"
	"testing"
	"time"
)

func TestServiceUnsplitObservationDoesNotInventReleaseOrClearFence(t *testing.T) {
	r := newMoneyRepository(t)
	ctx := context.Background()
	in := servicePayment()
	if _, err := r.AcceptServicePayment(ctx, in); err != nil {
		t.Fatal(err)
	}
	before, _ := r.ReadServiceFunds(ctx, in.OrderID)
	observation := m.ServiceUnsplitObservation{Payment: in, ProfileVersion: "original-profile", ExpectedFundsFingerprint: m.ServiceFingerprint(before), UnsplitMinor: 0, ProofID: "signed-original-response", OccurredAt: time.Now().UTC()}
	got, err := r.ObserveServiceUnsplit(ctx, observation)
	if err != nil || got.ReconciliationReason != "CHANNEL_FUNDS_CHANGED_REQUIRES_RECONCILIATION" || got.AutomaticReleasedMinor != 0 || got.ReleasedMinor != 0 || got.RefundedMinor != 0 {
		t.Fatalf("invented release or missing fence: %+v %v", got, err)
	}
	operation := m.ServiceOperation{OrderID: in.OrderID, OperationID: "share-after-observation", Kind: m.ServiceShare, AmountMinor: 10, SourceProofID: "accepted"}
	if _, err := r.PrepareServiceOperation(ctx, operation); err == nil {
		t.Fatal("changed channel funds admitted a new share")
	}
	observation.ExpectedFundsFingerprint = m.ServiceFingerprint(got)
	observation.UnsplitMinor = 101
	observation.ProofID = "matching-later-signed-response"
	got, err = r.ObserveServiceUnsplit(ctx, observation)
	if err != nil || got.ReconciliationReason == "" {
		t.Fatal("matching observation cleared independent fence")
	}
}

func TestServiceUnsplitStaleSnapshotIsEvidenceNotAdmission(t *testing.T) {
	r := newMoneyRepository(t)
	ctx := context.Background()
	in := servicePayment()
	if _, err := r.AcceptServicePayment(ctx, in); err != nil {
		t.Fatal(err)
	}
	before, _ := r.ReadServiceFunds(ctx, in.OrderID)
	applyServiceEffect(t, r, m.ServiceRefund, "refund-before-observation", 1)
	observation := m.ServiceUnsplitObservation{Payment: in, ProfileVersion: "profile", ExpectedFundsFingerprint: m.ServiceFingerprint(before), UnsplitMinor: 0, ProofID: "signed-stale-snapshot", OccurredAt: time.Now().UTC()}
	if _, err := r.ObserveServiceUnsplit(ctx, observation); !errors.Is(err, m.ErrConflict) {
		t.Fatalf("stale snapshot admitted: %v", err)
	}
	got, _ := r.ReadServiceFunds(ctx, in.OrderID)
	if got.ReconciliationReason != "" || got.RefundedMinor != 1 || got.AutomaticReleasedMinor != 0 {
		t.Fatalf("stale query changed current facts: %+v", got)
	}
	var count int64
	r.db.Model(&serviceEffectRow{}).Where("kind=?", "UNSPLIT_OBSERVATION").Count(&count)
	if count != 1 {
		t.Fatal("stale signed original evidence not retained")
	}
}

func TestServiceUnsplitPendingOriginalEffectDoesNotInventAnomaly(t *testing.T) {
	r := newMoneyRepository(t)
	ctx := context.Background()
	in := servicePayment()
	if _, err := r.AcceptServicePayment(ctx, in); err != nil {
		t.Fatal(err)
	}
	op := m.ServiceOperation{OrderID: in.OrderID, OperationID: "original-unknown-share", Kind: m.ServiceShare, AmountMinor: 10, SourceProofID: "accepted"}
	if _, err := r.PrepareServiceOperation(ctx, op); err != nil {
		t.Fatal(err)
	}
	if err := r.AdmitServiceOperation(ctx, op); err != nil {
		t.Fatal(err)
	}
	f, _ := r.ReadServiceFunds(ctx, in.OrderID)
	q := m.ServiceUnsplitObservation{Payment: in, ProfileVersion: "profile", ExpectedFundsFingerprint: m.ServiceFingerprint(f), UnsplitMinor: 91, ProofID: "pending-signed-query", OccurredAt: time.Now().UTC()}
	if _, err := r.ObserveServiceUnsplit(ctx, q); !errors.Is(err, m.ErrConflict) {
		t.Fatal("unobserved in-flight effect used as admission")
	}
	f, _ = r.ReadServiceFunds(ctx, in.OrderID)
	if f.ReconciliationReason != "" || f.PendingOperationID != op.OperationID {
		t.Fatal("in-flight effect became a permanent anomaly or lost its reservation")
	}
	q.UnsplitMinor = 101
	q.ProofID = "matching-signed-query"
	if _, err := r.ObserveServiceUnsplit(ctx, q); err != nil {
		t.Fatal("matching funds prevented verified original replay", err)
	}
}
