package ecoservices

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"task-processor/internal/commercial/billing"
	e "task-processor/internal/ecoservices"
	"task-processor/internal/integration/ecoservicesbilling"
	"task-processor/internal/ledger/money"
	"testing"
	"time"
)

type rejectionSettlementProvider struct {
	fulfillmentPaymentProvider
	dispatched []billing.ServiceFinancialOperation
}

func (p *rejectionSettlementProvider) DispatchServiceOperation(_ context.Context, o billing.ServicePurchaseOrder, op billing.ServiceFinancialOperation) (billing.ServiceOperationObservation, error) {
	p.dispatched = append(p.dispatched, op)
	return billing.ServiceOperationObservation{EventID: "effect:" + op.ProviderRequestID, ProfileVersion: o.Profile.Version, ProviderMerchantID: o.Source.ProviderMerchantID, TransactionID: o.Payment.TransactionID, ProviderRequestID: op.ProviderRequestID, Kind: op.Reservation.Kind, AmountMinor: op.Reservation.AmountMinor, State: "SUCCESS", ProviderReference: "original-channel:" + op.ProviderRequestID, VerificationVersion: "signed-fixture", OccurredAt: time.Now().UTC()}, nil
}

type rejectionReservationSource struct {
	billing.ServicePurchaseSource
	hold func()
	held bool
}

func (s *rejectionReservationSource) AdmitServiceCommand(ctx context.Context, c billing.ServicePurchaseCommand, id string) error {
	if c.Kind == "SETTLE" && !s.held {
		s.held = true
		s.hold()
	}
	return s.ServicePurchaseSource.AdmitServiceCommand(ctx, c, id)
}

func TestUnagreedRejectionRestoresOriginalSettlementAfterUndispatchedReservation(t *testing.T) {
	r, s, m, _, b, req, _ := fulfillmentFundsFixture(t)
	ctx := context.Background()
	// Generate the real original acceptance command through E's existing path.
	req.State = "AWAITING_ACCEPTANCE"
	req.Delivery = &e.Delivery{Version: 1, Content: "delivered", SubmittedAt: time.Now().UTC()}
	if err := r.db.Save(requestRecord(req)).Error; err != nil {
		t.Fatal(err)
	}
	accepted, err := s.Mutate(ctx, e.Command{Scope: e.Scope{OrganizationID: "buyer", ActorID: "buyer"}, Key: uuid.NewString(), Kind: "accept", ID: req.ID, Version: req.Version, DeliveryVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	req = *accepted.Request
	fc, err := r.FinancialCommand(ctx, req.AcceptanceID)
	if err != nil {
		t.Fatal(err)
	}
	original, err := b.ReadServicePurchase(ctx, req.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	provider := &rejectionSettlementProvider{}
	source := &rejectionReservationSource{ServicePurchaseSource: ecoservicesbilling.Source{Repository: r}}
	purchases, err := billing.NewServicePurchases(b, m, provider, source, fulfillmentProtection{})
	if err != nil {
		t.Fatal(err)
	}
	source.hold = func() {
		proposed, err := s.Mutate(ctx, e.Command{Scope: e.Scope{OrganizationID: "buyer", ActorID: "buyer"}, Key: uuid.NewString(), Kind: "refund_propose", ID: req.ID, Version: req.Version, RefundAmountMinor: 40, Reason: "unagreed proposal"})
		if err != nil {
			t.Fatal(err)
		}
		req = *proposed.Request
	}
	trading := ecoservicesbilling.Trading{Purchases: purchases}
	denied, err := trading.ExecuteServiceCommand(ctx, fc)
	if err == nil || denied.State != "SOURCE_DENIED" || len(provider.dispatched) != 0 {
		t.Fatalf("did not capture undispatched reservation: %+v %v", denied, err)
	}
	blocked, err := b.ReadServicePurchase(ctx, req.OrderID)
	if err != nil || len(blocked.DeniedOperations) != 1 || blocked.Operation != nil {
		t.Fatalf("denial proof not retained: %+v %v", blocked, err)
	}
	var abandoned money.ServiceOperation
	for id := range blocked.DeniedOperations {
		abandoned = blocked.Operations[id].Reservation
	}
	if _, err := m.PrepareServiceOperation(ctx, abandoned); err == nil {
		t.Fatal("abandoned reservation resurrected")
	}
	reject := e.Command{Scope: e.Scope{Platform: true, ActorID: "platform-reviewer"}, Key: uuid.NewString(), Kind: "refund_review_reject", ID: req.ID, Version: req.Version, RefundVersion: req.Refund.Version, Reason: "No agreed refund; resume accepted order"}
	rejected, err := s.Mutate(ctx, reject)
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Request.FinancialFence || rejected.Request.Refund.State != "REJECTED" || rejected.Request.AcceptanceID != fc.SourceProofID {
		t.Fatalf("original acceptance lost: %+v", rejected.Request)
	}
	// The original E recovery worker, not a new command or scheduler, resumes it.
	r.db.Model(&financialRow{}).Where("id=?", fc.ID).Update("next_attempt_at", time.Time{})
	service, err := e.NewService(r, trading, 180)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	settled, err := b.ReadServicePurchase(ctx, req.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if settled.State != "SETTLED" || settled.TradeNo != original.TradeNo || settled.MoneyInput().Payment.PaymentID != original.MoneyInput().Payment.PaymentID || len(provider.dispatched) != 2 || len(settled.DeniedOperations) != 1 {
		t.Fatalf("original settlement not recovered exactly once: %+v calls=%d", settled, len(provider.dispatched))
	}
	for _, op := range provider.dispatched {
		if op.Reservation.OperationID == abandoned.OperationID {
			t.Fatal("reused abandoned operation")
		}
	}
	funds, err := m.ReadServiceFunds(ctx, req.OrderID)
	if err != nil || funds.SharedMinor != 10 || funds.ReleasedMinor != 91 || funds.RefundedMinor != 0 || funds.PendingOperationID != "" {
		t.Fatalf("wrong funds after original settlement: %+v %v", funds, err)
	}
	if err := service.Recover(ctx); err != nil || len(provider.dispatched) != 2 {
		t.Fatalf("recovery duplicated effect: %v", err)
	}
	replay, err := s.Mutate(ctx, reject)
	if err != nil || replay.Request.Version != rejected.Request.Version {
		t.Fatal("rejection replay changed", err)
	}
	var version versionRow
	if err := r.db.Where("id=? AND kind='REQUEST' AND version=?", req.ID, rejected.Request.Version).Take(&version).Error; err != nil {
		t.Fatal(err)
	}
	var durable map[string]any
	_ = json.Unmarshal(version.Payload, &durable)
	refund := durable["refund"].(map[string]any)
	review, ok := refund["review"].(map[string]any)
	if !ok || review["reason"] != reject.Reason || review["actorId"] != reject.Scope.ActorID {
		t.Fatal("rejection audit not durable in original request history")
	}
}
