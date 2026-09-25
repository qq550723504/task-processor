//go:build integration

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/commercial/billing"
	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	moneystore "task-processor/internal/integration/persistence/money"
	"task-processor/internal/listingsubscription"
)

type trialPurchasedSubscriptionPort struct{ owner *listingsubscription.Service }

func (port trialPurchasedSubscriptionPort) ResolvePurchasablePlan(ctx context.Context, code string) (billing.SubscriptionPlanSnapshot, error) {
	plan, err := port.owner.ResolvePurchasablePlan(ctx, code)
	return billing.SubscriptionPlanSnapshot{PlanCode: plan.PlanCode, DisplayName: plan.DisplayName, Fingerprint: plan.Fingerprint}, err
}

func (port trialPurchasedSubscriptionPort) ReadPurchasedSubscriptionState(ctx context.Context, organizationID string) (billing.SubscriptionPurchaseState, error) {
	state, err := port.owner.ReadPurchasedSubscriptionState(ctx, organizationID)
	return billing.SubscriptionPurchaseState{PlanCode: state.PlanCode, BlocksPurchase: state.BlocksPurchase}, err
}

func (port trialPurchasedSubscriptionPort) ActivatePurchasedSubscription(ctx context.Context, request billing.SubscriptionActivationRequest) (billing.SubscriptionActivationResult, error) {
	result, err := port.owner.ActivatePurchasedPlan(ctx, listingsubscription.PurchasedPlanActivationInput{OperationID: request.OperationID, OrganizationID: request.OrganizationID, ActorID: request.ActorID, SourceType: listingsubscription.PurchasedPlanSourceCommercialOrder, SourceID: request.CommercialOrderID, PlanCode: request.PlanCode, PlanFingerprint: request.PlanFingerprint, TermMonths: request.TermMonths})
	return trialActivationResult(result), err
}

func (port trialPurchasedSubscriptionPort) ReadPurchasedSubscriptionActivation(ctx context.Context, organizationID, orderID string) (billing.SubscriptionActivationResult, error) {
	result, err := port.owner.ReadPurchasedPlanActivation(ctx, organizationID, orderID)
	if err == listingsubscription.ErrPurchasedPlanActivationNotFound {
		return billing.SubscriptionActivationResult{}, billing.ErrSubscriptionActivationNotFound
	}
	return trialActivationResult(result), err
}

func trialActivationResult(result listingsubscription.PurchasedPlanActivationResult) billing.SubscriptionActivationResult {
	subscriptionID := ""
	if result.SubscriptionID > 0 {
		subscriptionID = strconv.FormatInt(result.SubscriptionID, 10)
	}
	return billing.SubscriptionActivationResult{OperationID: result.OperationID, OrganizationID: result.OrganizationID, CommercialOrderID: result.SourceID, PlanCode: result.PlanCode, PlanFingerprint: result.PlanFingerprint, ActivationRequestFingerprint: result.ActivationRequestFingerprint, Outcome: billing.SubscriptionActivationOutcome(result.Outcome), FailureCode: billing.SubscriptionActivationFailureCode(result.FailureCode), SubscriptionID: subscriptionID, StartsAt: result.StartsAt, ExpiresAt: result.ExpiresAt, EntitlementSetFingerprint: result.EntitlementSetFingerprint, DecidedAt: result.DecidedAt, Existing: result.Existing}
}

type trialPurchaseAuthorizer struct{}

func (trialPurchaseAuthorizer) ReauthorizeCommercialPurchase(_ context.Context, organizationID, actorID string) (billing.CommercialPurchaseAuthorization, error) {
	return billing.CommercialPurchaseAuthorization{OrganizationID: organizationID, ActorID: actorID, Roles: []string{"listingkit_admin"}, Allowed: true, ObservedAt: time.Now().UTC()}, nil
}

