package commercialbilling

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/ledger/orgresource"
)

func commercialRepository(t *testing.T) *Repository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:commercial-owner-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repository, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	repository.now = func() time.Time { return time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC) }
	return repository
}

func TestQuoteIsServerPricedAndOrderReplayRejectsChangedPayload(t *testing.T) {
	repository := commercialRepository(t)
	offer := billing.Offer{OfferID: "offer-ai", ProductKind: billing.ProductAIPoint, ResourceType: orgresource.ResourceAIPoint, Currency: billing.CurrencyCNY, UnitPriceMinor: 7, PricingVersion: "pricing-1", MinQuantity: 1, MaxQuantity: 1000, Status: billing.OfferActive}
	if err := repository.SaveOffer(context.Background(), offer); err != nil {
		t.Fatal(err)
	}
	quote, err := repository.CreateQuote(context.Background(), billing.QuoteRequest{OrganizationID: "org-a", OfferID: "offer-ai", Quantity: 10})
	if err != nil || quote.TotalMinor != 70 {
		t.Fatalf("quote=%#v err=%v", quote, err)
	}
	order, err := repository.CreatePendingResourceOrder(context.Background(), billing.CreateResourceOrderRequest{OrganizationID: "org-a", QuoteID: quote.QuoteID, IdempotencyKey: "idem-1"}, quote)
	if err != nil || order.Status != billing.OrderPending || order.AmountMinor != 70 {
		t.Fatalf("order=%#v err=%v", order, err)
	}
	replayed, err := repository.CreatePendingResourceOrder(context.Background(), billing.CreateResourceOrderRequest{OrganizationID: "org-a", QuoteID: quote.QuoteID, IdempotencyKey: "idem-1"}, quote)
	if err != nil || replayed.OrderID != order.OrderID {
		t.Fatalf("replayed=%#v err=%v", replayed, err)
	}
	_, err = repository.CreatePendingResourceOrder(context.Background(), billing.CreateResourceOrderRequest{OrganizationID: "org-a", QuoteID: quote.QuoteID, IdempotencyKey: "idem-1"}, func() billing.Quote { changed := quote; changed.ResourceQuantity = 11; return changed }())
	if !errors.Is(err, billing.ErrConflict) {
		t.Fatalf("changed idempotency payload err=%v", err)
	}
}

func TestQuoteFailsClosedWithoutApprovedPrice(t *testing.T) {
	repository := commercialRepository(t)
	offer := billing.Offer{OfferID: "offer-unpriced", ProductKind: billing.ProductDataRow, ResourceType: orgresource.ResourceDataRow, Currency: billing.CurrencyCNY, PricingVersion: "pricing-1", MinQuantity: 1, MaxQuantity: 10, Status: billing.OfferActive}
	if err := repository.SaveOffer(context.Background(), offer); !errors.Is(err, billing.ErrInvalid) {
		t.Fatalf("unpriced offer save err=%v", err)
	}
	if _, err := repository.CreateQuote(context.Background(), billing.QuoteRequest{OrganizationID: "org-a", OfferID: "offer-unpriced", Quantity: 1}); !errors.Is(err, billing.ErrOfferUnavailable) {
		t.Fatalf("unpriced quote err=%v", err)
	}
}
