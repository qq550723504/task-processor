package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type GormStudioBatchTaskLinkRepository struct {
	db *gorm.DB
}

func NewGormStudioBatchTaskLinkRepository(db *gorm.DB) *GormStudioBatchTaskLinkRepository {
	return &GormStudioBatchTaskLinkRepository{db: db}
}

func AutoMigrateStudioBatchTaskLinkRepository(db *gorm.DB) error {
	if err := db.AutoMigrate(&StudioBatchTaskLinkRecord{}); err != nil {
		return err
	}
	if err := ensureStudioBatchTaskLinkSourceColumn(db); err != nil {
		return err
	}
	if err := ensureStudioBatchTaskLinkImageStrategyColumn(db); err != nil {
		return err
	}
	if err := ensureStudioBatchTaskLinkProductImageUsageRouteColumn(db); err != nil {
		return err
	}
	if err := ensureStudioBatchTaskLinkProductImageUsageSettledColumn(db); err != nil {
		return err
	}
	if err := ensureStudioBatchTaskLinkPendingProductImageUsageReleaseClaimTokenColumn(db); err != nil {
		return err
	}
	return ensureStudioBatchTaskLinkTupleIndex(db)
}

func (r *GormStudioBatchTaskLinkRepository) GetStudioBatchTaskLinkByCandidateKey(ctx context.Context, candidateKey string) (*StudioBatchTaskLinkRecord, error) {
	var record StudioBatchTaskLinkRecord
	err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Where("candidate_key = ?", candidateKey).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *GormStudioBatchTaskLinkRepository) CreateStudioBatchTaskLink(ctx context.Context, link *StudioBatchTaskLinkRecord) error {
	if link == nil {
		return nil
	}

	row := *link
	applyStudioBatchTaskLinkCreateScope(ctx, &row)
	if strings.TrimSpace(row.ID) == "" {
		return fmt.Errorf("studio batch task link id is required")
	}
	err := r.db.WithContext(ctx).Create(&row).Error
	if !isStudioBatchTaskLinkMissingSourceColumnError(err) && !isStudioBatchTaskLinkMissingImageStrategyColumnError(err) && !isStudioBatchTaskLinkMissingClaimTokenColumnError(err) && !isStudioBatchTaskLinkMissingProductImageUsageRouteColumnError(err) && !isStudioBatchTaskLinkMissingProductImageUsageSettledColumnError(err) && !isStudioBatchTaskLinkMissingPendingProductImageUsageReleaseClaimTokenColumnError(err) {
		return err
	}
	if migrateErr := ensureStudioBatchTaskLinkClaimTokenColumn(r.db); migrateErr != nil {
		return migrateErr
	}
	if migrateErr := ensureStudioBatchTaskLinkSourceColumn(r.db); migrateErr != nil {
		return migrateErr
	}
	if migrateErr := ensureStudioBatchTaskLinkImageStrategyColumn(r.db); migrateErr != nil {
		return migrateErr
	}
	if migrateErr := ensureStudioBatchTaskLinkProductImageUsageRouteColumn(r.db); migrateErr != nil {
		return migrateErr
	}
	if migrateErr := ensureStudioBatchTaskLinkProductImageUsageSettledColumn(r.db); migrateErr != nil {
		return migrateErr
	}
	if migrateErr := ensureStudioBatchTaskLinkPendingProductImageUsageReleaseClaimTokenColumn(r.db); migrateErr != nil {
		return migrateErr
	}
	if migrateErr := ensureStudioBatchTaskLinkTupleIndex(r.db); migrateErr != nil {
		return migrateErr
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *GormStudioBatchTaskLinkRepository) UpdateStudioBatchTaskLink(ctx context.Context, link *StudioBatchTaskLinkRecord) error {
	if link == nil {
		return nil
	}
	for _, ensure := range []func(*gorm.DB) error{
		ensureStudioBatchTaskLinkClaimTokenColumn,
		ensureStudioBatchTaskLinkSourceColumn,
		ensureStudioBatchTaskLinkImageStrategyColumn,
		ensureStudioBatchTaskLinkProductImageUsageRouteColumn,
		ensureStudioBatchTaskLinkProductImageUsageSettledColumn,
		ensureStudioBatchTaskLinkPendingProductImageUsageReleaseClaimTokenColumn,
	} {
		if err := ensure(r.db); err != nil {
			return err
		}
	}

	update := func() *gorm.DB {
		return applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
			Model(&StudioBatchTaskLinkRecord{}).
			Where("id = ?", link.ID).
			Updates(map[string]any{
				"listingkit_task_id":          link.ListingKitTaskID,
				"claim_token":                 link.ClaimToken,
				"status":                      link.Status,
				"source":                      link.Source,
				"reason_code":                 link.ReasonCode,
				"message":                     link.Message,
				"product_image_usage_route":   link.ProductImageUsageRoute,
				"product_image_usage_settled": link.ProductImageUsageSettled,
				"pending_product_image_usage_release_claim_token": link.PendingProductImageUsageReleaseClaimToken,
				"updated_at": link.UpdatedAt,
			})
	}

	result := update()
	for attempts := 0; result.Error != nil && attempts < 3; attempts++ {
		switch {
		case isStudioBatchTaskLinkMissingClaimTokenColumnError(result.Error):
			if err := ensureStudioBatchTaskLinkClaimTokenColumn(r.db); err != nil {
				return err
			}
		case isStudioBatchTaskLinkMissingSourceColumnError(result.Error):
			if err := ensureStudioBatchTaskLinkSourceColumn(r.db); err != nil {
				return err
			}
		case isStudioBatchTaskLinkMissingProductImageUsageSettledColumnError(result.Error):
			if err := ensureStudioBatchTaskLinkProductImageUsageSettledColumn(r.db); err != nil {
				return err
			}
		case isStudioBatchTaskLinkMissingProductImageUsageRouteColumnError(result.Error):
			if err := ensureStudioBatchTaskLinkProductImageUsageRouteColumn(r.db); err != nil {
				return err
			}
		case isStudioBatchTaskLinkMissingPendingProductImageUsageReleaseClaimTokenColumnError(result.Error):
			if err := ensureStudioBatchTaskLinkPendingProductImageUsageReleaseClaimTokenColumn(r.db); err != nil {
				return err
			}
		default:
			return result.Error
		}
		result = update()
	}
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *GormStudioBatchTaskLinkRepository) ResolveStudioBatchProductImageUsageRoute(ctx context.Context, candidateKey string, claimToken string, route studioBatchProductImageUsageRoute, updatedAt time.Time) (studioBatchProductImageUsageRoute, bool, error) {
	if route != studioBatchProductImageUsageRouteLegacy && route != studioBatchProductImageUsageRouteLedger {
		return "", false, fmt.Errorf("unsupported studio batch product image usage route %q", route)
	}
	if err := ensureStudioBatchTaskLinkProductImageUsageRouteColumn(r.db); err != nil {
		return "", false, err
	}
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioBatchTaskLinkRecord{}).
		Where("candidate_key = ? AND status = ? AND claim_token = ? AND product_image_usage_route = ?", candidateKey, studioBatchTaskLinkStatusCreating, strings.TrimSpace(claimToken), studioBatchProductImageUsageRoutePending).
		Updates(map[string]any{"product_image_usage_route": route, "updated_at": updatedAt})
	if result.Error != nil {
		return "", false, result.Error
	}
	link, err := r.GetStudioBatchTaskLinkByCandidateKey(ctx, candidateKey)
	if err != nil {
		return "", false, err
	}
	return link.ProductImageUsageRoute, result.RowsAffected > 0, nil
}

