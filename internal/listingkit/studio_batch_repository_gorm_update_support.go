package listingkit

import (
	"context"
	"time"

	"gorm.io/gorm"
)

func (r *GormStudioBatchRepository) ReplaceStudioItemMaterializedDesigns(ctx context.Context, itemID string, designs []StudioMaterializedDesignRecord) error {
	item, err := r.GetStudioBatchItem(ctx, itemID)
	if err != nil {
		return err
	}

	rows := make([]StudioMaterializedDesignRecord, 0, len(designs))
	for _, design := range designs {
		row := design
		row.BatchID = item.BatchID
		row.ItemID = item.ID
		row.TenantID = item.TenantID
		row.UserID = item.UserID
		if row.TargetGroupKey == "" {
			row.TargetGroupKey = item.TargetGroupKey
		}
		if row.TargetGroupLabel == "" {
			row.TargetGroupLabel = item.TargetGroupLabel
		}
		if row.ReviewStatus == "" {
			row.ReviewStatus = StudioMaterializedDesignReviewStatusApproved
		}
		rows = append(rows, row)
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := applyStudioBatchAccessScope(tx, ctx).
			Where("item_id = ?", itemID).
			Delete(&StudioMaterializedDesignRecord{}).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

func (r *GormStudioBatchRepository) ReplaceStudioMaterializedDesignReviews(ctx context.Context, batchID string, designIDs []string, updatedAt time.Time) error {
	if _, err := r.GetStudioBatch(ctx, batchID); err != nil {
		return err
	}

	approvedIDs := append([]string(nil), designIDs...)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(approvedIDs) > 0 {
			var count int64
			if err := applyStudioBatchAccessScope(tx, ctx).
				Model(&StudioMaterializedDesignRecord{}).
				Where("batch_id = ? AND id IN ?", batchID, approvedIDs).
				Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(approvedIDs)) {
				return gorm.ErrRecordNotFound
			}
		}

		resetResult := applyStudioBatchAccessScope(tx, ctx).
			Model(&StudioMaterializedDesignRecord{}).
			Where("batch_id = ?", batchID).
			Updates(map[string]any{
				"review_status": StudioMaterializedDesignReviewStatusUnreviewed,
				"updated_at":    updatedAt,
			})
		if resetResult.Error != nil {
			return resetResult.Error
		}

		if len(approvedIDs) == 0 {
			return nil
		}

		approveResult := applyStudioBatchAccessScope(tx, ctx).
			Model(&StudioMaterializedDesignRecord{}).
			Where("batch_id = ? AND id IN ?", batchID, approvedIDs).
			Updates(map[string]any{
				"review_status": StudioMaterializedDesignReviewStatusApproved,
				"updated_at":    updatedAt,
			})
		return approveResult.Error
	})
}

func (r *GormStudioBatchRepository) UpdateStudioBatch(ctx context.Context, batch *StudioBatchRecord) error {
	if batch == nil {
		return nil
	}

	row := *batch
	applyStudioBatchScopeDefaults(ctx, &row.TenantID, &row.UserID)
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioBatchRecord{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"status":                         row.Status,
			"prompt":                         row.Prompt,
			"prompt_mode":                    row.PromptMode,
			"grouped_image_mode":             row.GroupedImageMode,
			"selection":                      row.Selection,
			"grouped_selections":             row.GroupedSelections,
			"style_count":                    row.StyleCount,
			"variation_intensity":            row.VariationIntensity,
			"artwork_model":                  row.ArtworkModel,
			"selected_sds_images":            row.SelectedSDSImages,
			"hot_style_reference_image_urls": row.HotStyleReferenceImageURLs,
			"hot_style_reference_brief":      row.HotStyleReferenceBrief,
			"hot_style_reference_prompt":     row.HotStyleReferencePrompt,
			"transparent_background":         row.TransparentBackground,
			"shein_store_id":                 row.SheinStoreID,
			"updated_at":                     row.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *GormStudioBatchRepository) UpdateStudioBatchItem(ctx context.Context, item *StudioBatchItemRecord) error {
	if item == nil {
		return nil
	}

	row := *item
	applyStudioBatchScopeDefaults(ctx, &row.TenantID, &row.UserID)
	existing, err := r.GetStudioBatchItem(ctx, row.ID)
	if err != nil {
		return err
	}
	if row.BatchID != "" && row.BatchID != existing.BatchID {
		return ErrStudioBatchOwnershipConflict
	}
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioBatchItemRecord{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"target_group_key":   row.TargetGroupKey,
			"target_group_label": row.TargetGroupLabel,
			"selection_ids":      row.SelectionIDs,
			"group_mode":         row.GroupMode,
			"status":             row.Status,
			"selection_count":    row.SelectionCount,
			"last_error":         row.LastError,
			"updated_at":         row.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *GormStudioBatchRepository) UpdateStudioGenerationAttempt(ctx context.Context, attempt *StudioGenerationAttemptRecord) error {
	if attempt == nil {
		return nil
	}

	row := *attempt
	applyStudioBatchScopeDefaults(ctx, &row.TenantID, &row.UserID)
	var existing StudioGenerationAttemptRecord
	if err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Where("id = ?", row.ID).
		First(&existing).Error; err != nil {
		return err
	}
	if row.BatchID != "" && row.BatchID != existing.BatchID {
		return ErrStudioBatchOwnershipConflict
	}
	if row.ItemID != "" && row.ItemID != existing.ItemID {
		return ErrStudioBatchOwnershipConflict
	}
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioGenerationAttemptRecord{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"attempt_no":              row.AttemptNo,
			"status":                  row.Status,
			"provider":                row.Provider,
			"upstream_job_id":         row.UpstreamJobID,
			"request_id":              row.RequestID,
			"request_payload":         row.RequestPayload,
			"submit_response_payload": row.SubmitResponsePayload,
			"result_payload":          row.ResultPayload,
			"result_checked_at":       row.ResultCheckedAt,
			"query_attempts":          row.QueryAttempts,
			"error_message":           row.ErrorMessage,
			"started_at":              row.StartedAt,
			"finished_at":             row.FinishedAt,
			"updated_at":              row.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *GormStudioBatchRepository) UpdateStudioMaterializedDesign(ctx context.Context, design *StudioMaterializedDesignRecord) error {
	if design == nil {
		return nil
	}

	row := *design
	applyStudioBatchScopeDefaults(ctx, &row.TenantID, &row.UserID)
	var existing StudioMaterializedDesignRecord
	if err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Where("id = ?", row.ID).
		First(&existing).Error; err != nil {
		return err
	}
	if row.BatchID != "" && row.BatchID != existing.BatchID {
		return ErrStudioBatchOwnershipConflict
	}
	if row.ItemID != "" && row.ItemID != existing.ItemID {
		return ErrStudioBatchOwnershipConflict
	}
	if row.SourceAttemptID != "" && row.SourceAttemptID != existing.SourceAttemptID {
		return ErrStudioBatchOwnershipConflict
	}
	if row.ReviewStatus == "" {
		row.ReviewStatus = existing.ReviewStatus
	}
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioMaterializedDesignRecord{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"target_group_key":   row.TargetGroupKey,
			"target_group_label": row.TargetGroupLabel,
			"image_url":          row.ImageURL,
			"review_status":      row.ReviewStatus,
			"sort_order":         row.SortOrder,
			"review_note":        row.ReviewNote,
			"updated_at":         row.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
