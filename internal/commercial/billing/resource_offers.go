package billing

import (
	"context"
)

type ResourceOfferCatalog interface {
	ListResourceOffers(context.Context) ([]Offer, error)
}

type ResourceAmountQuoteRequest struct {
	OrganizationID string
	OfferID        string
	AmountMinor    int64
}

type ResourceAmountQuote struct {
	Quote          Quote
	AmountMinor    int64
	UnitPriceMinor int64
	RemainderMinor int64
}

func (s *Service) ListResourceOffers(ctx context.Context) ([]Offer, error) {
	if s == nil {
		return nil, ErrFeatureUnavailable
	}
	catalog, ok := s.offers.(ResourceOfferCatalog)
	if !ok {
		return nil, ErrFeatureUnavailable
	}
	offers, err := catalog.ListResourceOffers(ctx)
	if err != nil {
		return nil, err
	}
	if len(offers) > 100 {
		return nil, ErrFeatureUnavailable
	}
	for _, offer := range offers {
		if offer.Validate() != nil || offer.ProductKind == ProductSubscriptionPlan || offer.UnitPriceMinor <= 0 || offer.Status != OfferActive {
			return nil, ErrOfferUnavailable
		}
	}
	return offers, nil
}

func (s *Service) CreateAmountQuote(ctx context.Context, request ResourceAmountQuoteRequest) (ResourceAmountQuote, error) {
	if s == nil || s.offers == nil || s.quotes == nil {
		return ResourceAmountQuote{}, ErrFeatureUnavailable
	}
	if !isCanonicalIdentifier(request.OrganizationID) || !isCanonicalIdentifier(request.OfferID) || request.AmountMinor <= 0 {
		return ResourceAmountQuote{}, ErrInvalid
	}
	offer, err := s.offers.ReadOffer(ctx, request.OfferID)
	if err != nil {
		return ResourceAmountQuote{}, err
	}
	if offer.Validate() != nil || offer.Status != OfferActive || offer.UnitPriceMinor <= 0 || (offer.ProductKind != ProductAIPoint && offer.ProductKind != ProductDataRow) {
		return ResourceAmountQuote{}, ErrOfferUnavailable
	}
	quantity := request.AmountMinor / offer.UnitPriceMinor
	if quantity < offer.MinQuantity || quantity > offer.MaxQuantity {
		return ResourceAmountQuote{}, ErrInvalid
	}
	quote, err := s.quotes.CreateQuote(ctx, QuoteRequest{OrganizationID: request.OrganizationID, OfferID: request.OfferID, Quantity: quantity})
	if err != nil {
		return ResourceAmountQuote{}, err
	}
	if quote.Validate() != nil || quote.OrganizationID != request.OrganizationID || quote.OfferID != offer.OfferID || quote.PricingVersion != offer.PricingVersion || quote.Currency != offer.Currency || quote.ProductKind != offer.ProductKind || quote.ResourceType != offer.ResourceType || quote.ResourceQuantity != quantity || quote.TotalMinor != quantity*offer.UnitPriceMinor {
		return ResourceAmountQuote{}, ErrQuoteExpired
	}
	return ResourceAmountQuote{Quote: quote, AmountMinor: request.AmountMinor, UnitPriceMinor: offer.UnitPriceMinor, RemainderMinor: request.AmountMinor - quote.TotalMinor}, nil
}
