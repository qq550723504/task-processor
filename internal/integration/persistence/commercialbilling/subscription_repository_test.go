package commercialbilling

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/commercial/billing"
)

func TestSubscriptionOfferQuoteAndOrderStayServerOwnedAndReplayable(t *testing.T) {
	repository := commercialRepository(t)
	ctx := context.Background()
	offer := billing.Offer{OfferID: "professional-zero", ProductKind: billing.ProductSubscriptionPlan, PlanCode: "professional", TermMonths: 1, SettlementMode: billing.SettlementZeroPrice, Currency: billing.CurrencyCNY, UnitPriceMinor: 0, PricingVersion: "pricing-1", Status: billing.OfferActive}
	if err := repository.SaveOffer(ctx, offer); err != nil {
		t.Fatal(err)
	}
	offers, err := repository.ListSubscriptionOffers(ctx)
	if err != nil || len(offers) != 1 || offers[0] != offer {
		t.Fatalf("offers = %+v, err = %v", offers, err)
	}

	plan := billing.SubscriptionPlanSnapshot{PlanCode: "professional", DisplayName: "Professional", Fingerprint: "plan-fingerprint"}
	quote, err := repository.CreateSubscriptionQuote(ctx, billing.SubscriptionQuoteRequest{OrganizationID: "org-a", OfferID: offer.OfferID}, offer, plan)
	if err != nil {
		t.Fatal(err)
	}
	if quote.OrganizationID != "org-a" || quote.PlanCode != plan.PlanCode || quote.PlanFingerprint != plan.Fingerprint || quote.TermMonths != 1 || quote.SettlementMode != billing.SettlementZeroPrice || quote.TotalMinor != 0 || quote.Fingerprint == "" {
		t.Fatalf("quote = %+v", quote)
	}

	request := billing.CreateSubscriptionOrderRequest{OrganizationID: "org-a", ActorID: "actor-a", QuoteID: quote.QuoteID, IdempotencyKey: "idem-subscription-1"}
	order, err := repository.CreatePendingSubscriptionOrder(ctx, request, quote)
	if err != nil {
		t.Fatal(err)
	}
	if order.Kind != billing.OrderSubscriptionPurchase || order.ProductKind != billing.ProductSubscriptionPlan || order.ActorID != request.ActorID || order.PlanCode != quote.PlanCode || order.PlanFingerprint != quote.PlanFingerprint || order.SettlementMode != billing.SettlementZeroPrice || order.Status != billing.OrderPending || len(order.Items) != 0 {
		t.Fatalf("order = %+v", order)
	}
	replayed, err := repository.CreatePendingSubscriptionOrder(ctx, request, quote)
	if err != nil || replayed.OrderID != order.OrderID {
		t.Fatalf("replayed = %+v, err = %v", replayed, err)
	}
	changed := request
	changed.ActorID = "actor-b"
	if _, err := repository.CreatePendingSubscriptionOrder(ctx, changed, quote); !errors.Is(err, billing.ErrConflict) {
		t.Fatalf("changed actor replay error = %v", err)
	}
}

func TestSubscriptionOrderEffectAdmissionUsesVersionAndTerminalFence(t *testing.T) {
	repository := commercialRepository(t)
	ctx := context.Background()
	offer := billing.Offer{OfferID: "professional-wallet", ProductKind: billing.ProductSubscriptionPlan, PlanCode: "professional", TermMonths: 1, SettlementMode: billing.SettlementWallet, Currency: billing.CurrencyCNY, UnitPriceMinor: 999, PricingVersion: "pricing-1", Status: billing.OfferActive}
	if err := repository.SaveOffer(ctx, offer); err != nil {
		t.Fatal(err)
	}
	quote, err := repository.CreateSubscriptionQuote(ctx, billing.SubscriptionQuoteRequest{OrganizationID: "org-a", OfferID: offer.OfferID}, offer, billing.SubscriptionPlanSnapshot{PlanCode: "professional", DisplayName: "Professional", Fingerprint: "plan-fingerprint"})
	if err != nil {
		t.Fatal(err)
	}
	order, err := repository.CreatePendingSubscriptionOrder(ctx, billing.CreateSubscriptionOrderRequest{OrganizationID: "org-a", ActorID: "actor-a", QuoteID: quote.QuoteID, IdempotencyKey: "idem-subscription-admit"}, quote)
	if err != nil {
		t.Fatal(err)
	}
	admittedAt := time.Date(2026, 9, 24, 1, 0, 0, 0, time.UTC)
	admitted, won, err := repository.AdmitSubscriptionOrderEffect(ctx, order.OrganizationID, order.OrderID, order.Version, billing.PendingSubscriptionOrderEffectReserve, admittedAt)
	if err != nil || !won || admitted.PendingEffect != billing.PendingSubscriptionOrderEffectReserve || admitted.PendingEffectAdmittedAt == nil || !admitted.PendingEffectAdmittedAt.Equal(admittedAt) {
		t.Fatalf("admitted = %+v, won = %v, err = %v", admitted, won, err)
	}
	_, won, err = repository.AdmitSubscriptionOrderEffect(ctx, order.OrganizationID, order.OrderID, order.Version, billing.PendingSubscriptionOrderEffectActivate, admittedAt)
	if err != nil || won {
		t.Fatalf("stale competing admission won = %v, err = %v", won, err)
	}

	admitted.PendingEffect = billing.PendingSubscriptionOrderEffectNone
	admitted.PendingEffectAdmittedAt = nil
	admitted.TerminalIntent = billing.SubscriptionOrderTerminalIntentCancel
	admitted.TerminalIntentReason = billing.OrderFailureAuthorizationRevoked
	terminalAt := admittedAt.Add(time.Minute)
	admitted.TerminalIntentAt = &terminalAt
	admitted.Status = billing.OrderCancelled
	admitted.FailureCode = billing.OrderFailureAuthorizationRevoked
	admitted.UpdatedAt = terminalAt
	if err := repository.PersistSubscriptionOrder(ctx, admitted); err != nil {
		t.Fatal(err)
	}
	current, err := repository.ReadOrder(ctx, admitted.OrganizationID, admitted.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	_, won, err = repository.AdmitSubscriptionOrderEffect(ctx, current.OrganizationID, current.OrderID, current.Version, billing.PendingSubscriptionOrderEffectActivate, terminalAt.Add(time.Minute))
	if err != nil || won {
		t.Fatalf("terminal order admission won = %v, err = %v", won, err)
	}
}
