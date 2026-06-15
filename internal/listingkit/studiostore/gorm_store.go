package studiostore

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/listingkit"
	"task-processor/internal/shared/tenantctx"
)

func (r *GormRepository) FindLatestSessionBySelectionKey(ctx context.Context, selectionKey string) (*listingkit.SheinStudioSession, error) {
	var session listingkit.SheinStudioSession
	err := applySessionAccessScope(r.db.WithContext(ctx), ctx, "tenant_id", "user_id").
		Where("selection_key = ?", selectionKey).
		Order("updated_at DESC").
		First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *GormRepository) CreateSession(ctx context.Context, session *listingkit.SheinStudioSession) error {
	if session != nil && session.TenantID == "" {
		session.TenantID = tenantctx.TenantIDFromContext(ctx)
	}
	if session != nil && session.UserID == "" {
		session.UserID = listingkit.RequestUserIDFromContext(ctx)
	}
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *GormRepository) GetSession(ctx context.Context, sessionID string) (*listingkit.SheinStudioSession, error) {
	var session listingkit.SheinStudioSession
	err := applySessionAccessScope(r.db.WithContext(ctx), ctx, "tenant_id", "user_id").Where("id = ?", sessionID).First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *GormRepository) UpdateSession(ctx context.Context, session *listingkit.SheinStudioSession) error {
	if session != nil && session.TenantID == "" {
		session.TenantID = tenantctx.TenantIDFromContext(ctx)
	}
	if session != nil && session.UserID == "" {
		session.UserID = listingkit.RequestUserIDFromContext(ctx)
	}
	return applySessionAccessScope(r.db.WithContext(ctx), ctx, "tenant_id", "user_id").Save(session).Error
}

func (r *GormRepository) DeleteSession(ctx context.Context, sessionID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := applySessionAccessScope(tx, ctx, "tenant_id", "").Where("session_id = ?", sessionID).Delete(&listingkit.SheinStudioDesign{}).Error; err != nil {
			return err
		}
		return applySessionAccessScope(tx, ctx, "tenant_id", "user_id").Where("id = ?", sessionID).Delete(&listingkit.SheinStudioSession{}).Error
	})
}

func (r *GormRepository) ReplaceDesigns(ctx context.Context, sessionID string, approvedIDs []string, designs []listingkit.SheinStudioDesign) error {
	tenantID := tenantctx.TenantIDFromContext(ctx)
	designs = normalizeStudioDesignBatch(sessionID, tenantID, designs)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(designs) > 0 {
			designIDs := make([]string, 0, len(designs))
			for _, design := range designs {
				designIDs = append(designIDs, design.ID)
			}
			if err := applySessionAccessScope(tx, ctx, "tenant_id", "").
				Where("session_id = ? AND id NOT IN ?", sessionID, designIDs).
				Delete(&listingkit.SheinStudioDesign{}).Error; err != nil {
				return err
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "id"}},
				DoUpdates: clause.Assignments(map[string]any{
					"tenant_id":              clause.Expr{SQL: "excluded.tenant_id"},
					"session_id":             clause.Expr{SQL: "excluded.session_id"},
					"image_url":              clause.Expr{SQL: "excluded.image_url"},
					"product_image_urls":     clause.Expr{SQL: "excluded.product_image_urls"},
					"prompt":                 clause.Expr{SQL: "excluded.prompt"},
					"revised_prompt":         clause.Expr{SQL: "excluded.revised_prompt"},
					"image_model":            clause.Expr{SQL: "excluded.image_model"},
					"transparent_background": clause.Expr{SQL: "excluded.transparent_background"},
					"variation_intensity":    clause.Expr{SQL: "excluded.variation_intensity"},
					"role":                   clause.Expr{SQL: "excluded.role"},
					"role_label":             clause.Expr{SQL: "excluded.role_label"},
					"review_note":            clause.Expr{SQL: "excluded.review_note"},
					"sort_order":             clause.Expr{SQL: "excluded.sort_order"},
					"approved":               clause.Expr{SQL: "excluded.approved"},
					"updated_at":             clause.Expr{SQL: "excluded.updated_at"},
				}),
			}).Create(&designs).Error; err != nil {
				return err
			}
		} else if err := applySessionAccessScope(tx, ctx, "tenant_id", "").Where("session_id = ?", sessionID).Delete(&listingkit.SheinStudioDesign{}).Error; err != nil {
			return err
		}
		return tx.Model(&listingkit.SheinStudioSession{}).
			Scopes(func(db *gorm.DB) *gorm.DB { return applySessionAccessScope(db, ctx, "tenant_id", "user_id") }).
			Where("id = ?", sessionID).
			Updates(map[string]any{
				"approved_design_ids": listingkit.SheinStudioStringList(approvedIDs),
				"updated_at":          gorm.Expr("CURRENT_TIMESTAMP"),
			}).Error
	})
}

