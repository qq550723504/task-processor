package preparationpersistence

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"task-processor/internal/listing/preparation"
	"task-processor/internal/product/collection"
)

func (r *Repository) ReadRetainedSource(ctx context.Context, scope preparation.Scope, id string) (preparation.SourceItem, error) {
	if r == nil || r.db == nil || ctx == nil || scope.Validate() != nil || !collection.ValidID(id) {
		return preparation.SourceItem{}, preparation.ErrInvalid
	}
	var row sourceRow
	err := r.db.WithContext(ctx).Where("organization_id=? AND actor_id=? AND member_id=? AND id=?", scope.OrganizationID, scope.ActorID, scope.MemberID, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return preparation.SourceItem{}, preparation.ErrNotFound
	}
	if err != nil {
		return preparation.SourceItem{}, preparation.ErrUnavailable
	}
	return row.value(), nil
}
