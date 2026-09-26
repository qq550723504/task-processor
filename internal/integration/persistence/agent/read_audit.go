package agentpersistence

import (
	"bytes"
	"context"
	"encoding/json"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"task-processor/internal/agent"
	"task-processor/internal/commercetool"
)

// Read is actor- and acquisition-scoped. Authorization belongs to the caller;
// possession of a run/request ID never broadens this lookup.
func (s *Store) Read(ctx context.Context, scope agent.Scope, contextID, key string) (agent.Record, error) {
	if s == nil || s.db == nil || !agent.ValidID(scope.OrganizationID) || !agent.ValidID(scope.ActorID) || !agent.ValidID(contextID) || !agent.ValidID(key) {
		return agent.Record{}, agent.ErrInvalid
	}
	var row runRow
	if err := s.db.WithContext(ctx).Where("org = ? AND actor = ? AND context_kind = ? AND context_id = ? AND request_key = ?", scope.OrganizationID, scope.ActorID, "acquisition", contextID, key).Take(&row).Error; err != nil {
		return agent.Record{}, agent.ErrUnavailable
	}
	return decode(row)
}

type toolAuditRow struct {
	RunID, CallID string
	Payload       []byte
}

func (toolAuditRow) TableName() string { return "product_agent_tool_calls" }

// RecordToolCall persists the existing safe audit envelope with this run owner.
// It contains hashes/correlation, never tool arguments or raw source/model text.
func (s *Store) RecordToolCall(ctx context.Context, audit commercetool.AuditRecord) error {
	if s == nil || s.db == nil || !agent.ValidID(audit.CallID) || !agent.ValidID(audit.AgentRunID) || audit.StartedAt.IsZero() || audit.FinishedAt.IsZero() {
		return agent.ErrInvalid
	}
	raw, err := json.Marshal(audit)
	if err != nil || len(raw) > 8192 {
		return agent.ErrInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run runRow
		if err := tx.Where("run_id = ? AND org = ? AND actor = ? AND context_id = ?", audit.AgentRunID, audit.TenantID, audit.UserID, audit.BusinessTaskID).Take(&run).Error; err != nil {
			return agent.ErrConflict
		}
		row := toolAuditRow{RunID: audit.AgentRunID, CallID: audit.CallID, Payload: raw}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 1 {
			return nil
		}
		var existing toolAuditRow
		if err := tx.Where("run_id = ? AND call_id = ?", row.RunID, row.CallID).Take(&existing).Error; err != nil {
			return err
		}
		if !bytes.Equal(existing.Payload, raw) {
			return agent.ErrConflict
		}
		return nil
	})
}

var _ commercetool.AuditRecorder = (*Store)(nil)
