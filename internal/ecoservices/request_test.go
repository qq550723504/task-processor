package ecoservices

import (
	"errors"
	"testing"
	"time"
)

func serviceRequest() Request {
	return Request{ID: "request", BuyerOrganizationID: "buyer", ProviderOrganizationID: "provider", State: "PAID_READY", Version: 1, OrderID: "order", PaymentReceiptID: "verified-payment", Quote: &Quote{CommissionBPS: 1000, AllocationBasis: "CUMULATIVE_NET_FLOOR_V1", PolicyVersion: PolicyVersion, AmountMinor: 101, Version: 1, Scope: "service scope", AcceptanceCriteria: "deliver registered company", DeliveryDays: 3}}
}
func TestStartCancelAndExactCustomerAcceptance(t *testing.T) {
	r := serviceRequest()
	buyer := Scope{OrganizationID: "buyer", ActorID: "buyer-user"}
	provider := Scope{OrganizationID: "provider", ActorID: "provider-user"}
	cancel := Command{Scope: buyer, Kind: "cancel", Key: "cancel", Version: 1}
	if _, err := TransitionRequest(&r, cancel, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := TransitionRequest(&r, Command{Scope: provider, Kind: "start", Version: r.Version}, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("cancelled request started: %v", err)
	}
	r = serviceRequest()
	if _, err := TransitionRequest(&r, Command{Scope: provider, Kind: "start", Version: 1}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := TransitionRequest(&r, cancel, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale cancellation succeeded: %v", err)
	}
	if _, err := TransitionRequest(&r, Command{Scope: provider, Kind: "deliver", Version: r.Version, Delivery: &Delivery{Content: "registration complete"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := TransitionRequest(&r, Command{Scope: provider, Kind: "accept", Version: r.Version, DeliveryVersion: r.Delivery.Version}, time.Now()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("provider accepted own delivery: %v", err)
	}
	if _, err := TransitionRequest(&r, Command{Scope: buyer, Kind: "accept", Key: "accept", Version: r.Version, DeliveryVersion: r.Delivery.Version - 1}, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("wrong delivery accepted: %v", err)
	}
	fc, err := TransitionRequest(&r, Command{Scope: buyer, Kind: "accept", Key: "accept", Version: r.Version, DeliveryVersion: r.Delivery.Version}, time.Now())
	if err != nil || fc == nil || fc.Kind != "SETTLE" || r.AcceptanceID == "" {
		t.Fatalf("customer acceptance missing durable source command: %+v %v", fc, err)
	}
}
func TestRefundAgreementNeedsBothExactVersionsAndPlatformReview(t *testing.T) {
	remaining := int64(101)
	r := serviceRequest()
	r.State = "SERVICING"
	buyer := Scope{OrganizationID: "buyer", ActorID: "b"}
	provider := Scope{OrganizationID: "provider", ActorID: "p"}
	admin := Scope{ActorID: "a", Platform: true}
	if _, err := TransitionRequest(&r, Command{Scope: buyer, Kind: "refund_propose", Version: r.Version, RefundAmountMinor: 40, Reason: "partial delivery", RefundableAmount: &remaining}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := TransitionRequest(&r, Command{Scope: admin, Kind: "refund_review", Version: r.Version, RefundVersion: 1, Reason: "approved"}, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("single-side agreement reviewed: %v", err)
	}
	if _, err := TransitionRequest(&r, Command{Scope: provider, Kind: "refund_propose", Version: r.Version, RefundAmountMinor: 50, Reason: "counteroffer", RefundableAmount: &remaining}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if r.Refund.BuyerConfirmed {
		t.Fatal("changed amount retained old buyer consent")
	}
	if _, err := TransitionRequest(&r, Command{Scope: buyer, Kind: "refund_confirm", Version: r.Version, RefundVersion: 1}, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("old proposal consent accepted: %v", err)
	}
	if _, err := TransitionRequest(&r, Command{Scope: buyer, Kind: "refund_confirm", Version: r.Version, RefundVersion: r.Refund.Version}, time.Now()); err != nil {
		t.Fatal(err)
	}
	review := Command{Scope: admin, Kind: "refund_review", Key: "review", ID: r.ID, Version: r.Version, RefundVersion: r.Refund.Version, Reason: "approved"}
	review.Fingerprint = Fingerprint(review)
	in, err := BuildRefundReviewAdmission(review, r)
	if err != nil {
		t.Fatal(err)
	}
	proof := RefundReviewProof{ReceiptID: "original-review-proof", InputFingerprint: Fingerprint(in), PaymentReceiptID: r.PaymentReceiptID, RemainingMinor: remaining}
	proof.ResultFingerprint = proof.Fingerprint()
	review.RefundReviewProof = &proof
	fc, err := TransitionRequest(&r, review, time.Now())
	if err != nil || fc == nil || fc.Kind != "REFUND" || fc.AmountMinor != 50 {
		t.Fatalf("approved agreed refund missing: %+v %v", fc, err)
	}
}
func TestChannelReleaseNeverCreatesCustomerAcceptance(t *testing.T) {
	r := serviceRequest()
	r.State = "AWAITING_ACCEPTANCE"
	ApplyFinancialResult(&r, FinancialResult{OrderID: r.OrderID, State: "AUTOMATICALLY_RELEASED", Reason: "AR1: manual review required"}, time.Now())
	if r.AcceptanceID != "" || r.State != "AWAITING_ACCEPTANCE" {
		t.Fatal("channel expiry fabricated customer acceptance")
	}
}
func TestRefundUnknownPaymentProjectionCannotStartService(t *testing.T) {
	r := serviceRequest()
	r.State = "ORDER_PENDING"
	r.PaymentReceiptID = ""
	if err := ApplyFinancialResult(&r, FinancialResult{OrderID: r.OrderID, PaymentReceiptID: "original-paid-receipt", State: "RECONCILIATION_REQUIRED", Reason: "CHANNEL_REFUND_REQUIRES_RECONCILIATION", Revision: 2}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if r.State != "ORDER_PENDING" || r.PaymentReceiptID == "" || !r.FinancialFence {
		t.Fatalf("unknown refund became deliverable: %+v", r)
	}
	if _, err := TransitionRequest(&r, Command{Scope: Scope{OrganizationID: "provider", ActorID: "provider-user"}, Kind: "start", Version: r.Version}, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("unknown refund service started: %v", err)
	}
}
func TestRejectingRefundDoesNotClearIndependentChannelReconciliation(t *testing.T) {
	r := serviceRequest()
	r.State, r.FinancialState, r.FinancialFence = "SERVICING", "RECONCILIATION_REQUIRED", true
	r.Refund = &RefundAgreement{Version: 1, AmountMinor: 40, Reason: "partial delivery", BuyerConfirmed: true, ProviderConfirmed: true, State: "NEGOTIATING"}
	_, err := TransitionRequest(&r, Command{Scope: Scope{ActorID: "platform", Platform: true}, Kind: "refund_review_reject", Version: r.Version, RefundVersion: 1, Reason: "proposal rejected"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !r.FinancialFence {
		t.Fatal("dispute rejection cleared an independent channel money hold")
	}
	if _, err := TransitionRequest(&r, Command{Scope: Scope{ActorID: "provider", OrganizationID: "provider"}, Kind: "deliver", Version: r.Version, Delivery: &Delivery{Content: "delivery"}}, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("unverified money allowed delivery: %v", err)
	}
}
func TestLatePaymentProjectionCannotOverwriteCompletedRefund(t *testing.T) {
	r := serviceRequest()
	r.State = "CANCEL_REQUESTED"
	r.FinancialFence = true
	r.PaymentReceiptID = "original-money-receipt"
	if err := ApplyFinancialResult(&r, FinancialResult{OrderID: r.OrderID, PaymentReceiptID: "original-money-receipt", State: "REFUNDED", Revision: 10, FullRefund: true}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := ApplyFinancialResult(&r, FinancialResult{OrderID: r.OrderID, PaymentReceiptID: "original-money-receipt", State: "PAID", Revision: 4}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if r.FinancialState != "REFUNDED" || r.State != "CANCELLED" {
		t.Fatalf("late original payment erased refund: %+v", r)
	}
}
func TestFullApprovedRefundStopsFurtherServiceAndRetainsAcceptance(t *testing.T) {
	r := serviceRequest()
	r.State = "ACCEPTED"
	r.AcceptanceID = "original-acceptance"
	r.FinancialFence = true
	r.Refund = &RefundAgreement{State: "APPROVED", AmountMinor: 100}
	if err := ApplyFinancialResult(&r, FinancialResult{OrderID: r.OrderID, State: "REFUNDED", FullRefund: true, Revision: 12}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if r.State != "CANCELLED" || r.AcceptanceID != "original-acceptance" {
		t.Fatalf("fully refunded service remained operable or erased acceptance: %+v", r)
	}
}

func TestFinancialProjectionVersionOnlyChangesForBusinessFacts(t *testing.T) {
	r := serviceRequest()
	r.State = "PAID_READY"
	r.PaymentReceiptID = "original-payment"
	r.FinancialState = "PAID"
	r.FinancialRevision = 10
	r.UpdatedAt = time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	version, updated := r.Version, r.UpdatedAt
	if err := ApplyFinancialResult(&r, FinancialResult{OrderID: r.OrderID, PaymentReceiptID: r.PaymentReceiptID, State: "PAID", Revision: 20}, updated.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if r.Version != version || !r.UpdatedAt.Equal(updated) || r.FinancialRevision != 20 {
		t.Fatalf("revision-only result changed visible request or lost floor: %+v", r)
	}
}

func TestFinancialProjectionStillVersionsRealChanges(t *testing.T) {
	for _, change := range []string{"reconciliation", "reason", "expiry", "refund-state", "payment", "hidden-payment", "close"} {
		t.Run(change, func(t *testing.T) {
			r := serviceRequest()
			r.State, r.PaymentReceiptID, r.FinancialState = "PAID_READY", "original-payment", "PAID"
			r.FinancialRevision = 10
			result := FinancialResult{OrderID: r.OrderID, PaymentReceiptID: r.PaymentReceiptID, State: r.FinancialState, Revision: 20}
			switch change {
			case "reconciliation":
				result.State = "RECONCILIATION_REQUIRED"
			case "reason":
				result.Reason = "verified-original-channel-reason"
			case "expiry":
				expires := time.Now().UTC().Add(time.Hour)
				result.FundsExpireAt = &expires
			case "refund-state":
				r.FinancialState, result.State = "REFUNDED", "REFUNDED"
				r.Refund = &RefundAgreement{State: "APPROVED", AmountMinor: 1}
			case "payment":
				r.State, r.PaymentReceiptID = "ORDER_PENDING", ""
			case "hidden-payment":
				r.State, r.PaymentReceiptID = "CANCEL_REQUESTED", ""
				r.FinancialState, result.State = "CANCELLATION_PENDING", "CANCELLATION_PENDING"
			case "close":
				r.State, r.PaymentReceiptID = "CANCEL_REQUESTED", ""
				r.FinancialState, result.State, result.PaymentReceiptID = "CLOSED_UNPAID", "CLOSED_UNPAID", ""
				result.ReceiptID = "channel-closed:original"
			}
			version := r.Version
			now := time.Now().UTC()
			if err := ApplyFinancialResult(&r, result, now); err != nil {
				t.Fatal(err)
			}
			if r.Version != version+1 || !r.UpdatedAt.Equal(now) {
				t.Fatalf("real %s change did not advance user version: %+v", change, r)
			}
			if change == "refund-state" && r.Refund.State != "REFUNDED" {
				t.Fatal("refund completion not projected")
			}
		})
	}
}
