package aiworkbenchpersistence

import (
	"context"
	"errors"
	"math"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/aiworkbench"
)

type taskActionRow struct {
	OrganizationID, ActorID, IdempotencyKey, TaskID string
	Action                                          string
	ExpectedRevision                                int64
	RequestFingerprint                              string
	State                                           string
	ErrorCode                                       *string
	CreatedAt                                       time.Time
	FinishedAt                                      *time.Time
}

func (taskActionRow) TableName() string { return "ai_workbench.task_action_receipts" }

func validTaskActionInput(input aiworkbench.TaskActionInput) bool {
	if !validKey(input.TaskID) || !validKey(input.Key) || !utf8.ValidString(input.Feedback) || len(input.Feedback) > 8<<10 || input.Revision > math.MaxInt64 {
		return false
	}
	switch input.Action {
	case aiworkbench.TaskActionStart, aiworkbench.TaskActionReview:
		return input.Revision == 0 && input.Feedback == ""
	case aiworkbench.TaskActionResume:
		return input.Revision > 0
	default:
		return false
	}
}

func actionFingerprint(input aiworkbench.TaskActionInput) string {
	return digest(struct {
		TaskID   string
		Action   aiworkbench.TaskActionKind
		Revision uint64
		Feedback string
	}{input.TaskID, input.Action, input.Revision, input.Feedback})
}

func actionReceipt(row taskActionRow) (aiworkbench.TaskActionReceipt, error) {
	code := ""
	if row.ErrorCode != nil {
		code = *row.ErrorCode
	}
	if !validDigest(row.RequestFingerprint) || row.ExpectedRevision < 0 ||
		(row.State == string(aiworkbench.TaskActionClaimed)) != (row.FinishedAt == nil) ||
		(row.State != string(aiworkbench.TaskActionClaimed) && row.State != string(aiworkbench.TaskActionComplete) &&
			row.State != string(aiworkbench.TaskActionFailed) && row.State != string(aiworkbench.TaskActionUnknown)) ||
		(row.State == string(aiworkbench.TaskActionFailed)) != (code != "") {
		return aiworkbench.TaskActionReceipt{}, aiworkbench.ErrUnavailable
	}
	return aiworkbench.TaskActionReceipt{Scope: aiworkbench.Scope{OrganizationID: row.OrganizationID, ActorID: row.ActorID},
		TaskID: row.TaskID, Key: row.IdempotencyKey, Action: aiworkbench.TaskActionKind(row.Action),
		Revision: uint64(row.ExpectedRevision), Fingerprint: row.RequestFingerprint,
		State: aiworkbench.TaskActionState(row.State), ErrorCode: code, CreatedAt: row.CreatedAt.UTC(), FinishedAt: row.FinishedAt}, nil
}

// BeginTaskAction serializes one scoped key before calling an external owner.
// The receipt is never used as the Agent or Review state source.
func (s *Store) BeginTaskAction(ctx context.Context, scope aiworkbench.Scope, input aiworkbench.TaskActionInput) (aiworkbench.TaskActionReceipt, bool, error) {
	if s == nil || s.db == nil || ctx == nil || !validScope(scope) || !validTaskActionInput(input) {
		return aiworkbench.TaskActionReceipt{}, false, aiworkbench.ErrInvalid
	}
	if _, err := s.GetTask(ctx, scope, input.TaskID); err != nil {
		return aiworkbench.TaskActionReceipt{}, false, err
	}
	fingerprint := actionFingerprint(input)
	var result aiworkbench.TaskActionReceipt
	var replay bool
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		token := "ai-workbench-task-action:" + scope.OrganizationID + "\x1f" + scope.ActorID + "\x1f" + input.Key
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", token).Error; err != nil {
			return err
		}
		var row taskActionRow
		err := tx.Where("organization_id = ? AND actor_id = ? AND idempotency_key = ?", scope.OrganizationID, scope.ActorID, input.Key).Take(&row).Error
		if err == nil {
			if row.TaskID != input.TaskID || row.Action != string(input.Action) || row.ExpectedRevision != int64(input.Revision) || row.RequestFingerprint != fingerprint {
				return aiworkbench.ErrIdempotencyConflict
			}
			result, err = actionReceipt(row)
			replay = true
			return err
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		row = taskActionRow{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID,
			IdempotencyKey: input.Key, TaskID: input.TaskID, Action: string(input.Action),
			ExpectedRevision: int64(input.Revision), RequestFingerprint: fingerprint,
			State: string(aiworkbench.TaskActionClaimed), CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		result, err = actionReceipt(row)
		return err
	})
	return result, replay, err
}

func (s *Store) FinishTaskAction(ctx context.Context, scope aiworkbench.Scope, key string, state aiworkbench.TaskActionState, code ...string) error {
	if s == nil || s.db == nil || ctx == nil || !validScope(scope) || !validKey(key) ||
		(state != aiworkbench.TaskActionComplete && state != aiworkbench.TaskActionFailed && state != aiworkbench.TaskActionUnknown) ||
		(state == aiworkbench.TaskActionFailed && (len(code) != 1 || code[0] != "REVISION_MISMATCH" && code[0] != "DEPENDENCY_UNAVAILABLE")) ||
		(state != aiworkbench.TaskActionFailed && len(code) != 0) {
		return aiworkbench.ErrInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row taskActionRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND actor_id = ? AND idempotency_key = ?",
			scope.OrganizationID, scope.ActorID, key).Take(&row).Error; err != nil {
			return missing(err)
		}
		if row.State == string(state) && (state != aiworkbench.TaskActionFailed || row.ErrorCode != nil && *row.ErrorCode == code[0]) {
			return nil
		}
		if row.State != string(aiworkbench.TaskActionClaimed) || row.FinishedAt != nil {
			return aiworkbench.ErrIdempotencyConflict
		}
		finished := time.Now().UTC()
		var errorCode any
		if state == aiworkbench.TaskActionFailed {
			errorCode = code[0]
		}
		updated := tx.Model(&taskActionRow{}).Where("organization_id = ? AND actor_id = ? AND idempotency_key = ? AND state = ?",
			scope.OrganizationID, scope.ActorID, key, aiworkbench.TaskActionClaimed).
			Updates(map[string]any{"state": string(state), "error_code": errorCode, "finished_at": finished})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return aiworkbench.ErrIdempotencyConflict
		}
		return nil
	})
}
