package listingadmin

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

func (q *ScheduledTaskConfigQuery) applyListScope(scope listQueryScope) {
	q.TenantID = scope.TenantID
	q.OwnerUserID = scope.OwnerUserID
	q.Page = scope.Page
	q.PageSize = scope.PageSize
}

func findScheduledTaskConfigRows(ctx context.Context, db *gorm.DB, query ScheduledTaskConfigQuery) ([]listingScheduledTaskConfig, int64, int, int, error) {
	scopeQuery := query
	if strings.TrimSpace(scopeQuery.OwnerUserID) == "" {
		scopeQuery.OwnerUserID = requestUserIDFromContext(ctx)
	}
	var rows []listingScheduledTaskConfig
	total, page, pageSize, err := findPagedRows(applyScheduledTaskConfigQuery(db, scopeQuery), scopeQuery.Page, scopeQuery.PageSize, &rows)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return rows, total, page, pageSize, nil
}

func applyScheduledTaskConfigQuery(db *gorm.DB, query ScheduledTaskConfigQuery) *gorm.DB {
	db = applyOwnedTenantQuery(db, query.TenantID, strings.TrimSpace(query.OwnerUserID))
	if query.StoreID != nil {
		db = db.Where("store_id = ?", *query.StoreID)
	}
	if platform := normalizeScheduledTaskPlatform(query.Platform); platform != "" {
		db = db.Where("platform = ?", platform)
	}
	if taskType := normalizeScheduledTaskType(query.TaskType); taskType != "" {
		db = db.Where("task_type = ?", taskType)
	}
	if query.Enabled != nil {
		db = db.Where("enabled = ?", boolToInt16(*query.Enabled))
	}
	return db
}
