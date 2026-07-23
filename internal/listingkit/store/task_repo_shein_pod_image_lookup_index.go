package store

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/sheinpodimage"
)

func BuildSheinPODImageLookupIndex(task *listingkit.Task) (listingkit.SheinPODImageLookupIndex, bool) {
	record, ok := sheinpodimage.BuildSheinPODImageLookupRecord(task)
	if !ok {
		return listingkit.SheinPODImageLookupIndex{}, false
	}

	userID := strings.TrimSpace(task.UserID)
	if userID == "" {
		userID = strings.TrimSpace(listingkit.ResolveTaskUserID(task))
	}
	normalize := sheinpodimage.NormalizeSheinPODImageLookupQueryToken
	return listingkit.SheinPODImageLookupIndex{
		TaskID:                       record.TaskID,
		TenantID:                     task.TenantID,
		UserID:                       userID,
		StoreID:                      record.StoreID,
		Status:                       record.Status,
		Prompt:                       record.Prompt,
		ProductName:                  record.ProductName,
		SupplierCode:                 record.SupplierCode,
		SellerSKU:                    record.SellerSKU,
		SheinSPUName:                 record.SheinSPUName,
		SheinVersion:                 record.SheinVersion,
		AIOriginalImageURL:           record.AIOriginalImageURL,
		AIOriginalImageKey:           record.AIOriginalImageKey,
		SDSMainImageURL:              record.SDSMainImageURL,
		SDSGalleryImageURLs:          append([]string(nil), record.SDSGalleryImageURLs...),
		NormalizedTaskID:             normalize(record.TaskID),
		NormalizedProductName:        normalize(record.ProductName),
		NormalizedSupplierCode:       normalize(record.SupplierCode),
		NormalizedSellerSKU:          normalize(record.SellerSKU),
		NormalizedSheinSPUName:       normalize(record.SheinSPUName),
		NormalizedSheinVersion:       normalize(record.SheinVersion),
		NormalizedAIOriginalImageURL: normalize(record.AIOriginalImageURL),
		NormalizedAIOriginalImageKey: normalize(record.AIOriginalImageKey),
		NormalizedSDSMainImageURL:    normalize(record.SDSMainImageURL),
		CreatedAt:                    record.CreatedAt,
		UpdatedAt:                    record.UpdatedAt,
	}, true
}

func syncSheinPODImageLookupIndex(ctx context.Context, tx *gorm.DB, task *listingkit.Task) error {
	if tx == nil || !tx.Migrator().HasTable(&listingkit.SheinPODImageLookupIndex{}) {
		return nil
	}
	index, ok := BuildSheinPODImageLookupIndex(task)
	if !ok {
		if task == nil || strings.TrimSpace(task.ID) == "" {
			return nil
		}
		return tx.WithContext(ctx).
			Where("task_id = ?", task.ID).
			Delete(&listingkit.SheinPODImageLookupIndex{}).Error
	}
	createdAt := index.CreatedAt
	updatedAt := index.UpdatedAt
	if err := tx.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "task_id"}},
			UpdateAll: true,
		}).
		Create(&index).Error; err != nil {
		return err
	}
	return tx.WithContext(ctx).
		Model(&listingkit.SheinPODImageLookupIndex{}).
		Where("task_id = ?", index.TaskID).
		UpdateColumns(map[string]any{
			"created_at": createdAt,
			"updated_at": updatedAt,
		}).Error
}

func loadTaskForSheinPODImageLookupIndex(ctx context.Context, tx *gorm.DB, taskID string) (*listingkit.Task, error) {
	var task listingkit.Task
	if err := tx.WithContext(ctx).
		Where("id = ?", taskID).
		First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, listingkit.ErrTaskNotFound
		}
		return nil, err
	}
	return &task, nil
}
