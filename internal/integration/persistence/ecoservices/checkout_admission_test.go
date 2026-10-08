package ecoservices

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	e "task-processor/internal/ecoservices"
)

func checkoutAdmissionFixture(t *testing.T, r *Repository) (e.Request, e.FinancialCommand) {
	t.Helper()
	req := e.Request{ID: uuid.NewString(), OrderID: uuid.NewString(), BuyerOrganizationID: "buyer", ProviderOrganizationID: "provider", State: "ORDER_PENDING", Version: 1, Quote: &e.Quote{AmountMinor: 101, DeliveryDays: 1, Version: 1, CommissionBPS: 1000, AllocationBasis: "CUMULATIVE_NET_FLOOR_V1", PolicyVersion: e.PolicyVersion}, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := r.db.Create(requestRecord(req)).Error; err != nil {
		t.Fatal(err)
	}
	c := e.FinancialCommand{ID: uuid.NewString(), RequestID: req.ID, OrderID: req.OrderID, Kind: "CREATE_PURCHASE", SourceProofID: uuid.NewString(), ActorID: "buyer", BuyerOrganizationID: "buyer", ProviderOrganizationID: "provider", MerchantID: "original-sub", Quote: *req.Quote, AmountMinor: 101, PolicyVersion: e.PolicyVersion, State: "PENDING"}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.db.Create(&financialRow{ID: c.ID, RequestID: c.RequestID, OrderID: c.OrderID, Kind: c.Kind, Fingerprint: e.Fingerprint(c), Payload: raw, State: c.State, CreatedAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	return req, c
}

func assertCheckoutReplayRejectsCommittedCancellation(t *testing.T, ctx context.Context, r *Repository, s *e.Service) {
	t.Helper()
	req, c := checkoutAdmissionFixture(t, r)
	if err := r.db.Create(applicationRecord(e.Application{ID: uuid.NewString(), OrganizationID: "provider", State: "ACTIVE", Version: 1, MerchantID: "original-sub"})).Error; err != nil {
		t.Fatal(err)
	}
	c.DispatchOperationID = "checkout:" + c.ID
	for _, key := range []string{c.DispatchOperationID, "", c.DispatchOperationID} {
		c.DispatchOperationID = key
		if admitted, err := r.AdmitFinancialCommand(ctx, c); err != nil || !admitted.DispatchAdmitted || admitted.ID != c.ID {
			t.Fatalf("original live checkout cannot be admitted/replayed: %+v %v", admitted, err)
		}
	}
	// Cancellation commits after the prior buyer authorization/admission and
	// before checkout's final E check. That check must read the current request.
	cancelled, err := s.Mutate(ctx, e.Command{Scope: e.Scope{OrganizationID: "buyer", ActorID: "buyer"}, Key: uuid.NewString(), Kind: "cancel", ID: req.ID, Version: req.Version})
	if err != nil || cancelled.Request.State != "CANCEL_REQUESTED" {
		t.Fatal("cancellation did not commit", err)
	}
	for _, key := range []string{c.DispatchOperationID, "", "another-checkout:" + c.ID} {
		c.DispatchOperationID = key
		if _, err := r.AdmitFinancialCommand(ctx, c); !errors.Is(err, e.ErrConflict) {
			t.Errorf("checkout replay %q ignored committed cancellation: %v", key, err)
		}
	}
	var count int64
	if err := r.db.Model(&operationRow{}).Where("organization_id=? AND kind=?", "SYSTEM_FINANCE", "financial_dispatch").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("rejected checkout changed admission receipts: count=%d err=%v", count, err)
	}
}

func TestCheckoutAdmissionReplayRejectsCommittedCancellation(t *testing.T) {
	r, s := fixture(t)
	assertCheckoutReplayRejectsCommittedCancellation(t, context.Background(), r, s)
}

func TestCheckoutAdmissionReplayHonorsCurrentFenceAndPaymentState(t *testing.T) {
	for _, state := range []string{"ORDER_PENDING", "PAID_READY", "CANCELLED"} {
		t.Run(state, func(t *testing.T) {
			r, _ := fixture(t)
			req, c := checkoutAdmissionFixture(t, r)
			c.DispatchOperationID = "checkout:" + c.ID
			if _, err := r.AdmitFinancialCommand(context.Background(), c); err != nil {
				t.Fatal(err)
			}
			req.State = state
			req.FinancialFence = state == "ORDER_PENDING"
			if err := r.db.Save(requestRecord(req)).Error; err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{c.DispatchOperationID, ""} {
				c.DispatchOperationID = key
				if _, err := r.AdmitFinancialCommand(context.Background(), c); !errors.Is(err, e.ErrConflict) {
					t.Errorf("prior admission %q replaced live %s check: %v", key, state, err)
				}
			}
		})
	}
}
