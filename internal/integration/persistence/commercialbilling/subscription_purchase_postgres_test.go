//go:build integration

package commercialbilling

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/commercial/billing"
	moneystore "task-processor/internal/integration/persistence/money"
	"task-processor/internal/ledger/money"
	"task-processor/internal/listingsubscription"
)

type postgresPurchasedSubscriptionPort struct{ owner *listingsubscription.Service }

func (port postgresPurchasedSubscriptionPort) ResolvePurchasablePlan(ctx context.Context, code string) (billing.SubscriptionPlanSnapshot, error) {
	plan, err := port.owner.ResolvePurchasablePlan(ctx, code)
	return billing.SubscriptionPlanSnapshot{PlanCode: plan.PlanCode, DisplayName: plan.DisplayName, Fingerprint: plan.Fingerprint}, err
}

func (port postgresPurchasedSubscriptionPort) ReadPurchasedSubscriptionState(ctx context.Context, organizationID string) (billing.SubscriptionPurchaseState, error) {
	state, err := port.owner.ReadPurchasedSubscriptionState(ctx, organizationID)
	return billing.SubscriptionPurchaseState{PlanCode: state.PlanCode, BlocksPurchase: state.BlocksPurchase}, err
}

func (port postgresPurchasedSubscriptionPort) ActivatePurchasedSubscription(ctx context.Context, request billing.SubscriptionActivationRequest) (billing.SubscriptionActivationResult, error) {
	result, err := port.owner.ActivatePurchasedPlan(ctx, listingsubscription.PurchasedPlanActivationInput{OperationID: request.OperationID, OrganizationID: request.OrganizationID, ActorID: request.ActorID, SourceType: listingsubscription.PurchasedPlanSourceCommercialOrder, SourceID: request.CommercialOrderID, PlanCode: request.PlanCode, PlanFingerprint: request.PlanFingerprint, TermMonths: request.TermMonths})
	return postgresActivationResult(result), err
}

func (port postgresPurchasedSubscriptionPort) ReadPurchasedSubscriptionActivation(ctx context.Context, organizationID, orderID string) (billing.SubscriptionActivationResult, error) {
	result, err := port.owner.ReadPurchasedPlanActivation(ctx, organizationID, orderID)
	if errors.Is(err, listingsubscription.ErrPurchasedPlanActivationNotFound) {
		return billing.SubscriptionActivationResult{}, billing.ErrSubscriptionActivationNotFound
	}
	return postgresActivationResult(result), err
}

func postgresActivationResult(result listingsubscription.PurchasedPlanActivationResult) billing.SubscriptionActivationResult {
	subscriptionID := ""
	if result.SubscriptionID > 0 {
		subscriptionID = strconv.FormatInt(result.SubscriptionID, 10)
	}
	return billing.SubscriptionActivationResult{OperationID: result.OperationID, OrganizationID: result.OrganizationID, CommercialOrderID: result.SourceID, PlanCode: result.PlanCode, PlanFingerprint: result.PlanFingerprint, ActivationRequestFingerprint: result.ActivationRequestFingerprint, Outcome: billing.SubscriptionActivationOutcome(result.Outcome), FailureCode: billing.SubscriptionActivationFailureCode(result.FailureCode), SubscriptionID: subscriptionID, StartsAt: result.StartsAt, ExpiresAt: result.ExpiresAt, EntitlementSetFingerprint: result.EntitlementSetFingerprint, DecidedAt: result.DecidedAt, Existing: result.Existing}
}

type postgresPurchaseAuthorizer struct{}

func (postgresPurchaseAuthorizer) ReauthorizeCommercialPurchase(_ context.Context, organizationID, actorID string) (billing.CommercialPurchaseAuthorization, error) {
	return billing.CommercialPurchaseAuthorization{OrganizationID: organizationID, ActorID: actorID, Roles: []string{"listingkit_admin"}, Allowed: true, ObservedAt: time.Now().UTC()}, nil
}

