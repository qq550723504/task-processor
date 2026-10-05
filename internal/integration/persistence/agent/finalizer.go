package agentpersistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"task-processor/internal/agent"
)

// FinalizeExpiredRunning closes only an expired durable RUNNING row. It never
// resumes a graph, invokes a model/tool, or grants another provider dispatch.
func (s *Store) FinalizeExpiredRunning(ctx context.Context, scope agent.Scope, binding agent.Binding, key string, observedRevision uint64, now time.Time) (agent.Record, error) {
	if s == nil || s.db == nil || ctx == nil || ctx.Err() != nil ||
		!agent.ValidID(scope.OrganizationID) || !agent.ValidID(scope.ActorID) ||
		!binding.Valid() || !agent.ValidID(key) || observedRevision == 0 || now.IsZero() {
		return agent.Record{}, agent.ErrInvalid
	}
	var result agent.Record
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row runRow
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"org = ? AND actor = ? AND context_kind = ? AND context_id = ? AND request_key = ?",
			scope.OrganizationID, scope.ActorID, binding.ContextKind, binding.ContextID, key).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return agent.ErrConflict
		}
		if err != nil {
			return err
		}
		stored, err := decode(row)
		if err != nil {
			return err
		}
		if stored.State.Scope != scope || stored.State.Request.Binding != binding || stored.State.Request.Key != key {
			return agent.ErrConflict
		}
		if stored.State.Phase != agent.Running {
			result = stored
			return nil
		}
		if stored.State.Revision != observedRevision || !now.After(stored.State.Deadline.Add(30*time.Second)) {
			return agent.ErrConflict
		}
		stored.State.Phase = agent.Stopped
		stored.State.StopReason = agent.StopExecutionOutcomeUnknown
		stored.State.Revision++
		stored.Checkpoint = nil
		updated, err := encode(stored)
		if err != nil {
			return err
		}
		write := tx.Model(&runRow{}).Where("run_id = ? AND revision = ? AND phase = ?",
			row.RunID, observedRevision, agent.Running).
			Updates(map[string]any{"revision": updated.Revision, "phase": updated.Phase, "payload": updated.Payload})
		if write.Error != nil {
			return write.Error
		}
		if write.RowsAffected != 1 {
			return agent.ErrConflict
		}
		result = stored
		return nil
	})
	if err != nil {
		return agent.Record{}, err
	}
	return result, nil
}
