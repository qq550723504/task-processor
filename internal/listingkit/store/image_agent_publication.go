package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/authidentity"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
)

type imageAgentPublicationReceiptRecord struct {
	TenantID            string    `gorm:"primaryKey;type:varchar(64)"`
	OwnerUserID         string    `gorm:"primaryKey;type:varchar(128)"`
	TaskID              string    `gorm:"primaryKey;type:varchar(36)"`
	IdempotencyKey      string    `gorm:"primaryKey;type:varchar(192)"`
	Fingerprint         string    `gorm:"type:varchar(64);not null"`
	AcknowledgementJSON []byte    `gorm:"not null"`
	CreatedAt           time.Time `gorm:"not null"`
}

func (imageAgentPublicationReceiptRecord) TableName() string {
	return "listingkit_image_agent_publication_receipts"
}

func AutoMigrateImageAgentPublicationReceipts(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	return db.AutoMigrate(&imageAgentPublicationReceiptRecord{})
}

func NewImageAgentPublicationTransactionRepository(db *gorm.DB) listingkit.ImageAgentPublicationTransactionRepository {
	return &taskRepository{db: db}
}

func (r *taskRepository) CommitImageAgentPublication(ctx context.Context, command listingkit.ImageAgentPublicationCommit, mutate listingkit.TaskResultMutation) (listingkit.ImageAgentPublicationAcknowledgement, error) {
	if err := validateImageAgentPublicationCommit(command); err != nil {
		return listingkit.ImageAgentPublicationAcknowledgement{}, err
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.TenantID != command.TenantID || identity.UserID != command.OwnerUserID {
		return listingkit.ImageAgentPublicationAcknowledgement{}, listingkit.ErrImageAgentPublicationConflict
	}
	if existing, found, err := r.findImageAgentPublicationReceipt(ctx, r.db, command); err != nil {
		if !isConcurrentImageAgentPublicationError(err) {
			return listingkit.ImageAgentPublicationAcknowledgement{}, err
		}
	} else if found {
		return matchImageAgentPublicationReceipt(existing, command)
	}

	var acknowledgement listingkit.ImageAgentPublicationAcknowledgement
	err := withImageAgentPublicationTransaction(ctx, r.db, func(tx *gorm.DB) error {
		if existing, found, err := r.findImageAgentPublicationReceipt(ctx, tx, command); err != nil {
			return err
		} else if found {
			matched, err := matchImageAgentPublicationReceipt(existing, command)
			acknowledgement = matched
			return err
		}
		var task listingkit.Task
		if err := applyTaskAccessScope(tx.Clauses(clause.Locking{Strength: "UPDATE"}), ctx).Where("id = ?", command.TaskID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return core.ErrTaskNotFound
			}
			return err
		}
		if task.TenantID != command.TenantID || listingkit.ResolveTaskUserID(&task) != command.OwnerUserID {
			return core.ErrTaskNotFound
		}
		if existing, found, err := r.findImageAgentPublicationReceipt(ctx, tx, command); err != nil {
			return err
		} else if found {
			matched, err := matchImageAgentPublicationReceipt(existing, command)
			acknowledgement = matched
			return err
		}
		ackJSON, err := json.Marshal(command.Acknowledgement)
		if err != nil {
			return err
		}
		receipt := imageAgentPublicationReceiptRecord{TenantID: command.TenantID, OwnerUserID: command.OwnerUserID, TaskID: command.TaskID, IdempotencyKey: command.IdempotencyKey, Fingerprint: command.Fingerprint, AcknowledgementJSON: ackJSON}
		if err := tx.Create(&receipt).Error; err != nil {
			return err
		}
		if mutate != nil {
			if err := mutate(&task); err != nil {
				return err
			}
		}
		if err := tx.Model(&listingkit.Task{}).Scopes(taskAccessScope(ctx)).Where("id = ?", command.TaskID).Updates(map[string]any{"status": task.Status, "error": task.Error, "result": task.Result, "retryable_block": task.RetryableBlock, "updated_at": currentTimestampValue(tx)}).Error; err != nil {
			return fmt.Errorf("persist image agent approved task result: %w", err)
		}
		finalTask, err := loadTaskForSheinPODImageLookupIndex(ctx, tx, command.TaskID)
		if err != nil {
			return err
		}
		if err := syncSheinPODImageLookupIndex(ctx, tx, finalTask); err != nil {
			return err
		}
		acknowledgement = cloneImageAgentPublicationAcknowledgement(command.Acknowledgement)
		return nil
	})
	if err == nil {
		return acknowledgement, nil
	}
	if recovered, found, recoverErr := r.recoverImageAgentPublicationReceipt(ctx, command); recoverErr != nil {
		return listingkit.ImageAgentPublicationAcknowledgement{}, recoverErr
	} else if found {
		return recovered, nil
	}
	return listingkit.ImageAgentPublicationAcknowledgement{}, err
}

