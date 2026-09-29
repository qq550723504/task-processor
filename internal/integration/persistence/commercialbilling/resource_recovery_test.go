package commercialbilling

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"task-processor/internal/commercial/billing"
	resourceadapter "task-processor/internal/integration/orgresource"
	moneystore "task-processor/internal/integration/persistence/money"
	"task-processor/internal/ledger/money"
	"task-processor/internal/ledger/orgresource"
)

// These are separate owner databases, including the failure window between
// money's durable decision and commercial's acknowledgement of its identity.
func resourceRecoveryOwners(t *testing.T, credit bool) (*Repository, *moneystore.Repository, *orgresource.PurchasedResourceGrantService) {
	t.Helper()
	open := func(name string, migrate func(*gorm.DB) error) *gorm.DB {
		db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), name+".db")), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, migrate(db))
		pool, err := db.DB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = pool.Close() })
		return db
	}
	commercial, err := New(open("commercial", AutoMigrate))
	require.NoError(t, err)
	wallet, err := moneystore.New(open("money", moneystore.AutoMigrate))
	require.NoError(t, err)
	repository, err := resourceadapter.NewGormRepository(open("resource", resourceadapter.AutoMigrate), resourceadapter.TransactionConfig{})
	require.NoError(t, err)
	grants, err := orgresource.NewPurchasedResourceGrantService(repository, resourceadapter.TrustedCommercialGrantAuthorizer{})
	require.NoError(t, err)
	if credit {
		creditRecoveryWallet(t, wallet, "initial")
	}
	require.NoError(t, commercial.CreateOffer(context.Background(), billing.Offer{OfferID: "recovery-offer", ProductKind: billing.ProductDataRow, ResourceType: orgresource.ResourceDataRow, Currency: "CNY", UnitPriceMinor: 7, PricingVersion: "test-price", MinQuantity: 1, MaxQuantity: 100, Status: billing.OfferActive}))
	return commercial, wallet, grants
}

func creditRecoveryWallet(t *testing.T, wallet *moneystore.Repository, suffix string) {
	t.Helper()
	ctx, now := context.Background(), time.Now().UTC()
	// Reuse the money owner's synthetic settlement fixture. This test does not
	// invoke a provider or validate the separate public wallet top-up flow.
	require.NoError(t, wallet.RecordPaymentSettlement(ctx, money.PaymentSettlement{PaymentID: "payment-" + suffix, PayerUserID: "synthetic", Currency: "CNY", GrossAmountMinor: 100, CommissionableAmountMinor: 100, Status: money.PaymentSettled, SettledAt: now, ProviderReference: "synthetic-" + suffix, Version: 1}))
	_, err := wallet.CreditSettledTopUp(ctx, "", money.OrganizationTopUpSettlement{PaymentID: "payment-" + suffix, CommercialOrderID: "topup-" + suffix, OrganizationID: "recovery-org", Currency: "CNY", AmountMinor: 100, SettledAt: now, ProviderReference: "synthetic-" + suffix, Version: 1})
	require.NoError(t, err)
}

type loseReserveReceipt struct {
	money.OrganizationWalletReader
	money.OrganizationWalletCommander
	calls int
}

func (w *loseReserveReceipt) ReserveCommercialPurchase(ctx context.Context, input money.ReserveWalletFundsInput) (money.WalletReservation, error) {
	w.calls++
	_, err := w.OrganizationWalletCommander.ReserveCommercialPurchase(ctx, input)
	if err != nil && !errors.Is(err, money.ErrWalletInsufficientBalance) {
		return money.WalletReservation{}, err
	}
	return money.WalletReservation{}, errors.New("synthetic reserve response lost")
}

