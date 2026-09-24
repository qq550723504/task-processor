package commercialbilling

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"task-processor/internal/authidentity"
	"task-processor/internal/commercial/billing"
	billinghttp "task-processor/internal/commercial/billing/httpapi"
	moneystore "task-processor/internal/integration/persistence/money"
	"task-processor/internal/ledger/money"
)

type subscriptionPurchasePortStub struct {
	plan                   billing.SubscriptionPlanSnapshot
	state                  billing.SubscriptionPurchaseState
	outcome                billing.SubscriptionActivationOutcome
	failure                billing.SubscriptionActivationFailureCode
	decisions              map[string]billing.SubscriptionActivationResult
	calls                  int
	unavailable            bool
	loseActivationResponse bool
}

func (stub *subscriptionPurchasePortStub) ResolvePurchasablePlan(_ context.Context, planCode string) (billing.SubscriptionPlanSnapshot, error) {
	plan := stub.plan
	plan.PlanCode = planCode
	if plan.DisplayName == "" || planCode != stub.plan.PlanCode {
		plan.DisplayName = planCode
	}
	return plan, nil
}

func (stub *subscriptionPurchasePortStub) ReadPurchasedSubscriptionState(context.Context, string) (billing.SubscriptionPurchaseState, error) {
	return stub.state, nil
}

func (stub *subscriptionPurchasePortStub) ActivatePurchasedSubscription(_ context.Context, request billing.SubscriptionActivationRequest) (billing.SubscriptionActivationResult, error) {
	stub.calls++
	if stub.unavailable {
		return billing.SubscriptionActivationResult{}, billing.ErrFeatureUnavailable
	}
	if existing, ok := stub.decisions[request.CommercialOrderID]; ok {
		existing.Existing = true
		return existing, nil
	}
	decidedAt := time.Date(2026, 9, 24, 2, 0, 0, 0, time.UTC)
	result := billing.SubscriptionActivationResult{OperationID: request.OperationID, OrganizationID: request.OrganizationID, CommercialOrderID: request.CommercialOrderID, PlanCode: request.PlanCode, PlanFingerprint: request.PlanFingerprint, ActivationRequestFingerprint: testActivationRequestFingerprint(request), Outcome: stub.outcome, FailureCode: stub.failure, DecidedAt: decidedAt}
	if result.Outcome == billing.SubscriptionActivationActivated {
		startsAt := decidedAt
		expiresAt := startsAt.AddDate(0, request.TermMonths, 0)
		result.SubscriptionID = "42"
		result.StartsAt = &startsAt
		result.ExpiresAt = &expiresAt
		result.EntitlementSetFingerprint = "entitlement-fingerprint"
	}
	stub.decisions[request.CommercialOrderID] = result
	if stub.loseActivationResponse {
		stub.loseActivationResponse = false
		return billing.SubscriptionActivationResult{}, billing.ErrFeatureUnavailable
	}
	return result, nil
}

func (stub *subscriptionPurchasePortStub) ReadPurchasedSubscriptionActivation(_ context.Context, _, orderID string) (billing.SubscriptionActivationResult, error) {
	result, ok := stub.decisions[orderID]
	if !ok {
		return billing.SubscriptionActivationResult{}, billing.ErrSubscriptionActivationNotFound
	}
	result.Existing = true
	return result, nil
}

type subscriptionPurchaseAuthorizerStub struct {
	allowed   bool
	decisions []bool
	calls     int
}

type competingAdmissionRepository struct {
	*Repository
	intercepted     bool
	beforeAdmission func(context.Context, string, string, time.Time) error
}

func (store *competingAdmissionRepository) AdmitSubscriptionOrderEffect(ctx context.Context, organizationID, orderID string, expectedVersion int64, effect billing.PendingSubscriptionOrderEffect, admittedAt time.Time) (billing.Order, bool, error) {
	if !store.intercepted {
		store.intercepted = true
		if err := store.beforeAdmission(ctx, organizationID, orderID, admittedAt); err != nil {
			return billing.Order{}, false, err
		}
	}
	return store.Repository.AdmitSubscriptionOrderEffect(ctx, organizationID, orderID, expectedVersion, effect, admittedAt)
}

type subscriptionAckLossWallet struct {
	*moneystore.Repository
	loseReserveResponse bool
	loseCommitResponse  bool
}

func (wallet *subscriptionAckLossWallet) ReserveCommercialPurchase(ctx context.Context, input money.ReserveWalletFundsInput) (money.WalletReservation, error) {
	reservation, err := wallet.Repository.ReserveCommercialPurchase(ctx, input)
	if err == nil && wallet.loseReserveResponse {
		wallet.loseReserveResponse = false
		return money.WalletReservation{}, money.ErrUnavailable
	}
	return reservation, err
}

