package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"task-processor/internal/imageagent"
)

type gormRepository struct {
	db *gorm.DB
}

func NewGormRepository(db *gorm.DB) imageagent.Repository {
	return &gormRepository{db: db}
}

func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is nil")
	}
	return db.AutoMigrate(&runRecord{}, &planRecord{}, &slotRecord{}, &attemptRecord{}, &eventRecord{})
}

func (r *gormRepository) CreateRun(ctx context.Context, run *imageagent.Run) error {
	if err := validateRun(run); err != nil {
		return err
	}
	row, err := runToRecord(*run)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("create image agent run: %w", err)
	}
	return nil
}

func (r *gormRepository) GetRun(ctx context.Context, scope imageagent.RunScope) (*imageagent.Run, error) {
	if err := validateScope(scope); err != nil {
		return nil, err
	}
	row, err := r.findRun(ctx, r.db, scope)
	if err != nil {
		return nil, err
	}
	run, err := recordToRun(row)
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *gormRepository) UpdateRun(ctx context.Context, scope imageagent.RunScope, expectedVersion int64, mutation imageagent.RunMutation) error {
	if err := validateScope(scope); err != nil {
		return err
	}
	payload, err := json.Marshal(mutation)
	if err != nil {
		return fmt.Errorf("marshal run mutation event: %w", err)
	}
	blockJSON, err := marshalJSON(mutation.Block)
	if err != nil {
		return fmt.Errorf("marshal run block: %w", err)
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&runRecord{}).
			Where("tenant_id = ? AND id = ? AND version = ?", scope.TenantID, scope.RunID, expectedVersion).
			Updates(map[string]any{
				"status":               string(mutation.Status),
				"current_node":         mutation.CurrentNode,
				"active_plan_revision": mutation.ActivePlanRevision,
				"block_json":           blockJSON,
				"version":              expectedVersion + 1,
			})
		if result.Error != nil {
			return fmt.Errorf("update image agent run: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			if _, err := r.findRun(ctx, tx, scope); err != nil {
				return err
			}
			return imageagent.ErrRevisionConflict
		}

		cursor, err := nextEventCursor(ctx, tx, scope)
		if err != nil {
			return err
		}
		return tx.Create(&eventRecord{
			TenantID: scope.TenantID, RunID: scope.RunID, Type: "run.updated", Cursor: cursor,
			ProjectionVersion: expectedVersion + 1, Payload: payload,
		}).Error
	})
}

func (r *gormRepository) AppendPlan(ctx context.Context, scope imageagent.RunScope, expectedActiveRevision int64, plan imageagent.Plan) error {
	if err := validateScope(scope); err != nil {
		return err
	}
	if err := imageagent.ValidatePlan(plan); err != nil {
		return err
	}
	planRow, slotRows, err := planToRecords(scope, plan)
	if err != nil {
		return err
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&runRecord{}).
			Where("tenant_id = ? AND id = ? AND active_plan_revision = ?", scope.TenantID, scope.RunID, expectedActiveRevision).
			Update("active_plan_revision", plan.Revision)
		if result.Error != nil {
			return fmt.Errorf("advance image agent plan revision: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			if _, err := r.findRun(ctx, tx, scope); err != nil {
				return err
			}
			return imageagent.ErrRevisionConflict
		}
		if err := tx.Create(&planRow).Error; err != nil {
			return fmt.Errorf("append image agent plan: %w", err)
		}
		if len(slotRows) == 0 {
			return nil
		}
		if err := tx.Create(&slotRows).Error; err != nil {
			return fmt.Errorf("append image agent plan slots: %w", err)
		}
		return nil
	})
}

func (r *gormRepository) SaveSlotResult(ctx context.Context, scope imageagent.RunScope, expectedActiveRevision int64, result imageagent.SlotResult) error {
	if err := validateScope(scope); err != nil {
		return err
	}
	if strings.TrimSpace(result.SlotID) == "" {
		return fmt.Errorf("slot ID cannot be empty")
	}
	candidates, err := marshalJSON(result.CandidateAssetIDs)
	if err != nil {
		return fmt.Errorf("marshal slot candidate asset IDs: %w", err)
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		run, err := r.findRun(ctx, tx, scope)
		if err != nil {
			return err
		}
		if run.ActivePlanRevision != expectedActiveRevision {
			return imageagent.ErrRevisionConflict
		}
		update := tx.Model(&slotRecord{}).
			Where("tenant_id = ? AND run_id = ? AND plan_revision = ? AND id = ?", scope.TenantID, scope.RunID, expectedActiveRevision, result.SlotID).
			Updates(map[string]any{
				"attempt":             result.Attempt,
				"status":              string(result.Status),
				"candidate_asset_ids": candidates,
				"error_code":          result.ErrorCode,
			})
		if update.Error != nil {
			return fmt.Errorf("save image agent slot result: %w", update.Error)
		}
		if update.RowsAffected == 0 {
			return imageagent.ErrRevisionConflict
		}
		return nil
	})
}

