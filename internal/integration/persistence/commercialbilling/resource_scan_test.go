package commercialbilling

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/ledger/money"
)

func TestResourceRecoveryScanAdvancesPastUnknownOrders(t *testing.T) {
	ctx := context.Background()
	commercial, wallet, grants := resourceRecoveryOwners(t, true)
	lost := &loseReserveReceipt{OrganizationWalletReader: wallet, OrganizationWalletCommander: wallet}
	service, err := billing.NewService(commercial, commercial, commercial, commercial, lost, grants)
	require.NoError(t, err)
	quote, err := service.CreateQuote(ctx, billing.QuoteRequest{OrganizationID: "recovery-org", OfferID: "recovery-offer", Quantity: 10})
	require.NoError(t, err)
	var tail billing.Order
	for i := 0; i < 55; i++ {
		tail, err = commercial.CreatePendingResourceOrder(ctx, billing.CreateResourceOrderRequest{ActorID: "test-admin", OrganizationID: "recovery-org", QuoteID: quote.QuoteID, IdempotencyKey: fmt.Sprintf("original-%02d", i)}, quote)
		require.NoError(t, err)
	}
	// First 54 operations have no money decision; the last operation does.
	_, err = wallet.ReserveCommercialPurchase(ctx, money.ReserveWalletFundsInput{OperationID: tail.OrderID, OrganizationID: tail.OrganizationID, CommercialOrderID: tail.OrderID, Currency: tail.Currency, AmountMinor: tail.AmountMinor})
	require.NoError(t, err)
	_ = service.ReconcileRecoverableResourceOrders(ctx, 50)
	_ = service.ReconcileRecoverableResourceOrders(ctx, 50)
	result, err := commercial.ReadOrder(ctx, tail.OrganizationID, tail.OrderID)
	require.NoError(t, err)
	require.Equal(t, billing.OrderFulfilled, result.Status)
	require.Zero(t, lost.calls, "the background scan cannot start reserve effects")
	_ = service.ReconcileRecoverableResourceOrders(ctx, 50)
	result, err = commercial.ReadOrder(ctx, tail.OrganizationID, tail.OrderID)
	require.NoError(t, err)
	require.Equal(t, billing.OrderFulfilled, result.Status)
}
