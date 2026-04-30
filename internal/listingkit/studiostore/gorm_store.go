package studiostore

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

func (r *GormRepository) FindLatestSessionBySelectionKey(ctx context.Context, selectionKey string) (*listingkit.SheinStudioSession, error) {
	var session listingkit.SheinStudioSession
	err := r.db.WithContext(ctx).
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
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *GormRepository) GetSession(ctx context.Context, sessionID string) (*listingkit.SheinStudioSession, error) {
	var session listingkit.SheinStudioSession
	err := r.db.WithContext(ctx).Where("id = ?", sessionID).First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *GormRepository) UpdateSession(ctx context.Context, session *listingkit.SheinStudioSession) error {
	return r.db.WithContext(ctx).Save(session).Error
}

func (r *GormRepository) ReplaceDesigns(ctx context.Context, sessionID string, approvedIDs []string, designs []listingkit.SheinStudioDesign) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", sessionID).Delete(&listingkit.SheinStudioDesign{}).Error; err != nil {
			return err
		}
		if len(designs) > 0 {
			if err := tx.Create(&designs).Error; err != nil {
				return err
			}
		}
		return tx.Model(&listingkit.SheinStudioSession{}).
			Where("id = ?", sessionID).
			Updates(map[string]any{
				"approved_design_ids": listingkit.SheinStudioStringList(approvedIDs),
				"updated_at":          gorm.Expr("CURRENT_TIMESTAMP"),
			}).Error
	})
}

func (r *GormRepository) ListSessionDesigns(ctx context.Context, sessionID string) ([]listingkit.SheinStudioDesign, error) {
	var designs []listingkit.SheinStudioDesign
	if err := r.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("sort_order ASC, created_at ASC").
		Find(&designs).Error; err != nil {
		return nil, err
	}
	return designs, nil
}

func (r *GormRepository) ListGalleryItems(ctx context.Context, limit int) ([]listingkit.SheinStudioSessionGalleryItem, error) {
	if limit <= 0 {
		limit = 240
	}

	rows := make([]struct {
		SessionID     string
		DesignID      string
		ImageURL      string
		Prompt        string
		SelectionJSON string
		Status        string
		CreatedAt     time.Time
		UpdatedAt     time.Time
		ReviewNote    string
		RevisedPrompt string
	}, 0, limit)

	if err := r.db.WithContext(ctx).
		Table("shein_studio_designs AS d").
		Select([]string{
			"d.session_id AS session_id",
			"d.id AS design_id",
			"d.image_url AS image_url",
			"s.prompt AS prompt",
			"s.selection AS selection_json",
			"s.status AS status",
			"d.created_at AS created_at",
			"d.updated_at AS updated_at",
			"d.review_note AS review_note",
			"d.revised_prompt AS revised_prompt",
		}).
		Joins("JOIN shein_studio_sessions AS s ON s.id = d.session_id").
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
			SessionID:     row.SessionID,
			DesignID:      row.DesignID,
			ImageURL:      row.ImageURL,
			Prompt:        row.Prompt,
			ProductName:   selection.ProductName,
			Status:        row.Status,
			CreatedAt:     row.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:     row.UpdatedAt.UTC().Format(time.RFC3339),
			ReviewNote:    row.ReviewNote,
			RevisedPrompt: row.RevisedPrompt,
		})
	}
	return items, nil
}