func TestPostgresIsolatedCatalogOwnerReadbackAndNoEntitlementSeed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("subscription_trial"), tcpostgres.WithUsername("subscription_trial"), tcpostgres.WithPassword("subscription_trial"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, listingsubscription.AutoMigrateRepository(db))
	require.NoError(t, commercialstore.AutoMigrate(db))
	require.NoError(t, moneystore.AutoMigrate(db))

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)
	portNumber, err := strconv.Atoi(port.Port())
	require.NoError(t, err)
	configData, err := json.Marshal(schemaOwnerManifest{Host: host, Port: portNumber, User: "subscription_trial", Password: "subscription_trial", Database: "subscription_trial", MaxConnections: 5, MaxIdleConnections: 1})
	require.NoError(t, err)
	configPath := filepath.Join(t.TempDir(), "private-db.json")
	require.NoError(t, os.WriteFile(configPath, configData, 0600))
	require.Error(t, runIsolatedTrial(configPath, "wrong_database"))
	require.NoError(t, runIsolatedTrial(configPath, "subscription_trial"))
	owner, err := listingsubscription.NewRuntimeService(listingsubscription.NewGormRepository(db))
	require.NoError(t, err)
	snapshot, err := owner.ResolvePurchasablePlan(ctx, trialPlanCode)
	require.NoError(t, err)
	require.NotEmpty(t, snapshot.Fingerprint)
	commercial, err := commercialstore.New(db)
	require.NoError(t, err)
	offer, err := commercial.ReadOffer(ctx, trialOfferID)
	require.NoError(t, err)
	require.Equal(t, snapshot.PlanCode, offer.PlanCode)
	require.Equal(t, "isolated-trial-v1", offer.PricingVersion)

	for _, table := range []string{"saas_tenant_subscriptions", "saas_tenant_entitlements", "commercial_quotes", "commercial_orders"} {
		var count int64
		require.NoError(t, db.Table(table).Count(&count).Error)
		require.Zero(t, count, table)
	}
	require.Error(t, runIsolatedTrial(configPath, "subscription_trial"))

	wallet, err := moneystore.New(db)
	require.NoError(t, err)
	service, err := billing.NewService(commercial, commercial, commercial, commercial, wallet, nil)
	require.NoError(t, err)
	require.NoError(t, service.EnableSubscriptionPurchases(trialPurchasedSubscriptionPort{owner: owner}, trialPurchaseAuthorizer{}))
	quote, err := service.CreateSubscriptionQuote(ctx, billing.SubscriptionQuoteRequest{OrganizationID: "org-isolated-trial", OfferID: trialOfferID})
	require.NoError(t, err)
	require.EqualValues(t, 0, quote.TotalMinor)
	order, err := service.CreateSubscriptionOrder(ctx, billing.CreateSubscriptionOrderRequest{OrganizationID: "org-isolated-trial", ActorID: "actor-isolated-trial", QuoteID: quote.QuoteID, IdempotencyKey: "isolated-trial-purchase-1"})
	require.NoError(t, err)
	require.Equal(t, billing.OrderFulfilled, order.Status)
	require.Equal(t, billing.SubscriptionActivationActivated, order.ActivationOutcome)
	require.NotEmpty(t, order.ActivationEntitlementSetFingerprint)
	require.NotNil(t, order.ActivationStartsAt)
	require.NotNil(t, order.ActivationExpiresAt)
	_, err = owner.AuthorizeUsage(ctx, "org-isolated-trial", listingsubscription.ModuleOSSStorage, "storage_bytes", 100*1024*1024+1)
	require.ErrorIs(t, err, listingsubscription.ErrSubscriptionQuotaExceed)
	replayed, err := service.CreateSubscriptionOrder(ctx, billing.CreateSubscriptionOrderRequest{OrganizationID: "org-isolated-trial", ActorID: "actor-isolated-trial", QuoteID: quote.QuoteID, IdempotencyKey: "isolated-trial-purchase-1"})
	require.NoError(t, err)
	require.Equal(t, order.OrderID, replayed.OrderID)
	var entitlements []struct {
		ModuleCode string     `gorm:"column:module_code"`
		Limits     string     `gorm:"column:limits"`
		StartsAt   *time.Time `gorm:"column:starts_at"`
		ExpiresAt  *time.Time `gorm:"column:expires_at"`
	}
	require.NoError(t, db.Table("saas_tenant_entitlements").Where("tenant_id = ?", "org-isolated-trial").Order("module_code").Find(&entitlements).Error)
	require.Len(t, entitlements, 4)
	wantLimits := map[string]string{
		listingsubscription.ModuleStoreManagement: `{"store_count":1}`,
		listingsubscription.ModuleRules:           `{}`,
		listingsubscription.ModuleListingKit:      `{"listingkit_generations_succeeded":5,"product_image_jobs_succeeded":5,"shein_drafts_succeeded":5,"ai_tokens":50000}`,
		listingsubscription.ModuleOSSStorage:      `{"storage_bytes_current":104857600,"storage_bytes":104857600}`,
	}
	for _, entitlement := range entitlements {
		expected, exists := wantLimits[entitlement.ModuleCode]
		require.True(t, exists, entitlement.ModuleCode)
		require.JSONEq(t, expected, entitlement.Limits)
		require.NotNil(t, entitlement.StartsAt)
		require.NotNil(t, entitlement.ExpiresAt)
		require.True(t, order.ActivationStartsAt.Equal(*entitlement.StartsAt))
		require.True(t, order.ActivationExpiresAt.Equal(*entitlement.ExpiresAt))
		delete(wantLimits, entitlement.ModuleCode)
	}
	require.Empty(t, wantLimits)
	var subscriptionCount int64
	require.NoError(t, db.Table("saas_tenant_subscriptions").Where("tenant_id = ? AND plan_code = ?", "org-isolated-trial", trialPlanCode).Count(&subscriptionCount).Error)
	require.EqualValues(t, 1, subscriptionCount)
}
