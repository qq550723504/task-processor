package billing

import "task-processor/internal/ledger/orgresource"

// InitialRetailOffers contains approved first-install inputs, never a runtime
// price fallback. All quotes and price displays read the catalog owner.
func InitialRetailOffers() []Offer {
	return []Offer{
		{OfferID: "store-service-30d-v1", ProductKind: ProductStoreRenewalPeriod, ResourceType: orgresource.ResourceStoreRenewalPeriod, Currency: CurrencyCNY, UnitPriceMinor: 16800, PricingVersion: "store-30d-168yuan-v1", MinQuantity: 1, MaxQuantity: 120, Status: OfferActive},
		{OfferID: "ai-point-v1", ProductKind: ProductAIPoint, ResourceType: orgresource.ResourceAIPoint, Currency: CurrencyCNY, UnitPriceMinor: 1, PricingVersion: "ai-point-1fen-v1", MinQuantity: 1, MaxQuantity: 1000000, Status: OfferActive},
		{OfferID: "data-row-1688-server-v1", ProductKind: ProductDataRow, ResourceType: orgresource.ResourceDataRow, Currency: CurrencyCNY, UnitPriceMinor: 5, PricingVersion: "1688-server-5fen-v1", MinQuantity: 1, MaxQuantity: 100000, Status: OfferActive},
	}
}
