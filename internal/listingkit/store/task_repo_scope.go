package store

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/listingkit"
	"task-processor/internal/shared/tenantctx"
)

func tenantScope(ctx context.Context, column string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return applyTenantScope(db, ctx, column)
	}
}

func taskAccessScope(ctx context.Context) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return applyTaskAccessScope(db, ctx)
	}
}

func applyTenantScope(db *gorm.DB, ctx context.Context, column string) *gorm.DB {
	tenantID, ok := tenantctx.TenantScopeFromContext(ctx)
	if !ok {
		return db
	}
	if tenantID == tenantctx.DefaultTenantID {
		return db.Where("("+column+" = ? OR "+column+" = '' OR "+column+" IS NULL)", tenantID)
	}
	return db.Where(column+" = ?", tenantID)
}

func applyTaskAccessScope(db *gorm.DB, ctx context.Context) *gorm.DB {
	db = applyTenantScope(db, ctx, "tenant_id")
	if !listingkit.OwnerScopeEnabled() {
		return db
	}
	if listingkit.RequestHasPlatformAdminAccess(ctx) {
		return db
	}
	userID := strings.TrimSpace(listingkit.RequestUserIDFromContext(ctx))
	if userID == "" {
		return db
	}
	return applyTaskUserScope(db, userID)
}

func applyTaskUserScope(db *gorm.DB, userID string) *gorm.DB {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return db
	}

	switch db.Dialector.Name() {
	case "postgres":
		return db.Where(
			"(BTRIM(COALESCE(user_id, '')) = ? OR (BTRIM(COALESCE(user_id, '')) = '' AND request IS NOT NULL AND request::jsonb ->> 'user_id' = ?))",
			userID,
			userID,
		)
	case "sqlite":
		return db.Where(
			"(TRIM(COALESCE(user_id, '')) = ? OR (TRIM(COALESCE(user_id, '')) = '' AND json_extract(request, '$.user_id') = ?))",
			userID,
			userID,
		)
	default:
		return db.Where("user_id = ?", userID)
	}
}

func filterTasksForUser(ctx context.Context, tasks []listingkit.Task) []listingkit.Task {
	if !listingkit.OwnerScopeEnabled() {
		return tasks
	}
	userID := listingkit.RequestUserIDFromContext(ctx)
	if userID == "" {
		return tasks
	}
	filtered := make([]listingkit.Task, 0, len(tasks))
	for _, task := range tasks {
		if taskVisibleToUser(ctx, &task) {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func taskVisibleToUser(ctx context.Context, task *listingkit.Task) bool {
	if !listingkit.OwnerScopeEnabled() {
		return true
	}
	if listingkit.RequestHasPlatformAdminAccess(ctx) {
		return true
	}
	requestUserID := strings.TrimSpace(listingkit.RequestUserIDFromContext(ctx))
	if requestUserID == "" {
		return true
	}
	return strings.TrimSpace(listingkit.ResolveTaskUserID(task)) == requestUserID
}

func currentTimestampValue(db *gorm.DB) any {
	if db != nil && db.Dialector != nil && db.Dialector.Name() == "sqlite" {
		return time.Now().UTC()
	}
	return gorm.Expr("NOW()")
}
