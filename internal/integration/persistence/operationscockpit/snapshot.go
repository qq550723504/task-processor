package operationscockpitpersistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"gorm.io/gorm"
	c "task-processor/internal/operationscockpit"
)

// Snapshot reads the goal head and all current fact revisions in one local
// repeatable-read snapshot. Permission rechecks run after that snapshot ends.
func (s *Store) Snapshot(ctx context.Context, scope c.Scope, query c.Query) (c.Snapshot, error) {
	result := c.Snapshot{CapturedAt: s.now(), Period: query.Period, Stores: map[string]c.StoreAggregate{}, Previous: map[string]c.StoreAggregate{}}
	result.Today = c.Today(result.CapturedAt)
	if !scope.Valid() {
		return result, c.ErrForbidden
	}
	if !query.Valid() {
		return result, c.ErrInvalid
	}
	initial, err := s.boundary.Current(ctx, scope)
	if err != nil {
		return result, err
	}
	if !initial.Allows(query.Module) {
		return result, c.ErrForbidden
	}
	if err := s.boundary.ReadStores(ctx, scope, query.StoreIDs); err != nil {
		return result, err
	}
	previous, _ := c.PreviousPeriod(query.Period)
	goalStores := map[string]c.StoreAggregate{}
	var head *goalRow
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		periods := []c.Period{query.Period, previous}
		ids := append([]string(nil), query.StoreIDs...)
		var goalPeriod c.Period
		var evaluate bool
		if initial.GoalsRead {
			var err error
			head, err = findGoal(tx, scope.OrganizationID, false)
			if err != nil {
				return err
			}
			if head != nil {
				view, err := head.view()
				if err != nil {
					return err
				}
				readErr := s.boundary.ReadStores(ctx, scope, view.Config.StoreIDs)
				valid := readErr == nil
				if readErr != nil && !errors.Is(readErr, c.ErrForbidden) && !errors.Is(readErr, c.ErrNotFound) {
					return readErr
				}
				if head.head().CanMaintain(scope, initial) {
					metadata, err := c.MaintenanceMetadata(head.head(), scope, initial, valid)
					if err != nil {
						return err
					}
					result.Head = &metadata
				}
				if valid {
					result.Goal = &view
					goalPeriod, evaluate = c.GoalThrough(view.Config, result.CapturedAt)
					if evaluate {
						periods = append(periods, goalPeriod)
						ids = append(ids, view.Config.StoreIDs...)
					}
				} else {
					result.GoalUnavailable = true
				}
			}
		}
		ids = uniqueSorted(ids)
		selected := map[string]bool{}
		for _, id := range query.StoreIDs {
			selected[id] = true
		}
		flush := func(id string, records []c.Record) error {
			if selected[id] {
				current, err := c.Aggregate(query.Period, records)
				if err != nil {
					return err
				}
				current.StoreID = id
				result.Stores[id] = current
				prior, err := c.Aggregate(previous, records)
				if err != nil {
					return err
				}
				prior.StoreID = id
				result.Previous[id] = prior
			}
			if evaluate && contains(result.Goal.Config.StoreIDs, id) {
				aggregate, err := c.Aggregate(goalPeriod, records)
				if err != nil {
					return err
				}
				aggregate.StoreID = id
				goalStores[id] = aggregate
			}
			return nil
		}
		if len(ids) == 0 {
			return nil
		}
		conditions := make([]string, 0, len(periods))
		args := []any{}
		for _, period := range periods {
			conditions = append(conditions, "(start_date<=? AND end_date>=?)")
			args = append(args, period.End, period.Start)
		}
		rows, err := tx.Model(&factRow{}).Select("record_id,store_id,revision,start_date::text,end_date::text,amounts").Where("organization_id=? AND store_id IN ?", scope.OrganizationID, ids).Where(strings.Join(conditions, " OR "), args...).Order("store_id,start_date,record_id").Rows()
		if err != nil {
			return err
		}
		defer rows.Close()
		var currentID string
		var records []c.Record
		seen := map[string]bool{}
		for rows.Next() {
			var record c.Record
			var amounts []byte
			if err := rows.Scan(&record.ID, &record.StoreID, &record.Revision, &record.Period.Start, &record.Period.End, &amounts); err != nil {
				return err
			}
			if json.Unmarshal(amounts, &record.Amounts) != nil || !record.Amounts.Valid() {
				return c.ErrUnavailable
			}
			if currentID != "" && currentID != record.StoreID {
				if err := flush(currentID, records); err != nil {
					return err
				}
				records = nil
			}
			currentID = record.StoreID
			seen[currentID] = true
			records = append(records, record)
			// Non-overlapping current facts can intersect each bounded period at
			// most 366 times, plus two records crossing its edges.
			if len(records) > len(periods)*368 {
				return c.ErrUnavailable
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if currentID != "" {
			if err := flush(currentID, records); err != nil {
				return err
			}
		}
		for _, id := range ids {
			if !seen[id] {
				if err := flush(id, nil); err != nil {
					return err
				}
			}
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return c.Snapshot{}, err
	}
	if result.Goal != nil {
		evaluation, err := c.EvaluateGoal(result.Goal.Config, result.CapturedAt, goalStores)
		if err != nil {
			return c.Snapshot{}, err
		}
		result.Evaluation = &evaluation
	}
	// Use current IAM and Store connections after releasing the old SQL snapshot.
	fresh, err := s.boundary.Current(ctx, scope)
	if err != nil {
		return c.Snapshot{}, err
	}
	if !fresh.Allows(query.Module) || initial.GoalsRead && !fresh.GoalsRead {
		return c.Snapshot{}, c.ErrForbidden
	}
	if err := s.boundary.ReadStores(ctx, scope, query.StoreIDs); err != nil {
		return c.Snapshot{}, err
	}
	if result.Goal != nil {
		if err := s.boundary.ReadStores(ctx, scope, result.Goal.Config.StoreIDs); err != nil {
			return c.Snapshot{}, err
		}
	}
	if result.Head != nil && (head == nil || !head.head().CanMaintain(scope, fresh)) {
		result.Head = nil
	}
	if ctx.Err() != nil {
		return c.Snapshot{}, ctx.Err()
	}
	return result, nil
}

func uniqueSorted(ids []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			result = append(result, id)
			seen[id] = true
		}
	}
	sort.Strings(result)
	return result
}
func contains(ids []string, id string) bool {
	for _, value := range ids {
		if id == value {
			return true
		}
	}
	return false
}
