package ecoservices

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	e "task-processor/internal/ecoservices"
)

func TestInitialApplicationRequiresReviewedChannelCompatibleEvidence(t *testing.T) {
	for _, f := range []struct {
		mime  string
		size  int64
		valid bool
	}{
		{"application/pdf", 100, false}, {"image/png", 2<<20 + 1, false},
		{"image/png", 2 << 20, true}, {"image/jpeg", 100, true},
	} {
		t.Run(fmt.Sprintf("%s-%d", f.mime, f.size), func(t *testing.T) {
			r, s := fixture(t)
			id := uuid.NewString()
			if err := r.db.Create(&fileRow{ID: id, OrganizationID: "provider", ParentKind: "APPLICATION", State: "CONFIRMED", ContentType: f.mime, SizeBytes: f.size}).Error; err != nil {
				t.Fatal(err)
			}
			_, err := s.Mutate(context.Background(), e.Command{Scope: e.Scope{OrganizationID: "provider", ActorID: "p"}, Key: uuid.NewString(), Kind: "application_submit", Application: &e.Application{CompanyName: "Provider", RegistrationNumber: "registration", Categories: []e.Category{e.StoreOpening}, Regions: []string{"Shanghai"}, FileIDs: []string{id}}})
			if f.valid && err != nil {
				t.Fatal(err)
			}
			if !f.valid && !errors.Is(err, e.ErrInvalid) {
				t.Fatalf("unusable original evidence accepted: %v", err)
			}
			if !f.valid {
				var count int64
				r.db.Model(&applicationRow{}).Count(&count)
				if count != 0 {
					t.Fatal("invalid application persisted")
				}
			}
		})
	}
}

type refundCapacityTrading struct {
	remaining int64
	calls     int
}

func (p *refundCapacityTrading) ExecuteServiceCommand(context.Context, e.FinancialCommand) (e.FinancialResult, error) {
	return e.FinancialResult{}, e.ErrUnavailable
}
func (p *refundCapacityTrading) ReadServiceRefundableAmount(context.Context, string) (int64, error) {
	p.calls++
	return p.remaining, nil
}
func (p *refundCapacityTrading) AdmitServiceRefundReview(_ context.Context, in e.RefundReviewAdmission) (e.RefundReviewProof, error) {
	p.calls++
	if in.AmountMinor > p.remaining {
		return e.RefundReviewProof{}, e.ErrInvalid
	}
	proof := e.RefundReviewProof{ReceiptID: "exact-review-proof", InputFingerprint: e.Fingerprint(in), PaymentReceiptID: in.PaymentReceiptID, RemainingMinor: p.remaining}
	proof.ResultFingerprint = proof.Fingerprint()
	return proof, nil
}

