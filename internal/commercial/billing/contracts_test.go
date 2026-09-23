package billing

import (
	"errors"
	"testing"
	"time"

	"task-processor/internal/ledger/money"
	"task-processor/internal/ledger/orgresource"
)

func TestResourceTypeForProductUsesCanonicalOrgResourceTypes(t *testing.T) {
	tests := []struct {
		product ProductKind
		want    orgresource.ResourceType
	}{
		{ProductStoreRenewalPeriod, orgresource.ResourceStoreRenewalPeriod},
		{ProductAIPoint, orgresource.ResourceAIPoint},
		{ProductDataRow, orgresource.ResourceDataRow},
	}
	for _, tt := range tests {
		got, ok := ResourceTypeForProduct(tt.product)
		if !ok || got != tt.want {
			t.Fatalf("%s maps to %q, %v", tt.product, got, ok)
		}
	}
	if _, ok := ResourceTypeForProduct("MODEL_TOKEN"); ok {
		t.Fatalf("unknown product kind must not become a resource")
	}
}

func TestQuoteValidationRequiresServerMoneySnapshot(t *testing.T) {
	now := time.Now().UTC()
	quote := Quote{
		QuoteID:          "quote-1",
		OrganizationID:   "org-1",
		OfferID:          "offer-1",
		ProductKind:      ProductAIPoint,
		ResourceType:     orgresource.ResourceAIPoint,
		ResourceQuantity: 100,
		Currency:         CurrencyCNY,
		TotalMinor:       1999,
		PricingVersion:   "v1",
		CreatedAt:        now,
		ExpiresAt:        now.Add(time.Minute),
		Fingerprint:      "fingerprint",
	}
	if err := quote.Validate(); err != nil {
		t.Fatalf("valid quote rejected: %v", err)
	}

	quote.TotalMinor = 0
	if !errors.Is(quote.Validate(), ErrInvalid) {
		t.Fatalf("quote without authoritative positive total must be invalid")
	}
}

func TestResourcePurchaseOrderBindsQuoteItemAndAmount(t *testing.T) {
	now := time.Now().UTC()
	order := Order{
		OrderID:            "order-1",
		OrganizationID:     "org-1",
		Kind:               OrderResourcePurchase,
		QuoteID:            "quote-1",
		Currency:           CurrencyCNY,
		AmountMinor:        1999,
		Status:             OrderPending,
		IdempotencyKey:     "idem-1",
		RequestFingerprint: "fp-1",
		Version:            1,
		CreatedAt:          now,
		UpdatedAt:          now,
		Items: []OrderItem{{
			OrderItemID:      "item-1",
			ProductKind:      ProductAIPoint,
			ResourceType:     orgresource.ResourceAIPoint,
			ResourceQuantity: 100,
			AmountMinor:      1999,
		}},
	}
	if err := order.Validate(); err != nil {
		t.Fatalf("valid resource order rejected: %v", err)
	}

	order.Items[0].AmountMinor = 2000
	if !errors.Is(order.Validate(), ErrInvalid) {
		t.Fatalf("order amount must match immutable item amount")
	}
}

func TestOrderValidationRequiresPositiveVersion(t *testing.T) {
	order := validResourcePurchaseOrder(time.Now().UTC())
	order.Version = 0
	if !errors.Is(order.Validate(), ErrInvalid) {
		t.Fatalf("order without a positive version = nil, want ErrInvalid")
	}

	order.Version = -1
	if !errors.Is(order.Validate(), ErrInvalid) {
		t.Fatalf("order with a negative version = nil, want ErrInvalid")
	}
}

func TestWalletTopUpOrderCannotCarryResourceItem(t *testing.T) {
	now := time.Now().UTC()
	order := Order{
		OrderID:            "order-1",
		OrganizationID:     "org-1",
		Kind:               OrderWalletTopUp,
		Currency:           CurrencyCNY,
		AmountMinor:        10000,
		Status:             OrderPending,
		IdempotencyKey:     "idem-1",
		RequestFingerprint: "fp-1",
		Version:            1,
		CreatedAt:          now,
		UpdatedAt:          now,
		Items: []OrderItem{{
			OrderItemID:      "item-1",
			ProductKind:      ProductDataRow,
			ResourceType:     orgresource.ResourceDataRow,
			ResourceQuantity: 10,
			AmountMinor:      10000,
		}},
	}
	if !errors.Is(order.Validate(), ErrInvalid) {
		t.Fatalf("wallet top-up must not mint resources directly")
	}
}

func TestFulfilledWalletTopUpRequiresPaymentProof(t *testing.T) {
	now := time.Now().UTC()
	order := validWalletTopUpOrder(now)
	order.Status = OrderFulfilled
	if !errors.Is(order.Validate(), ErrInvalid) {
		t.Fatalf("fulfilled wallet top-up without payment proof = nil, want ErrInvalid")
	}

	order.PaymentID = "payment-1"
	if err := order.Validate(); err != nil {
		t.Fatalf("fulfilled wallet top-up with payment proof rejected: %v", err)
	}
}

func TestPendingWalletTopUpRejectsPaymentEvidence(t *testing.T) {
	order := validWalletTopUpOrder(time.Now().UTC())
	order.PaymentID = "payment-1"

	if !errors.Is(order.Validate(), ErrInvalid) {
		t.Fatalf("pending wallet top-up with payment evidence = nil, want ErrInvalid")
	}
}

func TestCancelledWalletTopUpRejectsPaymentEvidence(t *testing.T) {
	order := validWalletTopUpOrder(time.Now().UTC())
	order.Status = OrderCancelled
	order.PaymentID = "payment-1"

	if !errors.Is(order.Validate(), ErrInvalid) {
		t.Fatalf("cancelled wallet top-up with payment evidence = nil, want ErrInvalid")
	}
}