func (r *GormStudioBatchTaskLinkRepository) ClaimStudioBatchProductImageUsageSettled(ctx context.Context, candidateKey string, updatedAt time.Time) (bool, error) {
	if err := ensureStudioBatchTaskLinkProductImageUsageSettledColumn(r.db); err != nil {
		return false, err
	}
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioBatchTaskLinkRecord{}).
		Where("candidate_key = ? AND product_image_usage_settled = ?", candidateKey, false).
		Updates(map[string]any{"product_image_usage_settled": true, "updated_at": updatedAt})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *GormStudioBatchTaskLinkRepository) ClaimStudioBatchTaskCandidateWithToken(ctx context.Context, candidateKey string, fromStatus string, toStatus string, claimToken string, updatedAt time.Time) (*StudioBatchTaskLinkRecord, bool, error) {
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioBatchTaskLinkRecord{}).
		Where("candidate_key = ? AND status = ?", candidateKey, fromStatus).
		Updates(map[string]any{
			"status":      toStatus,
			"claim_token": strings.TrimSpace(claimToken),
			"updated_at":  updatedAt,
		})
	if result.Error != nil {
		return nil, false, result.Error
	}
	link, err := r.GetStudioBatchTaskLinkByCandidateKey(ctx, candidateKey)
	if err != nil {
		return nil, false, err
	}
	return link, result.RowsAffected > 0, nil
}