func (wallet *subscriptionAckLossWallet) CommitCommercialPurchase(ctx context.Context, input money.CommitWalletReservationInput) (money.WalletReservation, error) {
	reservation, err := wallet.Repository.CommitCommercialPurchase(ctx, input)
	if err == nil && wallet.loseCommitResponse {
		wallet.loseCommitResponse = false
		return money.WalletReservation{}, money.ErrUnavailable
	}
	return reservation, err
}

func (stub *subscriptionPurchaseAuthorizerStub) ReauthorizeCommercialPurchase(_ context.Context, organizationID, actorID string) (billing.CommercialPurchaseAuthorization, error) {
	stub.calls++
	allowed := stub.allowed
	if len(stub.decisions) >= stub.calls {
		allowed = stub.decisions[stub.calls-1]
	}
	return billing.CommercialPurchaseAuthorization{OrganizationID: organizationID, ActorID: actorID, Roles: []string{"listingkit_admin"}, Allowed: allowed, ObservedAt: time.Now().UTC()}, nil
}

func TestSubscriptionServiceLostAdmissionCannotReviveRevokedTerminalOrder(t *testing.T) {
	for _, mode := range []billing.SettlementMode{billing.SettlementZeroPrice, billing.SettlementWallet} {
		t.Run(string(mode), func(t *testing.T) {
			commercial := commercialRepository(t)
			wallet := subscriptionWalletRepository(t, commercial)
			store := &competingAdmissionRepository{Repository: commercial}
			store.beforeAdmission = func(ctx context.Context, organizationID, orderID string, admittedAt time.Time) error {
				order, err := commercial.ReadOrder(ctx, organizationID, orderID)
				if err != nil {
					return err
				}
				order.TerminalIntent = billing.SubscriptionOrderTerminalIntentCancel
				order.TerminalIntentReason = billing.OrderFailureAuthorizationRevoked
				order.TerminalIntentAt = &admittedAt
				order.Status = billing.OrderCancelled
				order.FailureCode = billing.OrderFailureAuthorizationRevoked
				order.UpdatedAt = admittedAt
				return commercial.PersistSubscriptionOrder(ctx, order)
			}
			port := &subscriptionPurchasePortStub{plan: billing.SubscriptionPlanSnapshot{PlanCode: "professional", DisplayName: "Professional", Fingerprint: "plan-fingerprint"}, outcome: billing.SubscriptionActivationActivated, decisions: map[string]billing.SubscriptionActivationResult{}}
			service, err := billing.NewService(commercial, commercial, store, store, wallet, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := service.EnableSubscriptionPurchases(port, &subscriptionPurchaseAuthorizerStub{allowed: true}); err != nil {
				t.Fatal(err)
			}
			offer := billing.Offer{OfferID: "professional-race", ProductKind: billing.ProductSubscriptionPlan, PlanCode: "professional", TermMonths: 1, SettlementMode: mode, Currency: billing.CurrencyCNY, PricingVersion: "pricing-1", Status: billing.OfferActive}
			if mode == billing.SettlementWallet {
				offer.UnitPriceMinor = 400
				creditSubscriptionWallet(t, wallet, "org-lost-admission", 1000)
			}
			if err := commercial.SaveOffer(context.Background(), offer); err != nil {
				t.Fatal(err)
			}
			quote, err := service.CreateSubscriptionQuote(context.Background(), billing.SubscriptionQuoteRequest{OrganizationID: "org-lost-admission", OfferID: offer.OfferID})
			if err != nil {
				t.Fatal(err)
			}
			order, err := service.CreateSubscriptionOrder(context.Background(), billing.CreateSubscriptionOrderRequest{OrganizationID: "org-lost-admission", ActorID: "actor-a", QuoteID: quote.QuoteID, IdempotencyKey: "idem-lost-admission"})
			if !errors.Is(err, billing.ErrAuthorizationRevoked) || order.Status != billing.OrderCancelled || port.calls != 0 {
				t.Fatalf("lost admission result = %+v, err = %v, activation calls = %d", order, err, port.calls)
			}
			stored, err := commercial.ReadOrder(context.Background(), order.OrganizationID, order.OrderID)
			if err != nil || stored.Status != billing.OrderCancelled || stored.TerminalIntent != billing.SubscriptionOrderTerminalIntentCancel || stored.PendingEffect != billing.PendingSubscriptionOrderEffectNone {
				t.Fatalf("durable order after lost admission = %+v, err = %v", stored, err)
			}
			recovered, err := service.ReconcileSubscriptionOrder(context.Background(), order.OrganizationID, order.OrderID)
			if !errors.Is(err, billing.ErrAuthorizationRevoked) || recovered.Status != billing.OrderCancelled {
				t.Fatalf("recovery result = %+v, err = %v", recovered, err)
			}
		})
	}
}

func TestSubscriptionServiceLostAdmissionReturnsFulfilledCompetingOrder(t *testing.T) {
	for _, mode := range []billing.SettlementMode{billing.SettlementZeroPrice, billing.SettlementWallet} {
		t.Run(string(mode), func(t *testing.T) {
			commercial := commercialRepository(t)
			wallet := subscriptionWalletRepository(t, commercial)
			port := &subscriptionPurchasePortStub{plan: billing.SubscriptionPlanSnapshot{PlanCode: "professional", DisplayName: "Professional", Fingerprint: "plan-fingerprint"}, outcome: billing.SubscriptionActivationActivated, decisions: map[string]billing.SubscriptionActivationResult{}}
			competitor := subscriptionBillingService(t, commercial, wallet, port, &subscriptionPurchaseAuthorizerStub{allowed: true})
			store := &competingAdmissionRepository{Repository: commercial}
			store.beforeAdmission = func(ctx context.Context, organizationID, orderID string, _ time.Time) error {
				completed, err := competitor.ReconcileSubscriptionOrder(ctx, organizationID, orderID)
				if err == nil && completed.Status != billing.OrderFulfilled {
					return errors.New("competing order did not fulfill")
				}
				return err
			}
			service, err := billing.NewService(commercial, commercial, store, store, wallet, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := service.EnableSubscriptionPurchases(port, &subscriptionPurchaseAuthorizerStub{allowed: true}); err != nil {
				t.Fatal(err)
			}
			offer := billing.Offer{OfferID: "professional-competing", ProductKind: billing.ProductSubscriptionPlan, PlanCode: "professional", TermMonths: 1, SettlementMode: mode, Currency: billing.CurrencyCNY, PricingVersion: "pricing-1", Status: billing.OfferActive}
			if mode == billing.SettlementWallet {
				offer.UnitPriceMinor = 400
				creditSubscriptionWallet(t, wallet, "org-competing", 1000)
			}
			if err := commercial.SaveOffer(context.Background(), offer); err != nil {
				t.Fatal(err)
			}
			quote, err := service.CreateSubscriptionQuote(context.Background(), billing.SubscriptionQuoteRequest{OrganizationID: "org-competing", OfferID: offer.OfferID})
			if err != nil {
				t.Fatal(err)
			}
			order, err := service.CreateSubscriptionOrder(context.Background(), billing.CreateSubscriptionOrderRequest{OrganizationID: "org-competing", ActorID: "actor-a", QuoteID: quote.QuoteID, IdempotencyKey: "idem-competing"})
			if err != nil || order.Status != billing.OrderFulfilled || port.calls != 1 {
				t.Fatalf("lost admission result = %+v, err = %v, activation calls = %d", order, err, port.calls)
			}
			stored, err := commercial.ReadOrder(context.Background(), order.OrganizationID, order.OrderID)
			if err != nil || stored.Status != billing.OrderFulfilled || stored.Version != order.Version {
				t.Fatalf("durable order after competing fulfillment = %+v, err = %v", stored, err)
			}
		})
	}
}

func TestSubscriptionOrderPostReturnsContractErrorAfterDurableOrderCreation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, scenario := range []struct {
		name       string
		allowed    bool
		outcome    billing.SubscriptionActivationOutcome
		failure    billing.SubscriptionActivationFailureCode
		wantStatus int
		wantCode   string
	}{
		{name: "revoked", allowed: false, outcome: billing.SubscriptionActivationActivated, wantStatus: http.StatusForbidden, wantCode: "FORBIDDEN"},
		{name: "active subscription", allowed: true, outcome: billing.SubscriptionActivationRejected, failure: billing.SubscriptionActivationActiveSubscriptionExists, wantStatus: http.StatusConflict, wantCode: "ACTIVE_SUBSCRIPTION_EXISTS"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			commercial := commercialRepository(t)
			wallet := subscriptionWalletRepository(t, commercial)
			port := &subscriptionPurchasePortStub{plan: billing.SubscriptionPlanSnapshot{PlanCode: "professional", DisplayName: "Professional", Fingerprint: "plan-fingerprint"}, outcome: scenario.outcome, failure: scenario.failure, decisions: map[string]billing.SubscriptionActivationResult{}}
			service := subscriptionBillingService(t, commercial, wallet, port, &subscriptionPurchaseAuthorizerStub{allowed: scenario.allowed})
			offer := billing.Offer{OfferID: "professional-error", ProductKind: billing.ProductSubscriptionPlan, PlanCode: "professional", TermMonths: 1, SettlementMode: billing.SettlementZeroPrice, Currency: billing.CurrencyCNY, PricingVersion: "pricing-1", Status: billing.OfferActive}
			if err := commercial.SaveOffer(context.Background(), offer); err != nil {
				t.Fatal(err)
			}
			quote, err := service.CreateSubscriptionQuote(context.Background(), billing.SubscriptionQuoteRequest{OrganizationID: "org-http-error", OfferID: offer.OfferID})
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(response)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/commercial/subscription-orders", strings.NewReader(`{"quote_id":"`+quote.QuoteID+`"}`))
			request.Header.Set("Idempotency-Key", "idem-http-error")
			ctx.Request = request.WithContext(authidentity.WithAuthenticatedIdentity(request.Context(), authidentity.AuthenticatedIdentity{UserID: "actor-a", TenantID: "org-http-error", EffectiveOrganizationID: "org-http-error"}))
			billinghttp.NewHandler(service).CreateSubscriptionOrder(ctx)
			var body struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != scenario.wantStatus || body.Code != scenario.wantCode {
				t.Fatalf("POST status = %d, body = %s, decode err = %v", response.Code, response.Body.String(), err)
			}
			if order, found, err := commercial.FindSubscriptionOrderByIdempotency(context.Background(), "org-http-error", "idem-http-error"); err != nil || !found || order.OrderID == "" {
				t.Fatalf("durable order = %+v, found = %v, err = %v", order, found, err)
			}
		})
	}
}