func TestResourceOrderRecoversOriginalReserveDecisionWithoutReservingAgain(t *testing.T) {
	for _, credit := range []bool{true, false} {
		name := "reserved"
		if !credit {
			name = "rejected"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			commercial, wallet, grants := resourceRecoveryOwners(t, credit)
			lost := &loseReserveReceipt{OrganizationWalletReader: wallet, OrganizationWalletCommander: wallet}
			service, err := billing.NewService(commercial, commercial, commercial, commercial, lost, grants)
			require.NoError(t, err)
			quote, err := service.CreateQuote(ctx, billing.QuoteRequest{OrganizationID: "recovery-org", OfferID: "recovery-offer", Quantity: 10})
			require.NoError(t, err)
			request := billing.CreateResourceOrderRequest{ActorID: "test-admin", OrganizationID: "recovery-org", QuoteID: quote.QuoteID, IdempotencyKey: "original"}
			first, err := service.CreateResourceOrder(ctx, request)
			require.ErrorIs(t, err, billing.ErrReconciliationRequired)
			require.NotEmpty(t, first.OrderID)
			if !credit {
				creditRecoveryWallet(t, wallet, "later")
			}
			// A restarted billing service only reads the original money decision.
			restarted, err := billing.NewService(commercial, commercial, commercial, commercial, lost, grants)
			require.NoError(t, err)
			resolved, err := restarted.ReconcileResourceOrder(ctx, "recovery-org", first.OrderID)
			if credit {
				require.NoError(t, err)
				require.Equal(t, billing.OrderFulfilled, resolved.Status)
			} else {
				require.ErrorIs(t, err, billing.ErrInsufficientFunds)
				require.Equal(t, billing.OrderCancelled, resolved.Status)
			}
			require.Equal(t, 1, lost.calls)
			balance, err := wallet.ReadOrganizationWallet(ctx, "recovery-org", "CNY")
			require.NoError(t, err)
			if credit {
				require.Equal(t, int64(30), balance.AvailableMinor)
				require.Equal(t, int64(70), balance.LifetimeSpendMinor)
			} else {
				require.Equal(t, int64(100), balance.AvailableMinor)
				require.Zero(t, balance.LifetimeSpendMinor)
			}
			require.Zero(t, balance.ReservedMinor)
			// The public replay must also discover the original decision.
			again, replayErr := restarted.CreateResourceOrder(ctx, request)
			require.Equal(t, resolved.OrderID, again.OrderID)
			if credit {
				require.NoError(t, replayErr)
			} else {
				require.ErrorIs(t, replayErr, billing.ErrInsufficientFunds)
			}
			require.Equal(t, 1, lost.calls)
		})
	}
}

func TestResourceOrderRecoveryWithoutDecisionKeepsPendingAndDoesNotReserve(t *testing.T) {
	ctx := context.Background()
	commercial, wallet, grants := resourceRecoveryOwners(t, true)
	lost := &loseReserveReceipt{OrganizationWalletReader: wallet, OrganizationWalletCommander: wallet}
	service, err := billing.NewService(commercial, commercial, commercial, commercial, lost, grants)
	require.NoError(t, err)
	quote, err := service.CreateQuote(ctx, billing.QuoteRequest{OrganizationID: "recovery-org", OfferID: "recovery-offer", Quantity: 10})
	require.NoError(t, err)
	order, err := commercial.CreatePendingResourceOrder(ctx, billing.CreateResourceOrderRequest{ActorID: "test-admin", OrganizationID: "recovery-org", QuoteID: quote.QuoteID, IdempotencyKey: "not-dispatched"}, quote)
	require.NoError(t, err)
	unknown, err := service.ReconcileResourceOrder(ctx, order.OrganizationID, order.OrderID)
	require.ErrorIs(t, err, billing.ErrReconciliationRequired)
	require.Equal(t, billing.OrderPending, unknown.Status)
	require.Empty(t, unknown.WalletReservationID)
	require.Zero(t, lost.calls)
	durable, err := commercial.ReadOrder(ctx, order.OrganizationID, order.OrderID)
	require.NoError(t, err)
	require.NoError(t, durable.Validate())
	require.Equal(t, billing.OrderPending, durable.Status)
}

func TestResourcePurchasePersistsOriginalActorAndRejectsActorSwap(t *testing.T) {
	ctx := context.Background()
	commercial, wallet, grants := resourceRecoveryOwners(t, true)
	service, err := billing.NewService(commercial, commercial, commercial, commercial, wallet, grants)
	require.NoError(t, err)
	quote, err := service.CreateQuote(ctx, billing.QuoteRequest{OrganizationID: "recovery-org", OfferID: "recovery-offer", Quantity: 10})
	require.NoError(t, err)
	request := billing.CreateResourceOrderRequest{ActorID: "original-admin", OrganizationID: "recovery-org", QuoteID: quote.QuoteID, IdempotencyKey: "original-actor"}
	result, err := service.CreateResourceOrder(ctx, request)
	require.NoError(t, err)
	require.Equal(t, "original-admin", result.ActorID)
	durable, err := commercial.ReadOrder(ctx, result.OrganizationID, result.OrderID)
	require.NoError(t, err)
	require.Equal(t, "original-admin", durable.ActorID)
	request.ActorID = "another-admin"
	_, err = service.CreateResourceOrder(ctx, request)
	require.ErrorIs(t, err, billing.ErrConflict)
	request.ActorID = ""
	_, err = service.CreateResourceOrder(ctx, request)
	require.ErrorIs(t, err, billing.ErrInvalid)
}