func (r *GormRepository) UpsertDesigns(ctx context.Context, sessionID string, approvedIDs []string, designs []listingkit.SheinStudioDesign) error {
	tenantID := tenantctx.TenantIDFromContext(ctx)
	designs = normalizeStudioDesignBatch(sessionID, tenantID, designs)
	if len(designs) == 0 {
		return nil
	}
	approved := make(map[string]struct{}, len(approvedIDs))
	for _, id := range approvedIDs {
		approved[id] = struct{}{}
	}
	for idx := range designs {
		_, designs[idx].Approved = approved[designs[idx].ID]
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"tenant_id":              clause.Expr{SQL: "excluded.tenant_id"},
			"session_id":             clause.Expr{SQL: "excluded.session_id"},
			"image_url":              clause.Expr{SQL: "excluded.image_url"},
			"product_image_urls":     clause.Expr{SQL: "excluded.product_image_urls"},
			"prompt":                 clause.Expr{SQL: "excluded.prompt"},
			"revised_prompt":         clause.Expr{SQL: "excluded.revised_prompt"},
			"image_model":            clause.Expr{SQL: "excluded.image_model"},
			"transparent_background": clause.Expr{SQL: "excluded.transparent_background"},
			"variation_intensity":    clause.Expr{SQL: "excluded.variation_intensity"},
			"role":                   clause.Expr{SQL: "excluded.role"},
			"role_label":             clause.Expr{SQL: "excluded.role_label"},
			"review_note":            clause.Expr{SQL: "excluded.review_note"},
			"sort_order":             clause.Expr{SQL: "excluded.sort_order"},
			"approved":               clause.Expr{SQL: "excluded.approved"},
			"updated_at":             clause.Expr{SQL: "excluded.updated_at"},
		}),
	}).Create(&designs).Error
}

func normalizeStudioDesignBatch(
	sessionID string,
	tenantID string,
	designs []listingkit.SheinStudioDesign,
) []listingkit.SheinStudioDesign {
	if len(designs) == 0 {
		return nil
	}

	normalized := make([]listingkit.SheinStudioDesign, 0, len(designs))
	indexByID := make(map[string]int, len(designs))
	for _, design := range designs {
		design.ID = strings.TrimSpace(design.ID)
		if design.ID == "" {
			continue
		}
		if strings.TrimSpace(design.SessionID) == "" {
			design.SessionID = sessionID
		}
		if design.TenantID == "" {
			design.TenantID = tenantID
		}
		if idx, exists := indexByID[design.ID]; exists {
			normalized[idx] = design
			continue
		}
		indexByID[design.ID] = len(normalized)
		normalized = append(normalized, design)
	}
	return normalized
}

func (r *GormRepository) ListSessionDesigns(ctx context.Context, sessionID string) ([]listingkit.SheinStudioDesign, error) {
	var designs []listingkit.SheinStudioDesign
	if err := applySessionAccessScope(r.db.WithContext(ctx), ctx, "tenant_id", "").
		Where("session_id = ?", sessionID).
		Order("sort_order ASC, created_at ASC").
		Find(&designs).Error; err != nil {
		return nil, err
	}
	return designs, nil
}

func (r *GormRepository) CountSessionDesignsBySessionIDs(ctx context.Context, sessionIDs []string) (map[string]int, error) {
	counts := make(map[string]int, len(sessionIDs))
	if len(sessionIDs) == 0 {
		return counts, nil
	}

	rows := make([]struct {
		SessionID string
		Count     int
	}, 0, len(sessionIDs))
	if err := applySessionAccessScope(r.db.WithContext(ctx), ctx, "tenant_id", "").
		Model(&listingkit.SheinStudioDesign{}).
		Select("session_id, COUNT(*) AS count").
		Where("session_id IN ?", sessionIDs).
		Group("session_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, sessionID := range sessionIDs {
		counts[sessionID] = 0
	}
	for _, row := range rows {
		counts[row.SessionID] = row.Count
	}
	return counts, nil
}