func (r *GormStudioBatchTaskLinkRepository) ClaimStudioBatchTaskCandidateUpdatedAtWithToken(ctx context.Context, candidateKey string, fromStatus string, observedUpdatedAt time.Time, toStatus string, claimToken string, updatedAt time.Time) (*StudioBatchTaskLinkRecord, bool, error) {
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioBatchTaskLinkRecord{}).
		Where("candidate_key = ? AND status = ? AND updated_at = ?", candidateKey, fromStatus, observedUpdatedAt).
		Updates(map[string]any{
			"status":      toStatus,
			"claim_token": strings.TrimSpace(claimToken),
			"updated_at":  updatedAt,
		})
	if result.Error != nil {
		return nil, false, result.Error
	}
	link, err := r.GetStudioBatchTaskLinkByCandidateKey(ctx, candidateKey)
	if err != nil {
		return nil, false, err
	}
	return link, result.RowsAffected > 0, nil
}

func (r *GormStudioBatchTaskLinkRepository) ClaimStudioBatchTaskCandidateUpdatedAtWithTokenAndPendingRelease(ctx context.Context, candidateKey string, fromStatus string, observedUpdatedAt time.Time, toStatus string, claimToken string, pendingReleaseClaimToken string, updatedAt time.Time) (*StudioBatchTaskLinkRecord, bool, error) {
	if err := ensureStudioBatchTaskLinkPendingProductImageUsageReleaseClaimTokenColumn(r.db); err != nil {
		return nil, false, err
	}
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioBatchTaskLinkRecord{}).
		Where("candidate_key = ? AND status = ? AND updated_at = ?", candidateKey, fromStatus, observedUpdatedAt).
		Updates(map[string]any{
			"status":      toStatus,
			"claim_token": strings.TrimSpace(claimToken),
			"pending_product_image_usage_release_claim_token": strings.TrimSpace(pendingReleaseClaimToken),
			"updated_at": updatedAt,
		})
	if result.Error != nil {
		return nil, false, result.Error
	}
	link, err := r.GetStudioBatchTaskLinkByCandidateKey(ctx, candidateKey)
	if err != nil {
		return nil, false, err
	}
	return link, result.RowsAffected > 0, nil
}

