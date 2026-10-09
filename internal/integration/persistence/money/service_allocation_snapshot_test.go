package money

import (
	"context"
	"encoding/json"
	m "task-processor/internal/ledger/money"
	"testing"
)

func withAllocation(t *testing.T, in m.ServicePaymentInput, bps int64) m.ServicePaymentInput {
	t.Helper()
	raw, _ := json.Marshal(in)
	var fields map[string]any
	_ = json.Unmarshal(raw, &fields)
	fields["Allocation"] = map[string]any{"CommissionBPS": bps, "Basis": "CUMULATIVE_NET_FLOOR_V1"}
	raw, _ = json.Marshal(fields)
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	return in
}

func TestServiceFundsUsePersistedAllocationSnapshot(t *testing.T) {
	r := newMoneyRepository(t)
	in := withAllocation(t, servicePayment(), 2000)
	ctx := context.Background()
	if _, err := r.AcceptServicePayment(ctx, in); err != nil {
		t.Fatal(err)
	}
	f, err := r.ReadServiceFunds(ctx, in.OrderID)
	if err != nil || f.PlatformMinor != 20 || f.ProviderMinor != 81 {
		t.Fatalf("persisted 20 percent snapshot ignored: %+v %v", f, err)
	}
	if _, err := r.ReadServicePayment(ctx, withAllocation(t, in, 1000)); err != m.ErrConflict {
		t.Fatalf("changed original rate replay accepted: %v", err)
	}
	applyServiceEffect(t, r, m.ServiceShare, "share", 20)
	applyServiceEffect(t, r, m.ServiceFinish, "finish", 81)
	applyServiceEffect(t, r, m.ServiceReturn, "return", 1)
	applyServiceEffect(t, r, m.ServiceRefund, "refund", 2)
	f, err = r.ReadServiceFunds(ctx, in.OrderID)
	if err != nil || f.PlatformMinor != 19 || f.ProviderMinor != 80 || f.RefundedMinor != 2 {
		t.Fatalf("refund ignored original allocation snapshot: %+v %v", f, err)
	}
	if err := r.db.Model(&servicePaymentRow{}).Where("order_id=?", in.OrderID).Update("input", []byte(`{}`)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadServiceFunds(ctx, in.OrderID); err != m.ErrConflict {
		t.Fatalf("missing snapshot fell back to current fee: %v", err)
	}
}
