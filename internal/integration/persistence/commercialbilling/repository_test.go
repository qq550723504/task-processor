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
		orders = append(orders, orderRow{OrderID: id, OrganizationID: "org-orders", Kind: string(billing.OrderResourcePurchase), Currency: billing.CurrencyCNY, AmountMinor: amount, Status: string(billing.OrderFulfilled), IdempotencyKey: "idem-" + id, RequestFingerprint: "fingerprint-" + id, Version: 1, CreatedAt: created, UpdatedAt: created})
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
	var resourceBalance struct{ Available int64 }
	if err := db.WithContext(ctx).Table("saas_organization_resource_buckets").Select("available").Where("organization_id = ? AND resource_type = ?", "org-purchase", orgresource.ResourceAIPoint).Take(&resourceBalance).Error; err != nil || resourceBalance.Available != 10 {
		t.Fatalf("resource balance after purchase = %#v, err=%v", resourceBalance, err)
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