func (r *GormStudioBatchTaskLinkRepository) RefreshStudioBatchTaskLink(ctx context.Context, candidateKey string, claimToken string, updatedAt time.Time) (bool, error) {
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioBatchTaskLinkRecord{}).
		Where("candidate_key = ? AND status = ? AND claim_token = ? AND (listingkit_task_id IS NULL OR listingkit_task_id = '')", candidateKey, studioBatchTaskLinkStatusCreating, strings.TrimSpace(claimToken)).
		Updates(map[string]any{"updated_at": updatedAt})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *GormStudioBatchTaskLinkRepository) UpdateStudioBatchTaskLinkWithClaimToken(ctx context.Context, link *StudioBatchTaskLinkRecord, claimToken string) (bool, error) {
	if link == nil {
		return false, nil
	}
	for _, ensure := range []func(*gorm.DB) error{
		ensureStudioBatchTaskLinkClaimTokenColumn,
		ensureStudioBatchTaskLinkSourceColumn,
		ensureStudioBatchTaskLinkProductImageUsageRouteColumn,
		ensureStudioBatchTaskLinkProductImageUsageSettledColumn,
		ensureStudioBatchTaskLinkPendingProductImageUsageReleaseClaimTokenColumn,
	} {
		if err := ensure(r.db); err != nil {
			return false, err
		}
	}
	update := func() *gorm.DB {
		return applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
			Model(&StudioBatchTaskLinkRecord{}).
			Where("id = ? AND status = ? AND claim_token = ?", link.ID, studioBatchTaskLinkStatusCreating, strings.TrimSpace(claimToken)).
			Updates(map[string]any{
				"listingkit_task_id":          link.ListingKitTaskID,
				"claim_token":                 link.ClaimToken,
				"status":                      link.Status,
				"source":                      link.Source,
				"reason_code":                 link.ReasonCode,
				"message":                     link.Message,
				"product_image_usage_route":   link.ProductImageUsageRoute,
				"product_image_usage_settled": link.ProductImageUsageSettled,
				"pending_product_image_usage_release_claim_token": link.PendingProductImageUsageReleaseClaimToken,
				"updated_at": link.UpdatedAt,
			})
	}
	result := update()
	for attempts := 0; result.Error != nil && attempts < 3; attempts++ {
		switch {
		case isStudioBatchTaskLinkMissingClaimTokenColumnError(result.Error):
			if err := ensureStudioBatchTaskLinkClaimTokenColumn(r.db); err != nil {
				return false, err
			}
		case isStudioBatchTaskLinkMissingSourceColumnError(result.Error):
			if err := ensureStudioBatchTaskLinkSourceColumn(r.db); err != nil {
				return false, err
			}
		case isStudioBatchTaskLinkMissingProductImageUsageSettledColumnError(result.Error):
			if err := ensureStudioBatchTaskLinkProductImageUsageSettledColumn(r.db); err != nil {
				return false, err
			}
		case isStudioBatchTaskLinkMissingProductImageUsageRouteColumnError(result.Error):
			if err := ensureStudioBatchTaskLinkProductImageUsageRouteColumn(r.db); err != nil {
				return false, err
			}
		case isStudioBatchTaskLinkMissingPendingProductImageUsageReleaseClaimTokenColumnError(result.Error):
			if err := ensureStudioBatchTaskLinkPendingProductImageUsageReleaseClaimTokenColumn(r.db); err != nil {
				return false, err
			}
		default:
			return false, result.Error
		}
		result = update()
	}
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *GormStudioBatchTaskLinkRepository) ListStudioBatchTaskLinksByBatchID(ctx context.Context, batchID string) ([]StudioBatchTaskLinkRecord, error) {
	var links []StudioBatchTaskLinkRecord
	if err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Where("batch_id = ?", batchID).
		Order("id ASC").
		Find(&links).Error; err != nil {
		return nil, err
	}
	return links, nil
}

func (r *GormStudioBatchTaskLinkRepository) ClaimStudioBatchTaskCandidate(ctx context.Context, candidateKey string, fromStatus string, toStatus string, updatedAt time.Time) (*StudioBatchTaskLinkRecord, bool, error) {
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioBatchTaskLinkRecord{}).
		Where("candidate_key = ? AND status = ?", candidateKey, fromStatus).
		Updates(map[string]any{
			"status":     toStatus,
			"updated_at": updatedAt,
		})
	if result.Error != nil {
		return nil, false, result.Error
	}
	link, err := r.GetStudioBatchTaskLinkByCandidateKey(ctx, candidateKey)
	if err != nil {
		return nil, false, err
	}
	return link, result.RowsAffected > 0, nil
}