func TestSubscriptionServiceRevocationAfterReservePersistsIntentAndCannotRevive(t *testing.T) {
	commercial := commercialRepository(t)
	wallet := subscriptionWalletRepository(t, commercial)
	creditSubscriptionWallet(t, wallet, "org-revoked", 1000)
	port := &subscriptionPurchasePortStub{plan: billing.SubscriptionPlanSnapshot{PlanCode: "professional", DisplayName: "Professional", Fingerprint: "plan-fingerprint"}, outcome: billing.SubscriptionActivationActivated, decisions: map[string]billing.SubscriptionActivationResult{}}
	authorizer := &subscriptionPurchaseAuthorizerStub{decisions: []bool{true, false}, allowed: true}
	service := subscriptionBillingService(t, commercial, wallet, port, authorizer)
	offer := billing.Offer{OfferID: "professional-revoked", ProductKind: billing.ProductSubscriptionPlan, PlanCode: "professional", TermMonths: 1, SettlementMode: billing.SettlementWallet, Currency: billing.CurrencyCNY, UnitPriceMinor: 400, PricingVersion: "pricing-1", Status: billing.OfferActive}
	if err := commercial.SaveOffer(context.Background(), offer); err != nil {
		t.Fatal(err)
	}
	quote, err := service.CreateSubscriptionQuote(context.Background(), billing.SubscriptionQuoteRequest{OrganizationID: "org-revoked", OfferID: offer.OfferID})
	if err != nil {
		t.Fatal(err)
	}
	request := billing.CreateSubscriptionOrderRequest{OrganizationID: "org-revoked", ActorID: "actor-a", QuoteID: quote.QuoteID, IdempotencyKey: "idem-revoked"}
	order, err := service.CreateSubscriptionOrder(context.Background(), request)
	if !errors.Is(err, billing.ErrAuthorizationRevoked) || order.Status != billing.OrderCancelled || order.FailureCode != billing.OrderFailureAuthorizationRevoked || order.TerminalIntent != billing.SubscriptionOrderTerminalIntentCancel || order.WalletReservationID != "" || port.calls != 0 {
		t.Fatalf("order = %+v, err = %v, activation calls = %d", order, err, port.calls)
	}
	snapshot, err := wallet.ReadOrganizationWallet(context.Background(), "org-revoked", "CNY")
	if err != nil || snapshot.AvailableMinor != 1000 || snapshot.ReservedMinor != 0 || snapshot.LifetimeSpendMinor != 0 {
		t.Fatalf("wallet = %+v, err = %v", snapshot, err)
	}
	replayed, err := service.CreateSubscriptionOrder(context.Background(), request)
	if !errors.Is(err, billing.ErrAuthorizationRevoked) || replayed.OrderID != order.OrderID || authorizer.calls != 2 || port.calls != 0 {
		t.Fatalf("replayed = %+v, err = %v, auth calls = %d, activation calls = %d", replayed, err, authorizer.calls, port.calls)
	}
}

