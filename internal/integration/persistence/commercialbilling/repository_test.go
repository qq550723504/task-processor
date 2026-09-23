package commercialbilling

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"task-processor/internal/commercial/billing"
	orgresourceadapter "task-processor/internal/integration/orgresource"
	moneystore "task-processor/internal/integration/persistence/money"
	"task-processor/internal/ledger/money"
	"task-processor/internal/ledger/orgresource"
)

type loseFirstCommitAcknowledgement struct {
	money.OrganizationWalletReader
	money.OrganizationWalletCommander
	lose bool
}

type terminalPurchasedResourceGrant struct{}

func (terminalPurchasedResourceGrant) GrantPurchasedResource(context.Context, orgresource.PurchasedResourceGrantInput) (orgresource.PurchasedResourceGrantResult, error) {
	return orgresource.PurchasedResourceGrantResult{}, orgresource.ErrInvalidInput
}

type conflictingPurchasedResourceGrant struct {
	delegate billing.PurchasedResourceGrantPort
	seeded   bool
}

func (grant *conflictingPurchasedResourceGrant) GrantPurchasedResource(ctx context.Context, input orgresource.PurchasedResourceGrantInput) (orgresource.PurchasedResourceGrantResult, error) {
	if !grant.seeded {
		grant.seeded = true
		conflicting := input
		conflicting.Quantity--
		if _, err := grant.delegate.GrantPurchasedResource(ctx, conflicting); err != nil {
			return orgresource.PurchasedResourceGrantResult{}, err
		}
	}
	return grant.delegate.GrantPurchasedResource(ctx, input)
}

type failCancelledOrderUpdate struct{ *Repository }

func (store failCancelledOrderUpdate) UpdateOrder(ctx context.Context, order billing.Order) error {
	if order.Status == billing.OrderCancelled {
		return billing.ErrFeatureUnavailable
	}
	return store.Repository.UpdateOrder(ctx, order)
}

type failFirstCancelledOrderUpdate struct {
	*Repository
	failed bool
}

func (store *failFirstCancelledOrderUpdate) UpdateOrder(ctx context.Context, order billing.Order) error {
	if order.Status == billing.OrderCancelled && !store.failed {
		store.failed = true
		return billing.ErrFeatureUnavailable
	}
	return store.Repository.UpdateOrder(ctx, order)
}

func (wallet *loseFirstCommitAcknowledgement) CommitCommercialPurchase(ctx context.Context, input money.CommitWalletReservationInput) (money.WalletReservation, error) {
	result, err := wallet.OrganizationWalletCommander.CommitCommercialPurchase(ctx, input)
	if err == nil && wallet.lose {
		wallet.lose = false
		return money.WalletReservation{}, errors.New("commit acknowledgement lost")
	}
	return result, err
}

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
	if err != nil || order.Status != billing.OrderPending || order.AmountMinor != 70 || order.Description != "AI 点数 × 10" {
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
	searched, err := repository.ListOrders(context.Background(), "org-a", billing.OrderFilter{Query: "AI 点数", Limit: 10})
	if err != nil || len(searched.Items) != 1 || searched.Items[0].Description != "AI 点数 × 10" {
		t.Fatalf("description search result=%#v err=%v", searched, err)
	}
}

func TestOrderIdempotencyHasDatabaseUniqueFence(t *testing.T) {
	repository := commercialRepository(t)
	if !repository.db.Migrator().HasIndex(&orderRow{}, "uq_commercial_orders_org_idempotency") {
		t.Fatal("commercial order idempotency must be protected by a composite database unique index")
	}
}