func (r *GormRepository) ListGalleryItems(ctx context.Context, limit int) ([]listingkit.SheinStudioSessionGalleryItem, error) {
	if limit <= 0 {
		limit = 240
	}

	rows := make([]struct {
		SessionID             string
		TenantID              string
		DesignID              string
		ImageURL              string
		Prompt                string
		SelectionJSON         string
		Status                string
		CreatedAt             time.Time
		UpdatedAt             time.Time
		ReviewNote            string
		RevisedPrompt         string
		ImageModel            string
		TransparentBackground bool
		VariationIntensity    string
	}, 0, limit)

	if err := r.db.WithContext(ctx).
		Table("shein_studio_designs AS d").
		Select([]string{
			"d.session_id AS session_id",
			"d.tenant_id AS tenant_id",
			"d.id AS design_id",
			"d.image_url AS image_url",
			"s.prompt AS prompt",
			"s.selection AS selection_json",
			"s.status AS status",
			"d.created_at AS created_at",
			"d.updated_at AS updated_at",
			"d.review_note AS review_note",
			"d.revised_prompt AS revised_prompt",
			"d.image_model AS image_model",
			"d.transparent_background AS transparent_background",
			"d.variation_intensity AS variation_intensity",
		}).
		Joins("JOIN shein_studio_sessions AS s ON s.id = d.session_id").
		Scopes(func(db *gorm.DB) *gorm.DB { return applySessionAccessScope(db, ctx, "s.tenant_id", "s.user_id") }).
		Order("d.updated_at DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]listingkit.SheinStudioSessionGalleryItem, 0, len(rows))
	for _, row := range rows {
		var selection listingkit.SheinStudioSelectionSnapshot
		_ = selection.Scan(row.SelectionJSON)
		items = append(items, listingkit.SheinStudioSessionGalleryItem{
			SessionID:             row.SessionID,
			TenantID:              row.TenantID,
			DesignID:              row.DesignID,
			ImageURL:              row.ImageURL,
			Prompt:                row.Prompt,
			ProductName:           selection.ProductName,
			Status:                row.Status,
			CreatedAt:             row.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:             row.UpdatedAt.UTC().Format(time.RFC3339),
			ReviewNote:            row.ReviewNote,
			RevisedPrompt:         row.RevisedPrompt,
			ImageModel:            row.ImageModel,
			TransparentBackground: row.TransparentBackground,
			VariationIntensity:    row.VariationIntensity,
		})
	}
	return items, nil
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

func (r *GormRepository) ListBatchSessions(ctx context.Context, limit int) ([]listingkit.SheinStudioSession, error) {
	if limit <= 0 {
		limit = 24
	}
	var sessions []listingkit.SheinStudioSession
	if err := applySessionAccessScope(r.db.WithContext(ctx), ctx, "tenant_id", "user_id").
		Where("saved_as_batch = ?", true).
		Order("updated_at DESC").
		Limit(limit).
		Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *GormRepository) ListTenantBatchNames(ctx context.Context) ([]string, error) {
	names := make([]string, 0)
	if err := applyTenantScope(r.db.WithContext(ctx), ctx, "tenant_id").
		Model(&listingkit.SheinStudioSession{}).
		Where("saved_as_batch = ?", true).
		Pluck("batch_name", &names).Error; err != nil {
		return nil, err
	}
	return names, nil
}

func applySessionAccessScope(db *gorm.DB, ctx context.Context, tenantColumn string, userColumn string) *gorm.DB {
	db = applyTenantScope(db, ctx, tenantColumn)
	if !listingkit.OwnerScopeEnabled() || strings.TrimSpace(userColumn) == "" {
		return db
	}
	userID := strings.TrimSpace(listingkit.RequestUserIDFromContext(ctx))
	if userID == "" {
		return db
	}
	return db.Where(userColumn+" = ?", userID)
}