func TestSubscriptionOffersDeriveAvailabilityFromCanonicalSubscriptionState(t *testing.T) {
	commercial := commercialRepository(t)
	wallet := subscriptionWalletRepository(t, commercial)
	port := &subscriptionPurchasePortStub{
		plan:  billing.SubscriptionPlanSnapshot{PlanCode: "professional", DisplayName: "Professional", Fingerprint: "plan-fingerprint"},
		state: billing.SubscriptionPurchaseState{PlanCode: "professional", BlocksPurchase: true},
	}
	service := subscriptionBillingService(t, commercial, wallet, port, &subscriptionPurchaseAuthorizerStub{allowed: true})
	for _, offer := range []billing.Offer{
		{OfferID: "professional-current", ProductKind: billing.ProductSubscriptionPlan, PlanCode: "professional", TermMonths: 1, SettlementMode: billing.SettlementZeroPrice, Currency: billing.CurrencyCNY, PricingVersion: "pricing-1", Status: billing.OfferActive},
		{OfferID: "other-conflict", ProductKind: billing.ProductSubscriptionPlan, PlanCode: "other", TermMonths: 1, SettlementMode: billing.SettlementExternalPayment, Currency: billing.CurrencyCNY, UnitPriceMinor: 400, PricingVersion: "pricing-1", Status: billing.OfferActive},
	} {
		if err := commercial.SaveOffer(context.Background(), offer); err != nil {
			t.Fatal(err)
		}
	}
	views, err := service.ListSubscriptionOffers(context.Background(), "org-current")
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 2 || views[0].Availability != billing.SubscriptionOfferActiveConflict || views[1].Availability != billing.SubscriptionOfferCurrentPlan {
		t.Fatalf("offer views = %+v", views)
	}
}

