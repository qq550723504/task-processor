package operationscockpitpersistence

import (
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	c "task-processor/internal/operationscockpit"
)

var _ c.Repository = (*Store)(nil)

func (s *Store) readAccess(ctx context.Context, scope c.Scope, module string, ids []string) error {
	if !scope.Valid() {
		return c.ErrForbidden
	}
	access, err := s.boundary.Current(ctx, scope)
	if err != nil {
		return err
	}
	if !access.Allows(module) {
		return c.ErrForbidden
	}
	return s.boundary.ReadStores(ctx, scope, ids)
}

const factColumns = "organization_id,record_id,store_id,revision,start_date::text AS start_date,end_date::text AS end_date,amounts,note,updated_by,updated_at"

func recordView(row factRow) (c.Record, error) {
	var amounts c.Amounts
	period := c.Period{Start: row.StartDate, End: row.EndDate}
	if !c.UUID(row.RecordID) || !c.UUID(row.StoreID) || row.Revision <= 0 || !period.Valid() || json.Unmarshal(row.Amounts, &amounts) != nil || !amounts.Valid() {
		return c.Record{}, c.ErrUnavailable
	}
	return c.Record{ID: row.RecordID, StoreID: row.StoreID, Revision: row.Revision, Period: period, Amounts: amounts, Note: row.Note, UpdatedBy: row.UpdatedBy, UpdatedAt: row.UpdatedAt}, nil
}
func (s *Store) Fact(ctx context.Context, scope c.Scope, id string) (c.Record, error) {
	if !c.UUID(id) {
		return c.Record{}, c.ErrInvalid
	}
	if err := s.readAccess(ctx, scope, "stores", nil); err != nil {
		return c.Record{}, err
	}
	var row factRow
	err := s.db.WithContext(ctx).Select(factColumns).Where("organization_id=? AND record_id=?", scope.OrganizationID, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c.Record{}, c.ErrNotFound
	}
	if err != nil {
		return c.Record{}, err
	}
	result, err := recordView(row)
	if err != nil {
		return c.Record{}, err
	}
	if err := s.readAccess(ctx, scope, "stores", []string{result.StoreID}); err != nil {
		return c.Record{}, err
	}
	return result, nil
}
func (s *Store) Facts(ctx context.Context, scope c.Scope, store string, page int) ([]c.Record, error) {
	if !c.UUID(store) || page < 1 || page > 1000 {
		return nil, c.ErrInvalid
	}
	if err := s.readAccess(ctx, scope, "stores", []string{store}); err != nil {
		return nil, err
	}
	var rows []factRow
	err := s.db.WithContext(ctx).Select(factColumns).Where("organization_id=? AND store_id=?", scope.OrganizationID, store).Order("start_date DESC,record_id").Offset((page - 1) * 50).Limit(50).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make([]c.Record, 0, len(rows))
	for _, row := range rows {
		record, err := recordView(row)
		if err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	if err := s.readAccess(ctx, scope, "stores", []string{store}); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *Store) FactHistory(ctx context.Context, scope c.Scope, id string, before int64) ([]c.Record, error) {
	if before < 0 {
		return nil, c.ErrInvalid
	}
	current, err := s.Fact(ctx, scope, id)
	if err != nil {
		return nil, err
	}
	query := s.db.WithContext(ctx).Table("operations_cockpit.fact_versions").Select(factColumns).Where("organization_id=? AND record_id=? AND revision<=?", scope.OrganizationID, id, current.Revision)
	if before > 0 {
		query = query.Where("revision<?", before)
	}
	var rows []factRow
	if err := query.Order("revision DESC").Limit(20).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]c.Record, 0, len(rows))
	for _, row := range rows {
		view, err := recordView(row)
		if err != nil || view.StoreID != current.StoreID {
			return nil, c.ErrUnavailable
		}
		result = append(result, view)
	}
	if err := s.readAccess(ctx, scope, "stores", []string{current.StoreID}); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *Store) GoalHistory(ctx context.Context, scope c.Scope, before int64) ([]c.GoalVersion, error) {
	if before < 0 {
		return nil, c.ErrInvalid
	}
	current, err := s.Goal(ctx, scope)
	if err != nil {
		return nil, err
	}
	query := s.db.WithContext(ctx).Where("organization_id=? AND goal_id=? AND revision<=?", scope.OrganizationID, current.ID, current.Revision)
	if before > 0 {
		query = query.Where("revision<?", before)
	}
	var rows []goalVersionRow
	if err := query.Order("revision DESC").Limit(20).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]c.GoalVersion, 0, len(rows))
	seen := map[string]bool{}
	ids := []string{}
	for _, row := range rows {
		view, err := (goalRow{GoalID: row.GoalID, CreatorID: current.CreatorID, Revision: row.Revision, Config: row.Config, UpdatedBy: row.UpdatedBy, UpdatedAt: row.UpdatedAt}).view()
		if err != nil {
			return nil, err
		}
		for _, id := range view.Config.StoreIDs {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
		result = append(result, view)
	}
	if err := s.readAccess(ctx, scope, "goals", ids); err != nil {
		return nil, err
	}
	return result, nil
}
