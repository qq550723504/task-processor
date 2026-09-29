package httpapi

import (
	"github.com/gin-gonic/gin"
	"strconv"
	"task-processor/internal/authidentity"
	billing "task-processor/internal/commercial/billing"
	"task-processor/internal/ledger/orgresource"
	"time"
)

func (m memberResourcesModule) dataPrices(c *gin.Context) {
	id, ok := m.identity(c, false)
	if !ok {
		return
	}
	if !emptyResourceRead(c) {
		return
	}
	if m.prices == nil {
		writeMemberResourceError(c, billing.ErrOfferUnavailable)
		return
	}
	offers, err := m.prices.ListDataRowOffers(c.Request.Context())
	if err != nil {
		writeMemberResourceError(c, err)
		return
	}
	items := make([]gin.H, 0, len(offers))
	for _, offer := range offers {
		if offer.Validate() != nil || offer.ProductKind != billing.ProductDataRow || offer.ResourceType != orgresource.ResourceDataRow || offer.UnitPriceMinor <= 0 {
			writeMemberResourceError(c, billing.ErrOfferUnavailable)
			return
		}
		items = append(items, gin.H{"offerId": offer.OfferID, "pricingVersion": offer.PricingVersion, "currency": offer.Currency, "unitPriceMinor": strconv.FormatInt(offer.UnitPriceMinor, 10), "minQuantity": strconv.FormatInt(offer.MinQuantity, 10), "maxQuantity": strconv.FormatInt(offer.MaxQuantity, 10)})
	}
	writePointLimitJSON(c, 200, gin.H{"organizationId": id.EffectiveOrganizationID, "offers": items})
}
func (m memberResourcesModule) dataQuote(c *gin.Context) {
	id, ok := m.identity(c, true)
	if !ok {
		return
	}
	var input struct {
		OfferID     string `json:"offerId"`
		AmountMinor string `json:"amountMinor"`
	}
	if err := decodeMemberResourceBody(c.Request, &input); err != nil {
		writeMemberResourceError(c, err)
		return
	}
	amount, err := resourceInteger(input.AmountMinor, true)
	if err != nil || !authidentity.IsBoundedIdentifier(input.OfferID) {
		writeMemberResourceError(c, orgresource.ErrInvalidInput)
		return
	}
	if m.prices == nil {
		writeMemberResourceError(c, billing.ErrOfferUnavailable)
		return
	}
	offer, err := m.prices.ReadOffer(c.Request.Context(), input.OfferID)
	now := time.Now().UTC()
	if err != nil || offer.Validate() != nil || offer.Status != billing.OfferActive || offer.ProductKind != billing.ProductDataRow || offer.ResourceType != orgresource.ResourceDataRow || offer.UnitPriceMinor <= 0 || (offer.StartsAt != nil && now.Before(*offer.StartsAt)) || (offer.ExpiresAt != nil && !now.Before(*offer.ExpiresAt)) {
		writeMemberResourceError(c, billing.ErrOfferUnavailable)
		return
	}
	quantity := amount / offer.UnitPriceMinor
	if quantity < offer.MinQuantity || quantity > offer.MaxQuantity {
		writeMemberResourceError(c, orgresource.ErrInvalidInput)
		return
	}
	quote, err := m.prices.CreateQuote(c.Request.Context(), billing.QuoteRequest{OrganizationID: id.EffectiveOrganizationID, OfferID: offer.OfferID, Quantity: quantity})
	if err != nil || quote.Validate() != nil || quote.PricingVersion != offer.PricingVersion || quote.ProductKind != billing.ProductDataRow || quote.ResourceType != orgresource.ResourceDataRow || quote.Currency != offer.Currency || quote.ResourceQuantity != quantity || quote.TotalMinor != quantity*offer.UnitPriceMinor || quote.OrganizationID != id.EffectiveOrganizationID {
		writeMemberResourceError(c, billing.ErrQuoteExpired)
		return
	}
	writePointLimitJSON(c, 200, gin.H{"organizationId": id.EffectiveOrganizationID, "quoteId": quote.QuoteID, "quantity": strconv.FormatInt(quantity, 10), "unitPriceMinor": strconv.FormatInt(offer.UnitPriceMinor, 10), "pricingVersion": quote.PricingVersion, "currency": quote.Currency, "expiresAt": quote.ExpiresAt, "amountMinor": input.AmountMinor, "remainderMinor": strconv.FormatInt(amount-quote.TotalMinor, 10)})
}