func TestSubscriptionServiceZeroPriceCreatesCanonicalOrderAndActivationProof(t *testing.T) {
	commercial := commercialRepository(t)
	wallet := subscriptionWalletRepository(t, commercial)
	port := &subscriptionPurchasePortStub{plan: billing.SubscriptionPlanSnapshot{PlanCode: "professional", DisplayName: "Professional", Fingerprint: "plan-fingerprint"}, outcome: billing.SubscriptionActivationActivated, decisions: map[string]billing.SubscriptionActivationResult{}}
	authorizer := &subscriptionPurchaseAuthorizerStub{allowed: true}
	service := subscriptionBillingService(t, commercial, wallet, port, authorizer)
	offer := billing.Offer{OfferID: "professional-zero", ProductKind: billing.ProductSubscriptionPlan, PlanCode: "professional", TermMonths: 1, SettlementMode: billing.SettlementZeroPrice, Currency: billing.CurrencyCNY, PricingVersion: "pricing-1", Status: billing.OfferActive}
	if err := commercial.SaveOffer(context.Background(), offer); err != nil {
		t.Fatal(err)
	}
	quote, err := service.CreateSubscriptionQuote(context.Background(), billing.SubscriptionQuoteRequest{OrganizationID: "org-a", OfferID: offer.OfferID})
	if err != nil {
		t.Fatal(err)
	}
	request := billing.CreateSubscriptionOrderRequest{OrganizationID: "org-a", ActorID: "actor-a", QuoteID: quote.QuoteID, IdempotencyKey: "idem-zero"}
	order, err := service.CreateSubscriptionOrder(context.Background(), request)
	if err != nil || order.Status != billing.OrderFulfilled || order.ActivationOutcome != billing.SubscriptionActivationActivated || order.ActivationRequestFingerprint == "" || order.WalletReservationID != "" {
		t.Fatalf("order = %+v, err = %v", order, err)
	}
	replayed, err := service.CreateSubscriptionOrder(context.Background(), request)
	if err != nil || replayed.OrderID != order.OrderID || port.calls != 1 {
		t.Fatalf("replayed = %+v, err = %v, activation calls = %d", replayed, err, port.calls)
	}
}

func TestSubscriptionServiceRecoversLostActivationResponseByOwnerReadback(t *testing.T) {
	commercial := commercialRepository(t)
	wallet := subscriptionWalletRepository(t, commercial)
	port := &subscriptionPurchasePortStub{plan: billing.SubscriptionPlanSnapshot{PlanCode: "professional", DisplayName: "Professional", Fingerprint: "plan-fingerprint"}, outcome: billing.SubscriptionActivationActivated, decisions: map[string]billing.SubscriptionActivationResult{}, loseActivationResponse: true}
	service := subscriptionBillingService(t, commercial, wallet, port, &subscriptionPurchaseAuthorizerStub{allowed: true})
	offer := billing.Offer{OfferID: "professional-zero-readback", ProductKind: billing.ProductSubscriptionPlan, PlanCode: "professional", TermMonths: 1, SettlementMode: billing.SettlementZeroPrice, Currency: billing.CurrencyCNY, PricingVersion: "pricing-1", Status: billing.OfferActive}
	if err := commercial.SaveOffer(context.Background(), offer); err != nil {
		t.Fatal(err)
	}
	quote, err := service.CreateSubscriptionQuote(context.Background(), billing.SubscriptionQuoteRequest{OrganizationID: "org-readback", OfferID: offer.OfferID})
	if err != nil {
		t.Fatal(err)
	}
	order, err := service.CreateSubscriptionOrder(context.Background(), billing.CreateSubscriptionOrderRequest{OrganizationID: "org-readback", ActorID: "actor-a", QuoteID: quote.QuoteID, IdempotencyKey: "idem-readback"})
	if err != nil || order.Status != billing.OrderFulfilled || order.ActivationOutcome != billing.SubscriptionActivationActivated || port.calls != 1 {
		t.Fatalf("order = %+v, err = %v, activation calls = %d", order, err, port.calls)
	}
}