func TestReadMissingOrderReturnsNotFoundAndOrderPageIsBounded(t *testing.T) {
	repository := commercialRepository(t)
	if _, err := repository.ReadOrder(context.Background(), "org-a", "missing-order"); !errors.Is(err, billing.ErrNotFound) {
		t.Fatalf("missing order error = %v; want not found", err)
	}
	if _, err := repository.ListOrders(context.Background(), "org-a", billing.OrderFilter{Limit: billing.MaxOrderPageSize + 1}); !errors.Is(err, billing.ErrInvalid) {
		t.Fatalf("oversized order page error = %v; want invalid", err)
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

func TestOrderFiltersCursorAndSummaryUseCompleteCanonicalOrders(t *testing.T) {
	repository := commercialRepository(t)
	created := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	orders := make([]orderRow, 0, 105)
	items := make([]orderItemRow, 0, 105)
	for i := 0; i < 105; i++ {
		id := fmt.Sprintf("order-%03d", i)
		product := billing.ProductAIPoint
		amount := int64(2)
		if i == 104 {
			product = billing.ProductDataRow
			amount = 9
		}
		orders = append(orders, orderRow{OrderID: id, OrganizationID: "org-orders", Kind: string(billing.OrderResourcePurchase), Description: billing.DescribeOrder(billing.OrderResourcePurchase, product, 1), Currency: billing.CurrencyCNY, AmountMinor: amount, Status: string(billing.OrderFulfilled), IdempotencyKey: "idem-" + id, RequestFingerprint: "fingerprint-" + id, Version: 1, CreatedAt: created, UpdatedAt: created})
		items = append(items, orderItemRow{OrderItemID: "item-" + id, OrderID: id, ProductKind: string(product), ResourceType: "ai_point", ResourceQuantity: 1, AmountMinor: amount})
	}
	if err := repository.db.Create(&orders).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.db.Create(&items).Error; err != nil {
		t.Fatal(err)
	}
	product := billing.ProductDataRow
	filtered, err := repository.ListOrders(context.Background(), "org-orders", billing.OrderFilter{ProductKind: &product, Limit: 10})
	if err != nil || len(filtered.Items) != 1 || filtered.Items[0].Items[0].ProductKind != product {
		t.Fatalf("filtered=%#v err=%v", filtered, err)
	}
	searched, err := repository.ListOrders(context.Background(), "org-orders", billing.OrderFilter{Query: "order-104", Limit: 10})
	if err != nil || len(searched.Items) != 1 || searched.Items[0].OrderID != "order-104" {
		t.Fatalf("searched=%#v err=%v", searched, err)
	}
	first, err := repository.ListOrders(context.Background(), "org-orders", billing.OrderFilter{Limit: 50})
	if err != nil || len(first.Items) != 50 || first.NextCursor == "" {
		t.Fatalf("first page size=%d cursor=%q err=%v", len(first.Items), first.NextCursor, err)
	}
	second, err := repository.ListOrders(context.Background(), "org-orders", billing.OrderFilter{Limit: 50, Cursor: first.NextCursor})
	if err != nil || len(second.Items) != 50 || second.NextCursor == "" || first.Items[49].OrderID == second.Items[0].OrderID {
		t.Fatalf("second page size=%d cursor=%q err=%v", len(second.Items), second.NextCursor, err)
	}
	third, err := repository.ListOrders(context.Background(), "org-orders", billing.OrderFilter{Limit: 50, Cursor: second.NextCursor})
	if err != nil || len(third.Items) != 5 || third.NextCursor != "" {
		t.Fatalf("third page=%#v err=%v", third, err)
	}
	summary, err := repository.ReadOrderSummary(context.Background(), "org-orders", created.Add(-time.Hour), created.Add(time.Hour))
	if err != nil || summary.SpendMinor != 217 || summary.AIPointSpendMinor != 208 || summary.DataRowSpendMinor != 9 {
		t.Fatalf("summary=%#v err=%v", summary, err)
	}
}

func TestListOrdersFailsWhenLoadingCanonicalItemsFails(t *testing.T) {
	repository := commercialRepository(t)
	now := time.Now().UTC()
	if err := repository.db.Create(&orderRow{OrderID: "order-item-read-failure", OrganizationID: "org-item-read-failure", Kind: string(billing.OrderResourcePurchase), Currency: billing.CurrencyCNY, AmountMinor: 10, Status: string(billing.OrderFulfilled), IdempotencyKey: "idem-item-read-failure", RequestFingerprint: "fingerprint-item-read-failure", Version: 1, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.db.Migrator().DropTable(&orderItemRow{}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ListOrders(context.Background(), "org-item-read-failure", billing.OrderFilter{Limit: 10}); !errors.Is(err, billing.ErrFeatureUnavailable) {
		t.Fatalf("list orders after item lookup failure = %v; want dependency error", err)
	}
}

func TestResourcePurchasePersistsReservationAndGrantProofBeforeFulfillment(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:commercial-purchase-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, migrate := range []func(*gorm.DB) error{moneystore.AutoMigrate, AutoMigrate, orgresourceadapter.AutoMigrate} {
		if err := migrate(db); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	wallet, err := moneystore.New(db)
	if err != nil {
		t.Fatal(err)
	}
	commercial, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	resourceRepository, err := orgresourceadapter.NewGormRepository(db, orgresourceadapter.TransactionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	grantService, err := orgresource.NewPurchasedResourceGrantService(resourceRepository, orgresourceadapter.TrustedCommercialGrantAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := billing.NewService(commercial, commercial, commercial, commercial, wallet, grantService)
	if err != nil {
		t.Fatal(err)
	}
	if err := wallet.RecordPaymentSettlement(ctx, money.PaymentSettlement{PaymentID: "payment-purchase", PayerUserID: "user-a", Currency: "CNY", GrossAmountMinor: 100, CommissionableAmountMinor: 100, Status: money.PaymentSettled, SettledAt: time.Now().UTC(), ProviderReference: "provider-purchase", Version: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := wallet.CreditSettledTopUp(ctx, "", money.OrganizationTopUpSettlement{PaymentID: "payment-purchase", CommercialOrderID: "topup-purchase", OrganizationID: "org-purchase", Currency: "CNY", AmountMinor: 100, SettledAt: time.Now().UTC(), ProviderReference: "provider-purchase", Version: 1}); err != nil {
		t.Fatal(err)
	}
	if err := commercial.SaveOffer(ctx, billing.Offer{OfferID: "offer-purchase", ProductKind: billing.ProductAIPoint, ResourceType: orgresource.ResourceAIPoint, Currency: billing.CurrencyCNY, UnitPriceMinor: 7, PricingVersion: "pricing-1", MinQuantity: 1, MaxQuantity: 1000, Status: billing.OfferActive}); err != nil {
		t.Fatal(err)
	}
	quote, err := service.CreateQuote(ctx, billing.QuoteRequest{OrganizationID: "org-purchase", OfferID: "offer-purchase", Quantity: 10})
	if err != nil {
		t.Fatal(err)
	}
	order, err := service.CreateResourceOrder(ctx, billing.CreateResourceOrderRequest{OrganizationID: "org-purchase", QuoteID: quote.QuoteID, IdempotencyKey: "purchase-key"})
	if err != nil {
		t.Fatalf("CreateResourceOrder() error = %v; order=%#v", err, order)
	}
	if order.Status != billing.OrderFulfilled || order.WalletReservationState != money.WalletReservationCommitted || order.ResourceGrantOperationID == "" || order.ResourceGrantSourceIdentity != orgresource.CommercialOrderItemSourceIdentity(order.OrganizationID, order.OrderID, order.Items[0].OrderItemID) {
		t.Fatalf("fulfilled order proof = %#v", order)
	}
	walletSnapshot, err := wallet.ReadOrganizationWallet(ctx, "org-purchase", "CNY")
	if err != nil || walletSnapshot.AvailableMinor != 30 || walletSnapshot.ReservedMinor != 0 || walletSnapshot.LifetimeSpendMinor != 70 {
		t.Fatalf("wallet after purchase = %#v, err=%v", walletSnapshot, err)
	}
	lowFundsQuote, err := service.CreateQuote(ctx, billing.QuoteRequest{OrganizationID: "org-purchase", OfferID: "offer-purchase", Quantity: 10})
	if err != nil {
		t.Fatal(err)
	}
	lowFundsRequest := billing.CreateResourceOrderRequest{OrganizationID: "org-purchase", QuoteID: lowFundsQuote.QuoteID, IdempotencyKey: "purchase-low-funds"}
	firstLowFunds, err := service.CreateResourceOrder(ctx, lowFundsRequest)
	if !errors.Is(err, billing.ErrInsufficientFunds) || firstLowFunds.Status != billing.OrderCancelled || firstLowFunds.FailureCode != billing.OrderFailureInsufficientFunds {
		t.Fatalf("first low-funds request = %#v, err=%v", firstLowFunds, err)
	}
	replayedLowFunds, err := service.CreateResourceOrder(ctx, lowFundsRequest)
	if !errors.Is(err, billing.ErrInsufficientFunds) || replayedLowFunds.Status != billing.OrderCancelled || replayedLowFunds.FailureCode != billing.OrderFailureInsufficientFunds {
		t.Fatalf("replayed low-funds request = %#v, err=%v", replayedLowFunds, err)
	}
	if err := wallet.RecordPaymentSettlement(ctx, money.PaymentSettlement{PaymentID: "payment-terminal-grant", PayerUserID: "user-a", Currency: "CNY", GrossAmountMinor: 100, CommissionableAmountMinor: 100, Status: money.PaymentSettled, SettledAt: time.Now().UTC(), ProviderReference: "provider-terminal-grant", Version: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := wallet.CreditSettledTopUp(ctx, "", money.OrganizationTopUpSettlement{PaymentID: "payment-terminal-grant", CommercialOrderID: "topup-terminal-grant", OrganizationID: "org-purchase", Currency: "CNY", AmountMinor: 100, SettledAt: time.Now().UTC(), ProviderReference: "provider-terminal-grant", Version: 1}); err != nil {
		t.Fatal(err)
	}
	terminalService, err := billing.NewService(commercial, commercial, commercial, commercial, wallet, terminalPurchasedResourceGrant{})
	if err != nil {
		t.Fatal(err)
	}
	terminalQuote, err := terminalService.CreateQuote(ctx, billing.QuoteRequest{OrganizationID: "org-purchase", OfferID: "offer-purchase", Quantity: 10})
	if err != nil {
		t.Fatal(err)
	}
	terminalRequest := billing.CreateResourceOrderRequest{OrganizationID: "org-purchase", QuoteID: terminalQuote.QuoteID, IdempotencyKey: "purchase-terminal-grant"}
	firstTerminal, err := terminalService.CreateResourceOrder(ctx, terminalRequest)
	if !errors.Is(err, billing.ErrResourceGrantRejected) || firstTerminal.Status != billing.OrderCancelled || firstTerminal.FailureCode != billing.OrderFailureGrantRejected {
		t.Fatalf("first terminal grant failure = %#v, err=%v", firstTerminal, err)
	}
	replayedTerminal, err := terminalService.CreateResourceOrder(ctx, terminalRequest)
	if !errors.Is(err, billing.ErrResourceGrantRejected) || replayedTerminal.Status != billing.OrderCancelled || replayedTerminal.FailureCode != billing.OrderFailureGrantRejected {
		t.Fatalf("replayed terminal grant failure = %#v, err=%v", replayedTerminal, err)
	}
	updateFailureService, err := billing.NewService(commercial, commercial, failCancelledOrderUpdate{Repository: commercial}, commercial, wallet, grantService)
	if err != nil {
		t.Fatal(err)
	}
	updateFailureQuote, err := updateFailureService.CreateQuote(ctx, billing.QuoteRequest{OrganizationID: "org-purchase", OfferID: "offer-purchase", Quantity: 100})
	if err != nil {
		t.Fatal(err)
	}
	updateFailureRequest := billing.CreateResourceOrderRequest{OrganizationID: "org-purchase", QuoteID: updateFailureQuote.QuoteID, IdempotencyKey: "purchase-cancel-persistence-failure"}
	uncertainOrder, err := updateFailureService.CreateResourceOrder(ctx, updateFailureRequest)
	if !errors.Is(err, billing.ErrReconciliationRequired) || uncertainOrder.OrderID != "" {
		t.Fatalf("failed terminal persistence = %#v, err=%v; want unknown reconciliation outcome", uncertainOrder, err)
	}
	persistedAfterFailure, found, err := commercial.FindResourceOrderByIdempotency(ctx, "org-purchase", updateFailureRequest.IdempotencyKey)
	if err != nil || !found || persistedAfterFailure.Status != billing.OrderPending {
		t.Fatalf("order after cancellation persistence failure = %#v found=%v err=%v", persistedAfterFailure, found, err)
	}
	var resourceBalance struct{ Available int64 }
	if err := db.WithContext(ctx).Table("saas_organization_resource_buckets").Select("available").Where("organization_id = ? AND resource_type = ?", "org-purchase", orgresource.ResourceAIPoint).Take(&resourceBalance).Error; err != nil || resourceBalance.Available != 10 {
		t.Fatalf("resource balance after purchase = %#v, err=%v", resourceBalance, err)
	}
	releaseFailureStore := &failFirstCancelledOrderUpdate{Repository: commercial}
	releaseFailureService, err := billing.NewService(commercial, commercial, releaseFailureStore, commercial, wallet, terminalPurchasedResourceGrant{})
	if err != nil {
		t.Fatal(err)
	}
	releaseFailureQuote, err := releaseFailureService.CreateQuote(ctx, billing.QuoteRequest{OrganizationID: "org-purchase", OfferID: "offer-purchase", Quantity: 10})
	if err != nil {
		t.Fatal(err)
	}
	releaseFailureRequest := billing.CreateResourceOrderRequest{OrganizationID: "org-purchase", QuoteID: releaseFailureQuote.QuoteID, IdempotencyKey: "purchase-release-before-cancel-failure"}
	uncertainRelease, err := releaseFailureService.CreateResourceOrder(ctx, releaseFailureRequest)
	if !errors.Is(err, billing.ErrReconciliationRequired) || uncertainRelease.OrderID != "" {
		t.Fatalf("release before cancellation persistence failure = %#v, err=%v", uncertainRelease, err)
	}
	persistedReleaseFailure, found, err := commercial.FindResourceOrderByIdempotency(ctx, "org-purchase", releaseFailureRequest.IdempotencyKey)
	if err != nil || !found || persistedReleaseFailure.Status != billing.OrderFundsReserved {
		t.Fatalf("durable order after cancellation persistence failure = %#v found=%v err=%v", persistedReleaseFailure, found, err)
	}
	recoveredRelease, err := service.CreateResourceOrder(ctx, releaseFailureRequest)
	if !errors.Is(err, billing.ErrResourceGrantRejected) || recoveredRelease.Status != billing.OrderCancelled {
		t.Fatalf("reconcile released reservation = %#v, err=%v; must cancel without retrying grant", recoveredRelease, err)
	}
	resourceBalance.Available = 0
	if err := db.WithContext(ctx).Table("saas_organization_resource_buckets").Select("available").Where("organization_id = ? AND resource_type = ?", "org-purchase", orgresource.ResourceAIPoint).Take(&resourceBalance).Error; err != nil || resourceBalance.Available != 10 {
		t.Fatalf("resource balance after released-reservation recovery = %#v, err=%v", resourceBalance, err)
	}
}

func TestResourceGrantIdempotencyConflictKeepsReservationForReconciliation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:commercial-grant-conflict-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, migrate := range []func(*gorm.DB) error{moneystore.AutoMigrate, AutoMigrate, orgresourceadapter.AutoMigrate} {
		if err := migrate(db); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	wallet, err := moneystore.New(db)
	if err != nil {
		t.Fatal(err)
	}
	commercial, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	resourceRepository, err := orgresourceadapter.NewGormRepository(db, orgresourceadapter.TransactionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	grantService, err := orgresource.NewPurchasedResourceGrantService(resourceRepository, orgresourceadapter.TrustedCommercialGrantAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	conflictGrant := &conflictingPurchasedResourceGrant{delegate: grantService}
	service, err := billing.NewService(commercial, commercial, commercial, commercial, wallet, conflictGrant)
	if err != nil {
		t.Fatal(err)
	}
	if err := wallet.RecordPaymentSettlement(ctx, money.PaymentSettlement{PaymentID: "payment-grant-conflict", PayerUserID: "user-a", Currency: "CNY", GrossAmountMinor: 100, CommissionableAmountMinor: 100, Status: money.PaymentSettled, SettledAt: time.Now().UTC(), ProviderReference: "provider-grant-conflict", Version: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := wallet.CreditSettledTopUp(ctx, "", money.OrganizationTopUpSettlement{PaymentID: "payment-grant-conflict", CommercialOrderID: "topup-grant-conflict", OrganizationID: "org-grant-conflict", Currency: "CNY", AmountMinor: 100, SettledAt: time.Now().UTC(), ProviderReference: "provider-grant-conflict", Version: 1}); err != nil {
		t.Fatal(err)
	}
	if err := commercial.SaveOffer(ctx, billing.Offer{OfferID: "offer-grant-conflict", ProductKind: billing.ProductAIPoint, ResourceType: orgresource.ResourceAIPoint, Currency: billing.CurrencyCNY, UnitPriceMinor: 7, PricingVersion: "pricing-1", MinQuantity: 1, MaxQuantity: 1000, Status: billing.OfferActive}); err != nil {
		t.Fatal(err)
	}
	quote, err := service.CreateQuote(ctx, billing.QuoteRequest{OrganizationID: "org-grant-conflict", OfferID: "offer-grant-conflict", Quantity: 10})
	if err != nil {
		t.Fatal(err)
	}
	request := billing.CreateResourceOrderRequest{OrganizationID: "org-grant-conflict", QuoteID: quote.QuoteID, IdempotencyKey: "purchase-grant-conflict"}
	order, err := service.CreateResourceOrder(ctx, request)
	if !errors.Is(err, billing.ErrReconciliationRequired) || order.Status != billing.OrderReconciliationRequired {
		t.Fatalf("grant conflict outcome = %#v, err=%v; want reconciliation required", order, err)
	}
	walletSnapshot, err := wallet.ReadOrganizationWallet(ctx, "org-grant-conflict", "CNY")
	if err != nil || walletSnapshot.AvailableMinor != 30 || walletSnapshot.ReservedMinor != 70 || walletSnapshot.LifetimeSpendMinor != 0 {
		t.Fatalf("wallet after grant conflict = %#v, err=%v; reservation must remain held", walletSnapshot, err)
	}
	var resourceBalance struct{ Available int64 }
	if err := db.WithContext(ctx).Table("saas_organization_resource_buckets").Select("available").Where("organization_id = ? AND resource_type = ?", "org-grant-conflict", orgresource.ResourceAIPoint).Take(&resourceBalance).Error; err != nil || resourceBalance.Available != 9 {
		t.Fatalf("resource balance after conflicting source claim = %#v, err=%v", resourceBalance, err)
	}
	replayed, err := service.CreateResourceOrder(ctx, request)
	if !errors.Is(err, billing.ErrReconciliationRequired) || replayed.Status != billing.OrderReconciliationRequired {
		t.Fatalf("replayed grant conflict = %#v, err=%v; reservation must remain held", replayed, err)
	}
	walletSnapshot, err = wallet.ReadOrganizationWallet(ctx, "org-grant-conflict", "CNY")
	if err != nil || walletSnapshot.ReservedMinor != 70 || walletSnapshot.LifetimeSpendMinor != 0 {
		t.Fatalf("wallet after conflicting replay = %#v, err=%v; reservation must remain held", walletSnapshot, err)
	}
}

func TestResourcePurchaseReplayReconcilesLostCommitAcknowledgement(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:commercial-reconcile-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, migrate := range []func(*gorm.DB) error{moneystore.AutoMigrate, AutoMigrate, orgresourceadapter.AutoMigrate} {
		if err := migrate(db); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	walletRepository, err := moneystore.New(db)
	if err != nil {
		t.Fatal(err)
	}
	wallet := &loseFirstCommitAcknowledgement{OrganizationWalletReader: walletRepository, OrganizationWalletCommander: walletRepository, lose: true}
	commercial, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	resourceRepository, err := orgresourceadapter.NewGormRepository(db, orgresourceadapter.TransactionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	grantService, err := orgresource.NewPurchasedResourceGrantService(resourceRepository, orgresourceadapter.TrustedCommercialGrantAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := billing.NewService(commercial, commercial, commercial, commercial, wallet, grantService)
	if err != nil {
		t.Fatal(err)
	}
	if err := walletRepository.RecordPaymentSettlement(ctx, money.PaymentSettlement{PaymentID: "payment-reconcile", PayerUserID: "user-a", Currency: "CNY", GrossAmountMinor: 100, CommissionableAmountMinor: 100, Status: money.PaymentSettled, SettledAt: time.Now().UTC(), ProviderReference: "provider-reconcile", Version: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := walletRepository.CreditSettledTopUp(ctx, "", money.OrganizationTopUpSettlement{PaymentID: "payment-reconcile", CommercialOrderID: "topup-reconcile", OrganizationID: "org-reconcile", Currency: "CNY", AmountMinor: 100, SettledAt: time.Now().UTC(), ProviderReference: "provider-reconcile", Version: 1}); err != nil {
		t.Fatal(err)
	}
	if err := commercial.SaveOffer(ctx, billing.Offer{OfferID: "offer-reconcile", ProductKind: billing.ProductAIPoint, ResourceType: orgresource.ResourceAIPoint, Currency: billing.CurrencyCNY, UnitPriceMinor: 7, PricingVersion: "pricing-1", MinQuantity: 1, MaxQuantity: 1000, Status: billing.OfferActive}); err != nil {
		t.Fatal(err)
	}
	quote, err := service.CreateQuote(ctx, billing.QuoteRequest{OrganizationID: "org-reconcile", OfferID: "offer-reconcile", Quantity: 10})
	if err != nil {
		t.Fatal(err)
	}
	request := billing.CreateResourceOrderRequest{OrganizationID: "org-reconcile", QuoteID: quote.QuoteID, IdempotencyKey: "reconcile-key"}
	first, err := service.CreateResourceOrder(ctx, request)
	if !errors.Is(err, billing.ErrReconciliationRequired) || first.Status != billing.OrderReconciliationRequired {
		t.Fatalf("first purchase = %#v, err=%v; want persisted reconciliation", first, err)
	}
	recovered, err := service.CreateResourceOrder(ctx, request)
	if err != nil || recovered.Status != billing.OrderFulfilled || recovered.WalletReservationState != money.WalletReservationCommitted || recovered.ResourceGrantOperationID == "" {
		t.Fatalf("replayed purchase = %#v, err=%v", recovered, err)
	}
	walletSnapshot, err := walletRepository.ReadOrganizationWallet(ctx, "org-reconcile", "CNY")
	if err != nil || walletSnapshot.AvailableMinor != 30 || walletSnapshot.ReservedMinor != 0 || walletSnapshot.LifetimeSpendMinor != 70 {
		t.Fatalf("wallet after reconciliation = %#v, err=%v", walletSnapshot, err)
	}
}

func TestResourcePurchaseReplayResumesDurableIntermediateOrders(t *testing.T) {
	for _, initialStatus := range []billing.OrderStatus{billing.OrderFundsReserved, billing.OrderFulfilling} {
		t.Run(string(initialStatus), func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open("file:commercial-resume-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			for _, migrate := range []func(*gorm.DB) error{moneystore.AutoMigrate, AutoMigrate, orgresourceadapter.AutoMigrate} {
				if err := migrate(db); err != nil {
					t.Fatal(err)
				}
			}
			ctx := context.Background()
			wallet, err := moneystore.New(db)
			if err != nil {
				t.Fatal(err)
			}
			commercial, err := New(db)
			if err != nil {
				t.Fatal(err)
			}
			resourceRepository, err := orgresourceadapter.NewGormRepository(db, orgresourceadapter.TransactionConfig{})
			if err != nil {
				t.Fatal(err)
			}
			grantService, err := orgresource.NewPurchasedResourceGrantService(resourceRepository, orgresourceadapter.TrustedCommercialGrantAuthorizer{})
			if err != nil {
				t.Fatal(err)
			}
			service, err := billing.NewService(commercial, commercial, commercial, commercial, wallet, grantService)
			if err != nil {
				t.Fatal(err)
			}
			orgID := "org-resume-" + string(initialStatus)
			paymentID := "payment-resume-" + string(initialStatus)
			if err := wallet.RecordPaymentSettlement(ctx, money.PaymentSettlement{PaymentID: paymentID, PayerUserID: "user-a", Currency: "CNY", GrossAmountMinor: 100, CommissionableAmountMinor: 100, Status: money.PaymentSettled, SettledAt: time.Now().UTC(), ProviderReference: "provider-" + paymentID, Version: 1}); err != nil {
				t.Fatal(err)
			}
			if _, err := wallet.CreditSettledTopUp(ctx, "", money.OrganizationTopUpSettlement{PaymentID: paymentID, CommercialOrderID: "topup-" + paymentID, OrganizationID: orgID, Currency: "CNY", AmountMinor: 100, SettledAt: time.Now().UTC(), ProviderReference: "provider-" + paymentID, Version: 1}); err != nil {
				t.Fatal(err)
			}
			offerID := "offer-" + string(initialStatus)
			if err := commercial.SaveOffer(ctx, billing.Offer{OfferID: offerID, ProductKind: billing.ProductAIPoint, ResourceType: orgresource.ResourceAIPoint, Currency: billing.CurrencyCNY, UnitPriceMinor: 7, PricingVersion: "pricing-1", MinQuantity: 1, MaxQuantity: 1000, Status: billing.OfferActive}); err != nil {
				t.Fatal(err)
			}
			quote, err := service.CreateQuote(ctx, billing.QuoteRequest{OrganizationID: orgID, OfferID: offerID, Quantity: 10})
			if err != nil {
				t.Fatal(err)
			}
			request := billing.CreateResourceOrderRequest{OrganizationID: orgID, QuoteID: quote.QuoteID, IdempotencyKey: "retry-" + string(initialStatus)}
			order, err := commercial.CreatePendingResourceOrder(ctx, request, quote)
			if err != nil {
				t.Fatal(err)
			}
			reservation, err := wallet.ReserveCommercialPurchase(ctx, money.ReserveWalletFundsInput{OperationID: order.OrderID, OrganizationID: orgID, CommercialOrderID: order.OrderID, Currency: quote.Currency, AmountMinor: quote.TotalMinor})
			if err != nil {
				t.Fatal(err)
			}
			order.WalletReservationID = reservation.ReservationID
			order.WalletReservationState = reservation.State
			order.Status = initialStatus
			if initialStatus == billing.OrderFulfilling {
				grant, grantErr := grantService.GrantPurchasedResource(ctx, orgresource.PurchasedResourceGrantInput{OrganizationID: orgID, OperationID: "grant:" + order.OrderID, CommercialOrderID: order.OrderID, CommercialOrderItemID: order.Items[0].OrderItemID, ResourceType: order.Items[0].ResourceType, Quantity: order.Items[0].ResourceQuantity, Principal: orgresource.Principal{ID: "commercial-billing", Kind: orgresource.PrincipalTrustedCommercial}})
				if grantErr != nil {
					t.Fatal(grantErr)
				}
				order.ResourceGrantOperationID = grant.Snapshot.OperationID
				order.ResourceGrantSourceType = grant.Snapshot.SourceType
				order.ResourceGrantSourceIdentity = grant.Snapshot.SourceIdentity
			}
			order.UpdatedAt = time.Now().UTC()
			if err := commercial.UpdateOrder(ctx, order); err != nil {
				t.Fatal(err)
			}

			recoveryService := service
			if initialStatus == billing.OrderFundsReserved {
				uncertainWallet := &loseFirstCommitAcknowledgement{OrganizationWalletReader: wallet, OrganizationWalletCommander: wallet, lose: true}
				recoveryService, err = billing.NewService(commercial, commercial, commercial, commercial, uncertainWallet, grantService)
				if err != nil {
					t.Fatal(err)
				}
				firstRecovery, recoveryErr := recoveryService.CreateResourceOrder(ctx, request)
				if !errors.Is(recoveryErr, billing.ErrReconciliationRequired) || firstRecovery.Status != billing.OrderReconciliationRequired || firstRecovery.ResourceGrantOperationID == "" {
					t.Fatalf("lost commit acknowledgement after FUNDS_RESERVED recovery = %#v, err=%v; grant proof must already be durable", firstRecovery, recoveryErr)
				}
				persistedRecovery, readErr := commercial.ReadOrder(ctx, orgID, order.OrderID)
				if readErr != nil || persistedRecovery.Status != billing.OrderReconciliationRequired || persistedRecovery.ResourceGrantOperationID == "" {
					t.Fatalf("durable recovery after lost commit acknowledgement = %#v, err=%v", persistedRecovery, readErr)
				}
			}
			recovered, err := recoveryService.CreateResourceOrder(ctx, request)
			if err != nil || recovered.Status != billing.OrderFulfilled || recovered.WalletReservationState != money.WalletReservationCommitted {
				t.Fatalf("recovered order = %#v, err=%v", recovered, err)
			}
			walletSnapshot, err := wallet.ReadOrganizationWallet(ctx, orgID, "CNY")
			if err != nil || walletSnapshot.AvailableMinor != 30 || walletSnapshot.ReservedMinor != 0 || walletSnapshot.LifetimeSpendMinor != 70 {
				t.Fatalf("wallet after recovery = %#v, err=%v", walletSnapshot, err)
			}
		})
	}
}
