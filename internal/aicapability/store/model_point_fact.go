package store

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"strings"
	"task-processor/internal/aicapability"
)

// ReadModelInvocation reads either admitted priced-text operation from the
// existing AI invocation owner, never caller-supplied usage.
func (r *GormInvocationRecorder) ReadModelInvocation(ctx context.Context, org, id string) (aicapability.InvocationRecord, error) {
	if r == nil || r.db == nil || strings.TrimSpace(org) == "" || strings.TrimSpace(id) == "" {
		return aicapability.InvocationRecord{}, fmt.Errorf("invalid model fact identity")
	}
	var row invocationRow
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND invocation_id = ? AND operation IN ?", org, id,
		[]aicapability.Operation{aicapability.OperationProductAgentDecision, aicapability.OperationAIWorkbenchChatPlan}).Take(&row).Error; err != nil {
		return aicapability.InvocationRecord{}, err
	}
	fact := invocationRecordFromRow(row)
	if !fact.PointTariff.Valid() || fact.MaximumPromptTokens <= 0 || fact.MaximumCompletionTokens <= 0 || fact.StartedAt.IsZero() || fact.UserID == "" || fact.MemberID == "" || fact.InputHash == "" || !validPricedTextOperationIdentity(fact) {
		return aicapability.InvocationRecord{}, fmt.Errorf("invalid priced model fact")
	}
	if err := validateUsage(fact); err != nil {
		return aicapability.InvocationRecord{}, err
	}
	return fact, nil
}

// Startup verifies a freshly installed schema. It never alters an old ledger.
func VerifyModelPointInvocationSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("model invocation database is required")
	}
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(&invocationRow{}); err != nil {
		return err
	}
	columns, err := db.WithContext(ctx).Migrator().ColumnTypes(&invocationRow{})
	if err != nil {
		return err
	}
	present := make(map[string]bool, len(columns))
	for _, column := range columns {
		present[column.Name()] = true
	}
	for _, name := range stmt.Schema.DBNames {
		if !present[name] {
			return fmt.Errorf("current model invocation schema is missing %s; install on a fresh database", name)
		}
	}
	return nil
}