func TestSubscriptionServiceRestartReplaysOnlyAlreadyAdmittedActivation(t *testing.T) {
	commercial := commercialRepository(t)
	wallet := subscriptionWalletRepository(t, commercial)
	port := &subscriptionPurchasePortStub{plan: billing.SubscriptionPlanSnapshot{PlanCode: "professional", DisplayName: "Professional", Fingerprint: "plan-fingerprint"}, outcome: billing.SubscriptionActivationActivated, decisions: map[string]billing.SubscriptionActivationResult{}, unavailable: true}
	initialAuthorizer := &subscriptionPurchaseAuthorizerStub{allowed: true}
	service := subscriptionBillingService(t, commercial, wallet, port, initialAuthorizer)
	offer := billing.Offer{OfferID: "professional-zero-restart", ProductKind: billing.ProductSubscriptionPlan, PlanCode: "professional", TermMonths: 1, SettlementMode: billing.SettlementZeroPrice, Currency: billing.CurrencyCNY, PricingVersion: "pricing-1", Status: billing.OfferActive}
	if err := commercial.SaveOffer(context.Background(), offer); err != nil {
		t.Fatal(err)
	}
	quote, err := service.CreateSubscriptionQuote(context.Background(), billing.SubscriptionQuoteRequest{OrganizationID: "org-restart", OfferID: offer.OfferID})
	if err != nil {
		t.Fatal(err)
	}
	request := billing.CreateSubscriptionOrderRequest{OrganizationID: "org-restart", ActorID: "actor-a", QuoteID: quote.QuoteID, IdempotencyKey: "idem-restart"}
	unknown, err := service.CreateSubscriptionOrder(context.Background(), request)
	if !errors.Is(err, billing.ErrReconciliationRequired) || unknown.Status != billing.OrderReconciliationRequired || unknown.PendingEffect != billing.PendingSubscriptionOrderEffectActivate || initialAuthorizer.calls != 1 {
		t.Fatalf("unknown order = %+v, err = %v, auth calls = %d", unknown, err, initialAuthorizer.calls)
	}

	port.unavailable = false
	recoveryAuthorizer := &subscriptionPurchaseAuthorizerStub{allowed: false}
	restarted := subscriptionBillingService(t, commercial, wallet, port, recoveryAuthorizer)
	recovered, err := restarted.ReconcileSubscriptionOrder(context.Background(), unknown.OrganizationID, unknown.OrderID)
	if err != nil || recovered.Status != billing.OrderFulfilled || recovered.OrderID != unknown.OrderID || recoveryAuthorizer.calls != 0 {
		t.Fatalf("recovered order = %+v, err = %v, recovery auth calls = %d", recovered, err, recoveryAuthorizer.calls)
	}
}