func TestPostgresConcurrentDistinctSubscriptionOrdersActivateAndChargeOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("subscription_purchase"), tcpostgres.WithUsername("subscription_purchase"), tcpostgres.WithPassword("subscription_purchase"), tcpostgres.BasicWaitStrategies())
	if err != nil {
		t.Fatalf("start PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	for _, migrate := range []func(*gorm.DB) error{AutoMigrate, moneystore.AutoMigrate, listingsubscription.AutoMigrateRepository} {
		if err := migrate(db); err != nil {
			t.Fatal(err)
		}
	}
	subscriptionRepository := listingsubscription.NewGormRepository(db)
	fixtureOwner, err := listingsubscription.NewService(subscriptionRepository)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fixtureOwner.UpsertPlan(ctx, listingsubscription.PlanInput{Code: "postgres-purchased", Name: "PostgreSQL purchased plan", Active: true, Modules: []listingsubscription.PlanModuleInput{{ModuleCode: listingsubscription.ModuleListingKit, Limits: map[string]int{"product_image_jobs": 5}}}}, "test-fixture")
	if err != nil {
		t.Fatal(err)
	}
	runtimeOwner, err := listingsubscription.NewRuntimeService(subscriptionRepository)
	if err != nil {
		t.Fatal(err)
	}
	commercial, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	wallet, err := moneystore.New(db)
	if err != nil {
		t.Fatal(err)
	}
	creditSubscriptionWallet(t, wallet, "org-postgres-purchase", 1000)
	service, err := billing.NewService(commercial, commercial, commercial, commercial, wallet, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.EnableSubscriptionPurchases(postgresPurchasedSubscriptionPort{owner: runtimeOwner}, postgresPurchaseAuthorizer{}); err != nil {
		t.Fatal(err)
	}
	offer := billing.Offer{OfferID: "postgres-wallet-offer", ProductKind: billing.ProductSubscriptionPlan, PlanCode: "postgres-purchased", TermMonths: 1, SettlementMode: billing.SettlementWallet, Currency: billing.CurrencyCNY, UnitPriceMinor: 400, PricingVersion: "postgres-test-v1", Status: billing.OfferActive}
	if err := commercial.SaveOffer(ctx, offer); err != nil {
		t.Fatal(err)
	}
	quotes := make([]billing.Quote, 2)
	for index := range quotes {
		quotes[index], err = service.CreateSubscriptionQuote(ctx, billing.SubscriptionQuoteRequest{OrganizationID: "org-postgres-purchase", OfferID: offer.OfferID})
		if err != nil {
			t.Fatal(err)
		}
	}
	type outcome struct {
		order billing.Order
		err   error
	}
	results := make(chan outcome, 2)
	var wg sync.WaitGroup
	for index := range quotes {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			order, callErr := service.CreateSubscriptionOrder(ctx, billing.CreateSubscriptionOrderRequest{OrganizationID: "org-postgres-purchase", ActorID: "actor-postgres", QuoteID: quotes[index].QuoteID, IdempotencyKey: "postgres-order-" + strconv.Itoa(index)})
			results <- outcome{order: order, err: callErr}
		}(index)
	}
	wg.Wait()
	close(results)
	fulfilled, cancelled := 0, 0
	for result := range results {
		switch result.order.Status {
		case billing.OrderFulfilled:
			fulfilled++
			if result.err != nil || result.order.ActivationOutcome != billing.SubscriptionActivationActivated {
				t.Fatalf("fulfilled result = %+v, err = %v", result.order, result.err)
			}
		case billing.OrderCancelled:
			cancelled++
			if !errors.Is(result.err, billing.ErrActiveSubscriptionExists) || result.order.ActivationOutcome != billing.SubscriptionActivationRejected {
				t.Fatalf("cancelled result = %+v, err = %v", result.order, result.err)
			}
		default:
			t.Fatalf("unexpected result = %+v, err = %v", result.order, result.err)
		}
	}
	if fulfilled != 1 || cancelled != 1 {
		t.Fatalf("fulfilled=%d cancelled=%d", fulfilled, cancelled)
	}
	snapshot, err := wallet.ReadOrganizationWallet(ctx, "org-postgres-purchase", money.WalletCurrencyCNY)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.AvailableMinor != 600 || snapshot.ReservedMinor != 0 || snapshot.LifetimeSpendMinor != 400 {
		t.Fatalf("wallet = %+v", snapshot)
	}
	overview, err := subscriptionRepository.ReadCommercialOverview(ctx, "org-postgres-purchase", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if overview.Subscription == nil || overview.Subscription.PlanCode != "postgres-purchased" || overview.Subscription.EffectiveStatus != "active" || len(overview.Entitlements) != 1 || overview.Entitlements[0].ModuleCode != listingsubscription.ModuleListingKit || overview.Entitlements[0].EffectiveStatus != "active" {
		t.Fatalf("commercial overview = %+v", overview)
	}
}
