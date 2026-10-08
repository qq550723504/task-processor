package ecoservices

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	e "task-processor/internal/ecoservices"
)

func assertLatePaymentReopensOnlyOriginalCancellation(t *testing.T, ctx context.Context, r *Repository, s *e.Service) {
	t.Helper()
	req, create := checkoutAdmissionFixture(t, r)
	if err := r.db.Create(applicationRecord(e.Application{ID: uuid.NewString(), OrganizationID: "provider", State: "ACTIVE", Version: 1, MerchantID: "original-sub"})).Error; err != nil {
		t.Fatal(err)
	}
	create.DispatchOperationID = "checkout:" + create.ID
	if _, err := r.AdmitFinancialCommand(ctx, create); err != nil {
		t.Fatal(err)
	}
	create.DispatchOperationID = ""
	if _, err := s.Mutate(ctx, e.Command{Scope: e.Scope{OrganizationID: "buyer", ActorID: "buyer"}, Key: uuid.NewString(), Kind: "cancel", ID: req.ID, Version: req.Version}); err != nil {
		t.Fatal(err)
	}
	var cancelRow financialRow
	if err := r.db.Where("order_id=? AND kind='CANCEL'", req.OrderID).Take(&cancelRow).Error; err != nil {
		t.Fatal(err)
	}
	cancel, err := financialFact(cancelRow)
	if err != nil {
		t.Fatal(err)
	}
	cancel.DispatchOperationID = "close:" + cancel.ID
	if _, err := r.AdmitFinancialCommand(ctx, cancel); err != nil {
		t.Fatal(err)
	}
	cancel.DispatchOperationID = ""
	closed := e.FinancialResult{OrderID: req.OrderID, State: "CLOSED_UNPAID", ReceiptID: "channel-closed:original", Revision: 10}
	if err := r.CompleteFinancialCommand(ctx, cancel, closed); err != nil {
		t.Fatal(err)
	}
	if err := r.WakeOriginalServicePurchase(ctx, req.OrderID); err != nil {
		t.Fatal(err)
	}
	commands, err := r.PendingFinancialCommands(ctx, 20)
	if err != nil || len(commands) != 1 || commands[0].ID != create.ID {
		t.Fatalf("verified payment did not wake original create: %+v %v", commands, err)
	}
	create = commands[0]
	paid := e.FinancialResult{OrderID: req.OrderID, State: "CANCELLATION_PENDING", PaymentReceiptID: "original-canonical-paid", Revision: 12}
	if err := r.CompleteFinancialCommand(ctx, create, paid); err != nil {
		t.Fatal(err)
	}
	if err := r.db.Where("id=?", cancel.ID).Take(&cancelRow).Error; err != nil {
		t.Fatal(err)
	}
	if cancelRow.State != "PROCESSING" || cancelRow.RecoveryGeneration != 1 || cancelRow.Fingerprint != e.Fingerprint(cancel) || !cancelRow.DispatchAdmitted || string(cancelRow.Result) != mustJSON(t, closed) {
		t.Fatalf("original cancellation/proof was not reactivated: %+v", cancelRow)
	}
	page, err := r.Read(ctx, e.Query{Scope: e.Scope{OrganizationID: "buyer"}, Kind: "requests", ID: req.ID, Page: 1, PageSize: 1})
	if err != nil || len(page.Requests) != 1 || page.Requests[0].State != "CANCEL_REQUESTED" || page.Requests[0].FinancialState != "CANCELLATION_PENDING" || page.Requests[0].PaymentReceiptID != paid.PaymentReceiptID {
		t.Fatalf("late payment became deliverable or retained unpaid closure: %+v %v", page, err)
	}
	// An old cancellation worker must not erase the newer verified payment wake.
	if err := r.CompleteFinancialCommand(ctx, cancel, closed); err != nil {
		t.Fatal(err)
	}
	commands, err = r.PendingFinancialCommands(ctx, 20)
	if err != nil || len(commands) != 1 || commands[0].ID != cancel.ID || commands[0].RecoveryGeneration != 1 {
		t.Fatalf("old worker drained late cancellation: %+v %v", commands, err)
	}
	cancel = commands[0]
	cancel.DispatchOperationID = "original-late-refund:" + cancel.ID
	if _, err := r.AdmitFinancialCommand(ctx, cancel); err != nil {
		t.Fatalf("original cancellation cannot admit its refund: %v", err)
	}
	cancel.DispatchOperationID = ""
	refunded := e.FinancialResult{OrderID: req.OrderID, State: "REFUNDED", PaymentReceiptID: paid.PaymentReceiptID, ReceiptID: "original-full-refund", FullRefund: true, Revision: 20}
	if err := r.CompleteFinancialCommand(ctx, cancel, refunded); err != nil {
		t.Fatal(err)
	}
	// Repeated/stale paid projections cannot reopen a completed full refund.
	if err := r.CompleteFinancialCommand(ctx, create, paid); err != nil {
		t.Fatal(err)
	}
	if err := r.WakeOriginalServicePurchase(ctx, req.OrderID); err != nil {
		t.Fatal(err)
	}
	commands, err = r.PendingFinancialCommands(ctx, 20)
	if err != nil || len(commands) != 1 || commands[0].ID != create.ID {
		t.Fatalf("duplicate notification created another cancellation: %+v %v", commands, err)
	}
	if err := r.CompleteFinancialCommand(ctx, commands[0], refunded); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := r.db.Model(&financialRow{}).Where("order_id=?", req.OrderID).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("recovery generated a new command: %d %v", count, err)
	}
	page, _ = r.Read(ctx, e.Query{Scope: e.Scope{OrganizationID: "buyer"}, Kind: "requests", ID: req.ID, Page: 1, PageSize: 1})
	if page.Requests[0].State != "CANCELLED" || page.Requests[0].FinancialState != "REFUNDED" {
		t.Fatalf("stale wake changed confirmed refund: %+v", page.Requests[0])
	}
	if remaining, err := r.PendingFinancialCommands(ctx, 20); err != nil || len(remaining) != 0 {
		t.Fatalf("duplicate notification left busy recovery: %+v %v", remaining, err)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestLatePaymentReopensOnlyOriginalCancellation(t *testing.T) {
	r, s := fixture(t)
	assertLatePaymentReopensOnlyOriginalCancellation(t, context.Background(), r, s)
}

func TestLateCancellationRequiresTrustedPaymentAndCurrentClosure(t *testing.T) {
	for _, state := range []string{"CLOSED_UNPAID", "REFUNDED", "RECONCILIATION_REQUIRED"} {
		r, _ := fixture(t)
		req, c := checkoutAdmissionFixture(t, r)
		req.State, req.FinancialState, req.FinancialRevision = "CANCELLED", state, 20
		if err := r.db.Save(requestRecord(req)).Error; err != nil {
			t.Fatal(err)
		}
		// Neither an unpaid observation nor a stale payment receipt can reopen it.
		for _, result := range []e.FinancialResult{
			{OrderID: req.OrderID, State: "CANCELLATION_PENDING", Revision: 21},
			{OrderID: req.OrderID, State: "CANCELLATION_PENDING", PaymentReceiptID: "stale-paid", Revision: 19},
		} {
			if err := r.CompleteFinancialCommand(context.Background(), c, result); err != nil {
				t.Fatal(err)
			}
			var current requestRow
			if err := r.db.Where("id=?", req.ID).Take(&current).Error; err != nil || current.State != "CANCELLED" {
				t.Fatalf("unproven/stale observation reopened %s: %+v %v", state, current, err)
			}
		}
	}
}
