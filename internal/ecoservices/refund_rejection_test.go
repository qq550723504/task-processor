package ecoservices

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestPlatformRejectsUnagreedRefundWithoutApprovingMoney(t *testing.T) {
	for _, proposer := range []string{"buyer", "provider"} {
		t.Run(proposer, func(t *testing.T) {
			r := serviceRequest()
			r.State = "ACCEPTED"
			r.AcceptanceID = "original-acceptance"
			r.FinancialState = "SETTLEMENT_PENDING"
			remaining := int64(101)
			now := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
			if _, err := TransitionRequest(&r, Command{Scope: Scope{OrganizationID: proposer, ActorID: "proposer"}, Kind: "refund_propose", Version: r.Version, RefundAmountMinor: 40, RefundableAmount: &remaining, Reason: "proposal"}, now); err != nil {
				t.Fatal(err)
			}
			c := Command{Scope: Scope{Platform: true, ActorID: "platform-reviewer"}, Kind: "refund_review_reject", Version: r.Version, RefundVersion: r.Refund.Version, Reason: "No agreed refund; continue original acceptance"}
			approval := c
			approval.Kind = "refund_review"
			approval.RefundableAmount = &remaining
			if _, err := TransitionRequest(&r, approval, now); !errors.Is(err, ErrConflict) {
				t.Fatalf("unagreed refund approved: %v", err)
			}
			for _, invalid := range []Command{{Scope: Scope{OrganizationID: "buyer", ActorID: "buyer"}, Kind: c.Kind, Version: c.Version, RefundVersion: c.RefundVersion, Reason: c.Reason}, {Scope: c.Scope, Kind: c.Kind, Version: c.Version, RefundVersion: 0, Reason: c.Reason}, {Scope: c.Scope, Kind: c.Kind, Version: c.Version, RefundVersion: c.RefundVersion}} {
				if _, err := TransitionRequest(&r, invalid, now); err == nil {
					t.Fatal("unauthorized/stale/reasonless rejection allowed")
				}
			}
			fc, err := TransitionRequest(&r, c, now)
			if err != nil || fc != nil || r.Refund.State != "REJECTED" || r.FinancialFence || r.AcceptanceID != "original-acceptance" || r.FinancialState != "SETTLEMENT_PENDING" {
				t.Fatalf("original settlement hold not released: %+v %v", r, err)
			}
			encoded, _ := json.Marshal(r.Refund)
			var fact map[string]any
			_ = json.Unmarshal(encoded, &fact)
			review, ok := fact["review"].(map[string]any)
			if !ok || review["reason"] != c.Reason || review["actorId"] != c.Scope.ActorID || review["reviewedAt"] != now.Format(time.RFC3339) {
				t.Fatalf("review audit missing: %s", encoded)
			}
			// A second, unilateral proposal cannot hide an independent money hold.
			r.Refund = &RefundAgreement{Version: 2, AmountMinor: 40, Reason: "new proposal", BuyerConfirmed: true, State: "NEGOTIATING"}
			r.FinancialState = "RECONCILIATION_REQUIRED"
			r.FinancialFence = true
			c.Version = r.Version
			c.RefundVersion = 2
			if _, err := TransitionRequest(&r, c, now); err != nil || !r.FinancialFence {
				t.Fatalf("unagreed rejection cleared independent reconciliation: %v", err)
			}
		})
	}
}
