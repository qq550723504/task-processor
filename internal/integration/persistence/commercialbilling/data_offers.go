package commercialbilling

import (
	"context"
	billing "task-processor/internal/commercial/billing"
)

func (r *Repository) ListResourceOffers(ctx context.Context) ([]billing.Offer, error) {
	if r == nil || r.db == nil {
		return nil, billing.ErrFeatureUnavailable
	}
	var rows []offerRow
	if err := r.db.WithContext(ctx).Where("product_kind IN ? AND status = ?", []string{string(billing.ProductStoreRenewalPeriod), string(billing.ProductAIPoint), string(billing.ProductDataRow)}, string(billing.OfferActive)).Order("offer_id").Limit(101).Find(&rows).Error; err != nil || len(rows) > 100 {
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

func (r *Repository) ListDataRowOffers(ctx context.Context) ([]billing.Offer, error) {
	offers, err := r.ListResourceOffers(ctx)
	if err != nil {
		return nil, err
	}
	data := make([]billing.Offer, 0, len(offers))
	for _, offer := range offers {
		if offer.ProductKind == billing.ProductDataRow {
			data = append(data, offer)
		}
	}
	return data, nil
}

// ListActiveDataRowOfferIDs includes scheduled and expired rows that still have
// ACTIVE status. Schema initialization must not create a second active-status
// price just because an existing offer is outside its current sale window.
func (r *Repository) ListActiveDataRowOfferIDs(ctx context.Context) ([]string, error) {
	if r == nil || r.db == nil {
		return nil, billing.ErrFeatureUnavailable
	}
	var ids []string
	if err := r.db.WithContext(ctx).Model(&offerRow{}).Where("product_kind = ? AND status = ?", string(billing.ProductDataRow), string(billing.OfferActive)).Order("offer_id").Pluck("offer_id", &ids).Error; err != nil {
		return nil, billing.ErrFeatureUnavailable
	}
	return ids, nil
}
