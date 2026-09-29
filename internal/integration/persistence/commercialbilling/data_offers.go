package commercialbilling

import (
	"context"
	billing "task-processor/internal/commercial/billing"
)

func (r *Repository) ListDataRowOffers(ctx context.Context) ([]billing.Offer, error) {
	var rows []offerRow
	if err := r.db.WithContext(ctx).Where("product_kind = ? AND status = ?", string(billing.ProductDataRow), string(billing.OfferActive)).Order("offer_id").Limit(101).Find(&rows).Error; err != nil || len(rows) > 100 {
		return nil, billing.ErrFeatureUnavailable
	}
	now := r.now().UTC()
	result := make([]billing.Offer, 0, len(rows))
	for _, row := range rows {
		offer := offerFromRow(row)
		if offer.Validate() != nil || offer.UnitPriceMinor <= 0 || (offer.StartsAt != nil && now.Before(*offer.StartsAt)) || (offer.ExpiresAt != nil && !now.Before(*offer.ExpiresAt)) {
			continue
		}
		result = append(result, offer)
	}
	return result, nil
}