func TestSubscriptionServiceRecoversLostWalletReserveAndCommitResponses(t *testing.T) {
	commercial := commercialRepository(t)
	canonicalWallet := subscriptionWalletRepository(t, commercial)
	creditSubscriptionWallet(t, canonicalWallet, "org-wallet-readback", 1000)
	wallet := &subscriptionAckLossWallet{Repository: canonicalWallet, loseReserveResponse: true, loseCommitResponse: true}
	port := &subscriptionPurchasePortStub{plan: billing.SubscriptionPlanSnapshot{PlanCode: "professional", DisplayName: "Professional", Fingerprint: "plan-fingerprint"}, outcome: billing.SubscriptionActivationActivated, decisions: map[string]billing.SubscriptionActivationResult{}}
	authorizer := &subscriptionPurchaseAuthorizerStub{allowed: true}
	service := subscriptionBillingService(t, commercial, wallet, port, authorizer)
	offer := billing.Offer{OfferID: "professional-wallet-readback", ProductKind: billing.ProductSubscriptionPlan, PlanCode: "professional", TermMonths: 1, SettlementMode: billing.SettlementWallet, Currency: billing.CurrencyCNY, UnitPriceMinor: 400, PricingVersion: "pricing-1", Status: billing.OfferActive}
	if err := commercial.SaveOffer(context.Background(), offer); err != nil {
		t.Fatal(err)
	}
	quote, err := service.CreateSubscriptionQuote(context.Background(), billing.SubscriptionQuoteRequest{OrganizationID: "org-wallet-readback", OfferID: offer.OfferID})
	if err != nil {
		t.Fatal(err)
	}
	order, err := service.CreateSubscriptionOrder(context.Background(), billing.CreateSubscriptionOrderRequest{OrganizationID: "org-wallet-readback", ActorID: "actor-a", QuoteID: quote.QuoteID, IdempotencyKey: "idem-wallet-readback"})
	if err != nil || order.Status != billing.OrderFulfilled || order.WalletReservationState != money.WalletReservationCommitted || authorizer.calls != 2 {
		t.Fatalf("order = %+v, err = %v, auth calls = %d", order, err, authorizer.calls)
	}
	snapshot, err := canonicalWallet.ReadOrganizationWallet(context.Background(), "org-wallet-readback", money.WalletCurrencyCNY)
	if err != nil || snapshot.AvailableMinor != 600 || snapshot.ReservedMinor != 0 || snapshot.LifetimeSpendMinor != 400 {
		t.Fatalf("wallet = %+v, err = %v", snapshot, err)
	}
}

func TestSubscriptionServiceInsufficientDecisionCannotReviveAfterTopUp(t *testing.T) {
	commercial := commercialRepository(t)
	wallet := subscriptionWalletRepository(t, commercial)
	port := &subscriptionPurchasePortStub{plan: billing.SubscriptionPlanSnapshot{PlanCode: "professional", DisplayName: "Professional", Fingerprint: "plan-fingerprint"}, outcome: billing.SubscriptionActivationActivated, decisions: map[string]billing.SubscriptionActivationResult{}}
	authorizer := &subscriptionPurchaseAuthorizerStub{allowed: true}
	service := subscriptionBillingService(t, commercial, wallet, port, authorizer)
	offer := billing.Offer{OfferID: "professional-wallet-insufficient", ProductKind: billing.ProductSubscriptionPlan, PlanCode: "professional", TermMonths: 1, SettlementMode: billing.SettlementWallet, Currency: billing.CurrencyCNY, UnitPriceMinor: 400, PricingVersion: "pricing-1", Status: billing.OfferActive}
	if err := commercial.SaveOffer(context.Background(), offer); err != nil {
		t.Fatal(err)
	}
	quote, err := service.CreateSubscriptionQuote(context.Background(), billing.SubscriptionQuoteRequest{OrganizationID: "org-insufficient-order", OfferID: offer.OfferID})
	if err != nil {
		t.Fatal(err)
	}
	request := billing.CreateSubscriptionOrderRequest{OrganizationID: "org-insufficient-order", ActorID: "actor-a", QuoteID: quote.QuoteID, IdempotencyKey: "idem-insufficient-order"}
	cancelled, err := service.CreateSubscriptionOrder(context.Background(), request)
	if !errors.Is(err, billing.ErrInsufficientFunds) || cancelled.Status != billing.OrderCancelled || cancelled.FailureCode != billing.OrderFailureInsufficientFunds || port.calls != 0 {
		t.Fatalf("cancelled order = %+v, err = %v", cancelled, err)
	}
	creditSubscriptionWallet(t, wallet, "org-insufficient-order", 1000)
	replayed, err := service.CreateSubscriptionOrder(context.Background(), request)
	if !errors.Is(err, billing.ErrInsufficientFunds) || replayed.OrderID != cancelled.OrderID || replayed.Status != billing.OrderCancelled || port.calls != 0 {
		t.Fatalf("replayed order = %+v, err = %v, activation calls = %d", replayed, err, port.calls)
	}
	snapshot, err := wallet.ReadOrganizationWallet(context.Background(), "org-insufficient-order", money.WalletCurrencyCNY)
	if err != nil || snapshot.AvailableMinor != 1000 || snapshot.ReservedMinor != 0 {
		t.Fatalf("wallet = %+v, err = %v", snapshot, err)
	}
}

