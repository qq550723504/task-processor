package orgresourceadapter

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/ledger/orgresource"
)

type positiveCreditAllocation struct {
	gross          int64
	debtRepaid     int64
	net            int64
	availableAfter int64
}

// applyPositiveCredit is the single transaction-bound path for value flowing
// back into available. Callers must already hold the matching bucket lock.
func applyPositiveCredit(ctx context.Context, tx *gorm.DB, organizationID, resourceType string, gross, availableBefore int64, now time.Time) (positiveCreditAllocation, error) {
	debt := organizationResourceDebtRow{OrganizationID: organizationID, ResourceType: resourceType}
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&debt).Error; err != nil {
		return positiveCreditAllocation{}, err
	}
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("organization_id = ? AND resource_type = ?", organizationID, resourceType).
		Take(&debt).Error; err != nil {
		return positiveCreditAllocation{}, err
	}
	allocation := positiveCreditAllocation{gross: gross}
	allocation.debtRepaid = min(debt.Amount, gross)
	allocation.net = gross - allocation.debtRepaid
	allocation.availableAfter = availableBefore + allocation.net
	if allocation.availableAfter < availableBefore {
		return positiveCreditAllocation{}, fmt.Errorf("%w: available balance overflow", orgresource.ErrInvalidInput)
	}
	updated := tx.WithContext(ctx).Model(&organizationResourceDebtRow{}).
		Where("organization_id = ? AND resource_type = ?", organizationID, resourceType).
		Updates(map[string]any{"amount": debt.Amount - allocation.debtRepaid, "updated_at": now})
	if updated.Error != nil {
		return positiveCreditAllocation{}, updated.Error
	}
	if updated.RowsAffected != 1 {
		return positiveCreditAllocation{}, fmt.Errorf("resource credit lost the durable debt fence")
	}
	return allocation, nil
}