func (r *gormRepository) AppendAttempt(ctx context.Context, attempt imageagent.StepAttempt) error {
	if err := validateAttempt(attempt); err != nil {
		return err
	}
	row := attemptRecord{
		TenantID: attempt.TenantID, RunID: attempt.RunID, SlotID: attempt.SlotID, Node: attempt.Node,
		IdempotencyKey: attempt.IdempotencyKey, Attempt: attempt.Attempt, Outcome: attempt.Outcome, ErrorCategory: attempt.ErrorCategory,
	}
	scope := imageagent.RunScope{TenantID: attempt.TenantID, RunID: attempt.RunID}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := r.findRun(ctx, tx, scope); err != nil {
			return err
		}
		var existing attemptRecord
		err := tx.Where("tenant_id = ? AND run_id = ? AND slot_id = ? AND idempotency_key = ?", row.TenantID, row.RunID, row.SlotID, row.IdempotencyKey).First(&existing).Error
		switch {
		case err == nil:
			if sameAttempt(existing, row) {
				return nil
			}
			return fmt.Errorf("attempt idempotency key already exists")
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return fmt.Errorf("lookup image agent attempt: %w", err)
		}
		if err := tx.Create(&row).Error; err != nil {
			return fmt.Errorf("append image agent attempt: %w", err)
		}
		return nil
	})
}

func (r *gormRepository) findRun(ctx context.Context, db *gorm.DB, scope imageagent.RunScope) (runRecord, error) {
	var row runRecord
	err := db.WithContext(ctx).Where("tenant_id = ? AND id = ?", scope.TenantID, scope.RunID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return runRecord{}, imageagent.ErrRunNotFound
	}
	if err != nil {
		return runRecord{}, fmt.Errorf("get image agent run: %w", err)
	}
	return row, nil
}

func runToRecord(run imageagent.Run) (runRecord, error) {
	budgetJSON, err := marshalJSON(run.Budget)
	if err != nil {
		return runRecord{}, fmt.Errorf("marshal run budget: %w", err)
	}
	usageJSON, err := marshalJSON(run.Usage)
	if err != nil {
		return runRecord{}, fmt.Errorf("marshal run usage: %w", err)
	}
	blockJSON, err := marshalJSON(run.Block)
	if err != nil {
		return runRecord{}, fmt.Errorf("marshal run block: %w", err)
	}
	return runRecord{
		TenantID: run.TenantID, ID: run.ID, BusinessTaskID: run.BusinessTaskID, UserID: run.UserID,
		Mode: string(run.Mode), IdempotencyKey: run.IdempotencyKey, Status: string(run.Status), CurrentNode: run.CurrentNode,
		ActivePlanRevision: run.ActivePlanRevision, Version: run.Version, BudgetJSON: budgetJSON, UsageJSON: usageJSON, BlockJSON: blockJSON,
	}, nil
}

func recordToRun(row runRecord) (imageagent.Run, error) {
	var budget imageagent.Budget
	if err := unmarshalJSON(row.BudgetJSON, &budget); err != nil {
		return imageagent.Run{}, fmt.Errorf("decode run budget: %w", err)
	}
	var usage imageagent.BudgetUsage
	if err := unmarshalJSON(row.UsageJSON, &usage); err != nil {
		return imageagent.Run{}, fmt.Errorf("decode run usage: %w", err)
	}
	var block *imageagent.Block
	if err := unmarshalJSON(row.BlockJSON, &block); err != nil {
		return imageagent.Run{}, fmt.Errorf("decode run block: %w", err)
	}
	return imageagent.Run{
		ID: row.ID, TenantID: row.TenantID, BusinessTaskID: row.BusinessTaskID, UserID: row.UserID, Mode: imageagent.RunMode(row.Mode),
		IdempotencyKey: row.IdempotencyKey, Status: imageagent.RunStatus(row.Status), CurrentNode: row.CurrentNode,
		ActivePlanRevision: row.ActivePlanRevision, Version: row.Version, Budget: budget, Usage: usage, Block: block,
	}, nil
}

func planToRecords(scope imageagent.RunScope, plan imageagent.Plan) (planRecord, []slotRecord, error) {
	sources, err := marshalJSON(plan.SourceAssetIDs)
	if err != nil {
		return planRecord{}, nil, err
	}
	styles, err := marshalJSON(plan.StyleReferenceIDs)
	if err != nil {
		return planRecord{}, nil, err
	}
	planRow := planRecord{TenantID: scope.TenantID, RunID: scope.RunID, Revision: plan.Revision, ParentRevision: plan.ParentRevision, IdempotencyKey: plan.IdempotencyKey, SourceAssetIDs: sources, StyleReferenceIDs: styles, CreatedBy: plan.CreatedBy}
	slots := make([]slotRecord, 0, len(plan.Slots))
	for _, slot := range plan.Slots {
		slotSources, err := marshalJSON(slot.SourceAssetIDs)
		if err != nil {
			return planRecord{}, nil, err
		}
		slotStyles, err := marshalJSON(slot.StyleReferenceIDs)
		if err != nil {
			return planRecord{}, nil, err
		}
		slots = append(slots, slotRecord{TenantID: scope.TenantID, RunID: scope.RunID, PlanRevision: plan.Revision, ID: slot.ID, Role: string(slot.Role), SourceAssetIDs: slotSources, StyleReferenceIDs: slotStyles, Brief: slot.Brief, IdempotencyKey: slot.IdempotencyKey, Status: string(slot.Status)})
	}
	return planRow, slots, nil
}

func marshalJSON(value any) ([]byte, error) { return json.Marshal(value) }

func unmarshalJSON(raw []byte, target any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, target)
}

func sameAttempt(left, right attemptRecord) bool {
	return left.TenantID == right.TenantID && left.RunID == right.RunID && left.SlotID == right.SlotID && left.Node == right.Node && left.IdempotencyKey == right.IdempotencyKey && left.Attempt == right.Attempt && left.Outcome == right.Outcome && left.ErrorCategory == right.ErrorCategory
}