func TestRefundProposalAndApprovalReadCanonicalRemainingAmount(t *testing.T) {
	r, _ := fixture(t)
	p := &refundCapacityTrading{remaining: 40}
	s, err := e.NewService(r, p, 180)
	if err != nil {
		t.Fatal(err)
	}
	req := e.Request{ID: uuid.NewString(), OrderID: uuid.NewString(), PaymentReceiptID: "original-payment", BuyerOrganizationID: "buyer", ProviderOrganizationID: "provider", State: "SERVICING", Version: 1, Quote: &e.Quote{AmountMinor: 100, Version: 1}}
	if err := r.db.Create(requestRecord(req)).Error; err != nil {
		t.Fatal(err)
	}
	buyer := e.Scope{OrganizationID: "buyer", ActorID: "b"}
	_, err = s.Mutate(context.Background(), e.Command{Scope: buyer, Key: uuid.NewString(), ID: req.ID, Kind: "refund_propose", Version: 1, RefundAmountMinor: 60, Reason: "second partial refund"})
	if !errors.Is(err, e.ErrInvalid) {
		t.Fatalf("over-refund proposal accepted: %v", err)
	}
	if p.calls != 1 {
		t.Fatal("proposal skipped canonical funds")
	}
	valid := e.Command{Scope: buyer, Key: uuid.NewString(), ID: req.ID, Kind: "refund_propose", Version: 1, RefundAmountMinor: 40, Reason: "remaining amount"}
	first, err := s.Mutate(context.Background(), valid)
	if err != nil {
		t.Fatal(err)
	}
	calls := p.calls
	p.remaining = 0
	again, err := s.Mutate(context.Background(), valid)
	if err != nil || again.Request.Version != first.Request.Version || p.calls != calls {
		t.Fatal("replay depends on changed canonical refund amount", err)
	}
	changed := valid
	changed.RefundAmountMinor = 20
	if _, err := s.Mutate(context.Background(), changed); !errors.Is(err, e.ErrConflict) {
		t.Fatal("changed same-key proposal accepted", err)
	}
	p.remaining = 40
	req.Refund = &e.RefundAgreement{Version: 2, AmountMinor: 60, Reason: "old proposal", State: "NEGOTIATING", BuyerConfirmed: true, ProviderConfirmed: true}
	req.FinancialFence = true
	if err := r.db.Save(requestRecord(req)).Error; err != nil {
		t.Fatal(err)
	}
	_, err = s.Mutate(context.Background(), e.Command{Scope: e.Scope{Platform: true, ActorID: "admin"}, Key: uuid.NewString(), ID: req.ID, Kind: "refund_review", Version: 1, RefundVersion: 2, Reason: "approved"})
	if !errors.Is(err, e.ErrInvalid) {
		t.Fatalf("over-refund approval accepted: %v", err)
	}
	var count int64
	r.db.Model(&financialRow{}).Count(&count)
	if count != 0 {
		t.Fatal("invalid financial command persisted")
	}
}

func TestRejectedDeliveryReasonSurvivesReadReplayAndReplacement(t *testing.T) {
	r, s, _, _, _, req, _ := fulfillmentFundsFixture(t)
	req.State = "AWAITING_ACCEPTANCE"
	req.Delivery = &e.Delivery{Version: 3, Content: "delivery", SubmittedAt: time.Now().UTC()}
	if err := r.db.Save(requestRecord(req)).Error; err != nil {
		t.Fatal(err)
	}
	c := e.Command{Scope: e.Scope{OrganizationID: "buyer", ActorID: "buyer-user"}, Key: uuid.NewString(), ID: req.ID, Kind: "reject", Version: req.Version, DeliveryVersion: 3, Reason: "Missing registration document"}
	first, err := s.Mutate(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Mutate(context.Background(), c)
	if err != nil || again.Request.Version != first.Request.Version {
		t.Fatal("rejection replay changed result", err)
	}
	page, err := s.Read(context.Background(), e.Query{Scope: e.Scope{OrganizationID: "provider", ActorID: "p"}, Kind: "requests", ID: req.ID, Page: 1, PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(page.Requests[0].Delivery)
	var delivery map[string]json.RawMessage
	json.Unmarshal(raw, &delivery)
	var rejection struct {
		Reason          string
		ActorID         string
		DeliveryVersion string
		RejectedAt      time.Time
	}
	if json.Unmarshal(delivery["rejection"], &rejection) != nil || rejection.Reason != c.Reason || rejection.ActorID != c.Scope.ActorID || rejection.DeliveryVersion != "3" || rejection.RejectedAt.IsZero() {
		t.Fatalf("provider lost durable rejection: %s", raw)
	}
	_, err = s.Mutate(context.Background(), e.Command{Scope: e.Scope{OrganizationID: "provider", ActorID: "p"}, Key: uuid.NewString(), ID: req.ID, Kind: "deliver", Version: first.Request.Version, Delivery: &e.Delivery{Content: "corrected delivery"}})
	if err != nil {
		t.Fatal(err)
	}
	var original versionRow
	if err := r.db.Where("id=? AND kind=? AND version=?", req.ID, "REQUEST", first.Request.Version).Take(&original).Error; err != nil {
		t.Fatal(err)
	}
	var historical e.Request
	json.Unmarshal(original.Payload, &historical)
	raw, _ = json.Marshal(historical.Delivery)
	json.Unmarshal(raw, &delivery)
	if string(delivery["rejection"]) == "null" || len(delivery["rejection"]) == 0 {
		t.Fatal("replacement erased original rejection")
	}
}