func validateImageAgentPublicationCommit(command listingkit.ImageAgentPublicationCommit) error {
	ack := command.Acknowledgement
	if strings.TrimSpace(command.TenantID) == "" || strings.TrimSpace(command.OwnerUserID) == "" || strings.TrimSpace(command.TaskID) == "" || strings.TrimSpace(command.IdempotencyKey) == "" || strings.TrimSpace(command.Fingerprint) == "" || ack.TaskID != command.TaskID || ack.IdempotencyKey != command.IdempotencyKey || strings.TrimSpace(ack.RunID) == "" || ack.PlanRevision <= 0 || strings.TrimSpace(ack.ResultDigest) == "" {
		return listingkit.ErrImageAgentPublicationConflict
	}
	return nil
}

func (r *taskRepository) findImageAgentPublicationReceipt(ctx context.Context, db *gorm.DB, command listingkit.ImageAgentPublicationCommit) (imageAgentPublicationReceiptRecord, bool, error) {
	var row imageAgentPublicationReceiptRecord
	err := db.WithContext(ctx).Where("tenant_id = ? AND owner_user_id = ? AND task_id = ? AND idempotency_key = ?", command.TenantID, command.OwnerUserID, command.TaskID, command.IdempotencyKey).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, false, nil
	}
	if err != nil {
		return row, false, err
	}
	return row, true, nil
}

func matchImageAgentPublicationReceipt(row imageAgentPublicationReceiptRecord, command listingkit.ImageAgentPublicationCommit) (listingkit.ImageAgentPublicationAcknowledgement, error) {
	if row.Fingerprint != command.Fingerprint {
		return listingkit.ImageAgentPublicationAcknowledgement{}, listingkit.ErrImageAgentPublicationConflict
	}
	var acknowledgement listingkit.ImageAgentPublicationAcknowledgement
	if err := json.Unmarshal(row.AcknowledgementJSON, &acknowledgement); err != nil {
		return acknowledgement, err
	}
	return cloneImageAgentPublicationAcknowledgement(acknowledgement), nil
}

func (r *taskRepository) recoverImageAgentPublicationReceipt(ctx context.Context, command listingkit.ImageAgentPublicationCommit) (listingkit.ImageAgentPublicationAcknowledgement, bool, error) {
	for attempt := 0; attempt < 32; attempt++ {
		row, found, err := r.findImageAgentPublicationReceipt(ctx, r.db, command)
		if err == nil && found {
			ack, matchErr := matchImageAgentPublicationReceipt(row, command)
			return ack, true, matchErr
		}
		if err != nil && !isConcurrentImageAgentPublicationError(err) {
			return listingkit.ImageAgentPublicationAcknowledgement{}, false, err
		}
		time.Sleep(time.Duration(attempt+1) * 2 * time.Millisecond)
	}
	return listingkit.ImageAgentPublicationAcknowledgement{}, false, nil
}

func withImageAgentPublicationTransaction(ctx context.Context, db *gorm.DB, transaction func(*gorm.DB) error) error {
	var err error
	for attempt := 0; attempt < 32; attempt++ {
		err = db.WithContext(ctx).Transaction(transaction)
		if err == nil || !isConcurrentImageAgentPublicationError(err) {
			return err
		}
		time.Sleep(time.Duration(attempt+1) * 2 * time.Millisecond)
	}
	return err
}

func isConcurrentImageAgentPublicationError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "locked") || strings.Contains(message, "busy") || strings.Contains(message, "unique") || strings.Contains(message, "duplicate")
}

func cloneImageAgentPublicationAcknowledgement(input listingkit.ImageAgentPublicationAcknowledgement) listingkit.ImageAgentPublicationAcknowledgement {
	input.CandidateAssetIDs = append([]string(nil), input.CandidateAssetIDs...)
	return input
}
