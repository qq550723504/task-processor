package commercialbilling

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/commercial/billing"
	"testing"
)

func TestResourceAmountQuoteOnlyChargesWholeUnitsWithinBudget(t *testing.T) {
	ctx := context.Background()
	commercial, wallet, grants := resourceRecoveryOwners(t, true)
	service, err := billing.NewService(commercial, commercial, commercial, commercial, wallet, grants)
	require.NoError(t, err)
	result, err := service.CreateAmountQuote(ctx, billing.ResourceAmountQuoteRequest{OrganizationID: "recovery-org", OfferID: "recovery-offer", AmountMinor: 71})
	require.NoError(t, err)
	require.Equal(t, int64(10), result.Quote.ResourceQuantity)
	require.Equal(t, int64(70), result.Quote.TotalMinor)
	require.Equal(t, int64(1), result.RemainderMinor)
	require.Equal(t, int64(7), result.UnitPriceMinor)
	_, err = service.CreateAmountQuote(ctx, billing.ResourceAmountQuoteRequest{OrganizationID: "recovery-org", OfferID: "recovery-offer", AmountMinor: 6})
	require.ErrorIs(t, err, billing.ErrInvalid)
	_, err = service.CreateAmountQuote(ctx, billing.ResourceAmountQuoteRequest{OrganizationID: "recovery-org", OfferID: "missing", AmountMinor: 100})
	require.ErrorIs(t, err, billing.ErrOfferUnavailable)
	offers, err := service.ListResourceOffers(ctx)
	require.NoError(t, err)
	require.Len(t, offers, 1)
	require.Equal(t, "recovery-offer", offers[0].OfferID)
}

type changePriceAtQuote struct {
	billing.QuoteEngine
	repo *Repository
}

func (q changePriceAtQuote) CreateQuote(ctx context.Context, input billing.QuoteRequest) (billing.Quote, error) {
	offer, err := q.repo.ReadOffer(ctx, input.OfferID)
	if err != nil {
		return billing.Quote{}, err
	}
	offer.UnitPriceMinor = 14
	offer.PricingVersion = "price-changed"
	if err := q.repo.SaveOffer(ctx, offer); err != nil {
		return billing.Quote{}, err
	}
	return q.QuoteEngine.CreateQuote(ctx, input)
}

func TestResourceAmountQuoteRequiresFreshConfirmationWhenPriceChanges(t *testing.T) {
	commercial, wallet, grants := resourceRecoveryOwners(t, true)
	service, err := billing.NewService(commercial, changePriceAtQuote{QuoteEngine: commercial, repo: commercial}, commercial, commercial, wallet, grants)
	require.NoError(t, err)
	_, err = service.CreateAmountQuote(context.Background(), billing.ResourceAmountQuoteRequest{OrganizationID: "recovery-org", OfferID: "recovery-offer", AmountMinor: 71})
	require.ErrorIs(t, err, billing.ErrQuoteExpired)
}