func TestWalletTopUpRejectsResourcePurchaseLifecycleStates(t *testing.T) {
	now := time.Now().UTC()
	for _, status := range []OrderStatus{
		OrderFundsReserved,
		OrderFulfilling,
		OrderReconciliationRequired,
	} {
		order := validWalletTopUpOrder(now)
		order.Status = status
		order.PaymentID = "payment-1"
		if !errors.Is(order.Validate(), ErrInvalid) {
			t.Fatalf("wallet top-up status %s = nil, want ErrInvalid", status)
		}
	}
}

func TestResourcePurchasePostReservationStatesRequireWalletReservation(t *testing.T) {
	now := time.Now().UTC()
	for _, status := range []OrderStatus{
		OrderFundsReserved,
		OrderFulfilling,
		OrderFulfilled,
		OrderReconciliationRequired,
	} {
		order := validResourcePurchaseOrder(now)
		order.Status = status
		if err := order.Validate(); !errors.Is(err, ErrInvalid) {
			t.Fatalf("status %s without wallet reservation = %v, want ErrInvalid", status, err)
		}
	}
}

func TestFulfilledResourcePurchaseRequiresGrantProof(t *testing.T) {
	order := validResourcePurchaseOrder(time.Now().UTC())
	order.Status = OrderFulfilled
	order.WalletReservationID = "reservation-1"
	if !errors.Is(order.Validate(), ErrInvalid) {
		t.Fatalf("fulfilled resource order without grant proof = nil, want ErrInvalid")
	}

	order.ResourceGrantOperationID = "grant-operation-1"
	order.ResourceGrantSourceType = orgresource.SourceCommercialOrderItem
	order.ResourceGrantSourceIdentity = orgresource.CommercialOrderItemSourceIdentity(order.OrganizationID, order.OrderID, order.Items[0].OrderItemID)
	order.WalletReservationState = money.WalletReservationReserved
	if !errors.Is(order.Validate(), ErrInvalid) {
		t.Fatalf("fulfilled resource order with uncommitted wallet reservation = nil, want ErrInvalid")
	}

	order.WalletReservationState = money.WalletReservationCommitted
	if err := order.Validate(); err != nil {
		t.Fatalf("fulfilled resource order with grant proof rejected: %v", err)
	}

	order.ResourceGrantSourceIdentity = "commercial_order_item:other-order-proof"
	if !errors.Is(order.Validate(), ErrInvalid) {
		t.Fatalf("fulfilled resource order with copied grant source identity = nil, want ErrInvalid")
	}

	order.ResourceGrantSourceType = "arbitrary-source"
	if !errors.Is(order.Validate(), ErrInvalid) {
		t.Fatalf("fulfilled resource order with non-canonical grant source = nil, want ErrInvalid")
	}
}

func TestCancelledResourcePurchaseRejectsGrantEvidence(t *testing.T) {
	order := validResourcePurchaseOrder(time.Now().UTC())
	order.Status = OrderCancelled
	order.ResourceGrantOperationID = "grant-operation-1"
	order.ResourceGrantSourceType = orgresource.SourceCommercialOrderItem
	order.ResourceGrantSourceIdentity = orgresource.CommercialOrderItemSourceIdentity(order.OrganizationID, order.OrderID, order.Items[0].OrderItemID)

	if !errors.Is(order.Validate(), ErrInvalid) {
		t.Fatalf("cancelled resource order with grant evidence = nil, want ErrInvalid")
	}
}

func TestCancelledResourcePurchaseRejectsCommittedReservation(t *testing.T) {
	order := validResourcePurchaseOrder(time.Now().UTC())
	order.Status = OrderCancelled
	order.WalletReservationID = "reservation-1"
	order.WalletReservationState = money.WalletReservationCommitted

	if !errors.Is(order.Validate(), ErrInvalid) {
		t.Fatalf("cancelled resource order with committed reservation = nil, want ErrInvalid")
	}
}

func TestResourcePurchaseSourceIDsMustBeCanonical(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Order)
	}{
		{name: "order id", mutate: func(order *Order) { order.OrderID = " order-1" }},
		{name: "organization id", mutate: func(order *Order) { order.OrganizationID = "org-1 " }},
		{name: "order item id", mutate: func(order *Order) { order.Items[0].OrderItemID = " item-1" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			order := validResourcePurchaseOrder(time.Now().UTC())
			test.mutate(&order)
			if !errors.Is(order.Validate(), ErrInvalid) {
				t.Fatalf("resource order with non-canonical %s = nil, want ErrInvalid", test.name)
			}
		})
	}
}

func validResourcePurchaseOrder(now time.Time) Order {
	return Order{
		OrderID:            "order-1",
		OrganizationID:     "org-1",
		Kind:               OrderResourcePurchase,
		QuoteID:            "quote-1",
		Currency:           CurrencyCNY,
		AmountMinor:        1999,
		Status:             OrderPending,
		IdempotencyKey:     "idem-1",
		RequestFingerprint: "fp-1",
		Version:            1,
		CreatedAt:          now,
		UpdatedAt:          now,
		Items: []OrderItem{{
			OrderItemID:      "item-1",
			ProductKind:      ProductAIPoint,
			ResourceType:     orgresource.ResourceAIPoint,
			ResourceQuantity: 100,
			AmountMinor:      1999,
		}},
	}
}

func validWalletTopUpOrder(now time.Time) Order {
	return Order{
		OrderID:            "order-1",
		OrganizationID:     "org-1",
		Kind:               OrderWalletTopUp,
		Currency:           CurrencyCNY,
		AmountMinor:        10000,
		Status:             OrderPending,
		IdempotencyKey:     "idem-1",
		RequestFingerprint: "fp-1",
		Version:            1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}