func (r *GormStudioBatchTaskLinkRepository) ClaimStudioBatchTaskCandidateUpdatedAt(ctx context.Context, candidateKey string, fromStatus string, observedUpdatedAt time.Time, toStatus string, updatedAt time.Time) (*StudioBatchTaskLinkRecord, bool, error) {
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioBatchTaskLinkRecord{}).
		Where("candidate_key = ? AND status = ? AND updated_at = ?", candidateKey, fromStatus, observedUpdatedAt).
		Updates(map[string]any{
			"status":     toStatus,
			"updated_at": updatedAt,
		})
	if result.Error != nil {
		return nil, false, result.Error
	}
	link, err := r.GetStudioBatchTaskLinkByCandidateKey(ctx, candidateKey)
	if err != nil {
		return nil, false, err
	}
	return link, result.RowsAffected > 0, nil
}

func ensureStudioBatchTaskLinkSourceColumn(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasColumn(&StudioBatchTaskLinkRecord{}, "Source") {
		if err := db.Migrator().AddColumn(&StudioBatchTaskLinkRecord{}, "Source"); err != nil {
			return err
		}
	}
	if !db.Migrator().HasIndex(&StudioBatchTaskLinkRecord{}, "Source") {
		if err := db.Migrator().CreateIndex(&StudioBatchTaskLinkRecord{}, "Source"); err != nil {
			return err
		}
	}
	return nil
}

func isStudioBatchTaskLinkMissingSourceColumnError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "source") &&
		(strings.Contains(message, "no column") ||
			strings.Contains(message, "does not exist") ||
			strings.Contains(message, "unknown column"))
}

func ensureStudioBatchTaskLinkTupleIndex(db *gorm.DB) error {
	const indexName = "idx_listingkit_studio_batch_task_links_tuple"
	if db == nil || (studioBatchTaskLinkTupleIndexIncludesCompatibilityFingerprint(db, indexName) &&
		studioBatchTaskLinkTupleIndexIncludesImageStrategy(db, indexName)) {
		return nil
	}
	if db.Migrator().HasIndex(&StudioBatchTaskLinkRecord{}, indexName) {
		if err := db.Migrator().DropIndex(&StudioBatchTaskLinkRecord{}, indexName); err != nil {
			return err
		}
	}
	return db.Migrator().CreateIndex(&StudioBatchTaskLinkRecord{}, indexName)
}

func ensureStudioBatchTaskLinkImageStrategyColumn(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasColumn(&StudioBatchTaskLinkRecord{}, "ImageStrategy") {
		return db.Migrator().AddColumn(&StudioBatchTaskLinkRecord{}, "ImageStrategy")
	}
	return nil
}

func isStudioBatchTaskLinkMissingImageStrategyColumnError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "image_strategy") &&
		(strings.Contains(message, "no column") ||
			strings.Contains(message, "does not exist") ||
			strings.Contains(message, "unknown column"))
}

func ensureStudioBatchTaskLinkClaimTokenColumn(db *gorm.DB) error {
	if db == nil || db.Migrator().HasColumn(&StudioBatchTaskLinkRecord{}, "ClaimToken") {
		return nil
	}
	return db.Migrator().AddColumn(&StudioBatchTaskLinkRecord{}, "ClaimToken")
}

func isStudioBatchTaskLinkMissingClaimTokenColumnError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "claim_token") &&
		(strings.Contains(message, "no column") ||
			strings.Contains(message, "no such column") ||
			strings.Contains(message, "does not exist") ||
			strings.Contains(message, "unknown column"))
}