func TestSubscriptionServiceWalletReleasesOnDurableActivationRejection(t *testing.T) {
	commercial := commercialRepository(t)
	wallet := subscriptionWalletRepository(t, commercial)
	creditSubscriptionWallet(t, wallet, "org-wallet", 1000)
	port := &subscriptionPurchasePortStub{plan: billing.SubscriptionPlanSnapshot{PlanCode: "professional", DisplayName: "Professional", Fingerprint: "plan-fingerprint"}, outcome: billing.SubscriptionActivationRejected, failure: billing.SubscriptionActivationActiveSubscriptionExists, decisions: map[string]billing.SubscriptionActivationResult{}}
	authorizer := &subscriptionPurchaseAuthorizerStub{allowed: true}
	service := subscriptionBillingService(t, commercial, wallet, port, authorizer)
	offer := billing.Offer{OfferID: "professional-wallet", ProductKind: billing.ProductSubscriptionPlan, PlanCode: "professional", TermMonths: 1, SettlementMode: billing.SettlementWallet, Currency: billing.CurrencyCNY, UnitPriceMinor: 400, PricingVersion: "pricing-1", Status: billing.OfferActive}
	if err := commercial.SaveOffer(context.Background(), offer); err != nil {
		t.Fatal(err)
	}
	quote, err := service.CreateSubscriptionQuote(context.Background(), billing.SubscriptionQuoteRequest{OrganizationID: "org-wallet", OfferID: offer.OfferID})
	if err != nil {
		t.Fatal(err)
	}
	order, err := service.CreateSubscriptionOrder(context.Background(), billing.CreateSubscriptionOrderRequest{OrganizationID: "org-wallet", ActorID: "actor-a", QuoteID: quote.QuoteID, IdempotencyKey: "idem-wallet-reject"})
	if !errors.Is(err, billing.ErrActiveSubscriptionExists) || order.Status != billing.OrderCancelled || order.FailureCode != billing.OrderFailureActiveSubscriptionExists || order.ActivationOutcome != billing.SubscriptionActivationRejected || order.WalletReservationID != "" {
		t.Fatalf("order = %+v, err = %v", order, err)
	}
	snapshot, err := wallet.ReadOrganizationWallet(context.Background(), "org-wallet", "CNY")
	if err != nil || snapshot.AvailableMinor != 1000 || snapshot.ReservedMinor != 0 || snapshot.LifetimeSpendMinor != 0 {
		t.Fatalf("wallet = %+v, err = %v", snapshot, err)
	}
	if authorizer.calls != 2 {
		t.Fatalf("authorization calls = %d, want separate RESERVE and ACTIVATE checks", authorizer.calls)
	}
}

func subscriptionBillingService(t *testing.T, commercial *Repository, wallet billing.WalletPort, port billing.SubscriptionPurchasePort, authorizer billing.SubscriptionPurchaseRecoveryAuthorizer) *billing.Service {
	t.Helper()
	service, err := billing.NewService(commercial, commercial, commercial, commercial, wallet, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.EnableSubscriptionPurchases(port, authorizer); err != nil {
		t.Fatal(err)
	}
	return service
}

func subscriptionWalletRepository(t *testing.T, commercial *Repository) *moneystore.Repository {
	t.Helper()
	if err := moneystore.AutoMigrate(commercial.db); err != nil {
		t.Fatal(err)
	}
	repository, err := moneystore.New(commercial.db)
	if err != nil {
		t.Fatal(err)
	}
	return repository
}

func creditSubscriptionWallet(t *testing.T, wallet *moneystore.Repository, organizationID string, amount int64) {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	paymentID := "payment-" + organizationID
	if err := wallet.RecordPaymentSettlement(ctx, money.PaymentSettlement{PaymentID: paymentID, PayerUserID: "payer", Currency: "CNY", GrossAmountMinor: amount, CommissionableAmountMinor: amount, Status: money.PaymentSettled, SettledAt: now, ProviderReference: "provider-" + organizationID, Version: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := wallet.CreditSettledTopUp(ctx, "", money.OrganizationTopUpSettlement{PaymentID: paymentID, CommercialOrderID: "topup-" + organizationID, OrganizationID: organizationID, Currency: "CNY", AmountMinor: amount, SettledAt: now, ProviderReference: "provider-" + organizationID, Version: 1}); err != nil {
		t.Fatal(err)
	}
}

func testActivationRequestFingerprint(request billing.SubscriptionActivationRequest) string {
	encoded, _ := json.Marshal(struct {
		Version         string `json:"version"`
		OperationID     string `json:"operation_id"`
		OrganizationID  string `json:"organization_id"`
		ActorID         string `json:"actor_id"`
		SourceType      string `json:"source_type"`
		SourceID        string `json:"source_id"`
		PlanCode        string `json:"plan_code"`
		PlanFingerprint string `json:"plan_fingerprint"`
		TermMonths      int    `json:"term_months"`
	}{"subscription-activation-v1", request.OperationID, request.OrganizationID, request.ActorID, "commercial_subscription_order", request.CommercialOrderID, request.PlanCode, request.PlanFingerprint, request.TermMonths})
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
