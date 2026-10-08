package commercialbilling

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"task-processor/internal/commercial/billing"
)

// InstallInitialRetailOffers only initializes a fresh or incomplete approved
// catalog. It does not upgrade existing owner-managed prices or reactivate them.
func (r *Repository) InstallInitialRetailOffers(ctx context.Context) error {
	if r == nil || r.db == nil {
		return billing.ErrFeatureUnavailable
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize catalog mutation during a PostgreSQL first installation.
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("LOCK TABLE commercial_offers IN SHARE ROW EXCLUSIVE MODE").Error; err != nil {
				return err
			}
		}
		offers := billing.InitialRetailOffers()
		for _, offer := range offers {
			var ids []string
			if err := tx.Model(&offerRow{}).Where("product_kind = ? AND status = ?", string(offer.ProductKind), string(billing.OfferActive)).Pluck("offer_id", &ids).Error; err != nil {
				return err
			}
			for _, id := range ids {
				if id != offer.OfferID {
					return fmt.Errorf("existing %s offer %q requires an explicit price decision", offer.ProductKind, id)
				}
			}
		}
		store := &Repository{db: tx, now: r.now}
		for _, offer := range offers {
			if err := store.CreateOfferIfAbsent(ctx, offer); err != nil {
				return err
			}
		}
		return nil
	})
}