func ensureStudioBatchTaskLinkProductImageUsageSettledColumn(db *gorm.DB) error {
	if db == nil || db.Migrator().HasColumn(&StudioBatchTaskLinkRecord{}, "ProductImageUsageSettled") {
		return nil
	}
	return db.Migrator().AddColumn(&StudioBatchTaskLinkRecord{}, "ProductImageUsageSettled")
}

func ensureStudioBatchTaskLinkProductImageUsageRouteColumn(db *gorm.DB) error {
	if db == nil || db.Migrator().HasColumn(&StudioBatchTaskLinkRecord{}, "ProductImageUsageRoute") {
		return nil
	}
	return db.Migrator().AddColumn(&StudioBatchTaskLinkRecord{}, "ProductImageUsageRoute")
}

func isStudioBatchTaskLinkMissingProductImageUsageRouteColumnError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "product_image_usage_route") &&
		(strings.Contains(message, "no column") || strings.Contains(message, "no such column") || strings.Contains(message, "does not exist") || strings.Contains(message, "unknown column"))
}

func isStudioBatchTaskLinkMissingProductImageUsageSettledColumnError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "product_image_usage_settled") &&
		(strings.Contains(message, "no column") || strings.Contains(message, "no such column") || strings.Contains(message, "does not exist") || strings.Contains(message, "unknown column"))
}

func ensureStudioBatchTaskLinkPendingProductImageUsageReleaseClaimTokenColumn(db *gorm.DB) error {
	if db == nil || db.Migrator().HasColumn(&StudioBatchTaskLinkRecord{}, "PendingProductImageUsageReleaseClaimToken") {
		return nil
	}
	return db.Migrator().AddColumn(&StudioBatchTaskLinkRecord{}, "PendingProductImageUsageReleaseClaimToken")
}

func isStudioBatchTaskLinkMissingPendingProductImageUsageReleaseClaimTokenColumnError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "pending_product_image_usage_release_claim_token") &&
		(strings.Contains(message, "no column") || strings.Contains(message, "no such column") || strings.Contains(message, "does not exist") || strings.Contains(message, "unknown column"))
}

func studioBatchTaskLinkTupleIndexIncludesImageStrategy(db *gorm.DB, indexName string) bool {
	switch db.Dialector.Name() {
	case "postgres":
		var indexDef string
		if err := db.Raw(`SELECT indexdef FROM pg_indexes WHERE tablename = ? AND indexname = ?`, StudioBatchTaskLinkRecord{}.TableName(), indexName).Scan(&indexDef).Error; err != nil {
			return false
		}
		return strings.Contains(indexDef, "image_strategy")
	case "sqlite":
		rows, err := db.Raw(`PRAGMA index_info(idx_listingkit_studio_batch_task_links_tuple)`).Rows()
		if err != nil {
			return false
		}
		defer rows.Close()
		for rows.Next() {
			var seqno int
			var cid int
			var name string
			if err := rows.Scan(&seqno, &cid, &name); err != nil {
				return false
			}
			if name == "image_strategy" {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func studioBatchTaskLinkTupleIndexIncludesCompatibilityFingerprint(db *gorm.DB, indexName string) bool {
	switch db.Dialector.Name() {
	case "postgres":
		var indexDef string
		if err := db.Raw(`SELECT indexdef FROM pg_indexes WHERE tablename = ? AND indexname = ?`, StudioBatchTaskLinkRecord{}.TableName(), indexName).Scan(&indexDef).Error; err != nil {
			return false
		}
		return strings.Contains(indexDef, "compatibility_fingerprint")
	case "sqlite":
		rows, err := db.Raw(`PRAGMA index_info(idx_listingkit_studio_batch_task_links_tuple)`).Rows()
		if err != nil {
			return false
		}
		defer rows.Close()
		for rows.Next() {
			var seqno int
			var cid int
			var name string
			if err := rows.Scan(&seqno, &cid, &name); err != nil {
				return false
			}
			if name == "compatibility_fingerprint" {
				return true
			}
		}
		return false
	default:
		return true
	}
}
