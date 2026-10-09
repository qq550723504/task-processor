package assetpersistence

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	productasset "task-processor/internal/product/asset"
)

// The successful action receipt is resolved before this function. Its head CAS
// and complete asset set are committed in the same existing Asset transaction.
func compareImageInventoryHead(tx *gorm.DB, commit productasset.ApprovalCommit) error {
	expected := commit.ImageSet.ExpectedHead
	query := tx.Where("tenant_id = ? AND product_key = ? AND target_platform = ?", commit.TenantID, commit.ProductKey, commit.TargetPlatform)
	if expected.ActionID == "" {
		candidate := ApprovedInventoryHeadRecord{TenantID: commit.TenantID, ProductKey: commit.ProductKey, TargetPlatform: commit.TargetPlatform, ActionID: commit.ActionID}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate)
		if created.Error != nil {
			return mapRepositoryError("create selected inventory head", created.Error)
		}
		if created.RowsAffected != 1 {
			return productasset.ErrApprovalConflict
		}
		return nil
	}
	var head ApprovedInventoryHeadRecord
	if err := query.Clauses(clause.Locking{Strength: "UPDATE"}).Take(&head).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return productasset.ErrApprovalConflict
		}
		return mapRepositoryError("lock selected inventory head", err)
	}
	if head.ActionID != expected.ActionID {
		return productasset.ErrApprovalConflict
	}
	var original ApprovalReceiptRecord
	if err := tx.Where("tenant_id = ? AND action_id = ?", commit.TenantID, head.ActionID).Take(&original).Error; err != nil {
		return mapRepositoryError("read selected inventory head receipt", err)
	}
	if original.PayloadHash != expected.PayloadHash {
		return productasset.ErrApprovalConflict
	}
	return nil
}

func (r *repository) ReadImageSetInventory(ctx context.Context, scope productasset.InventoryScope) (productasset.ImageSetInventory, error) {
	if err := productasset.ValidateInventoryScope(scope); err != nil {
		return productasset.ImageSetInventory{}, err
	}
	var head ApprovedInventoryHeadRecord
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND product_key = ? AND target_platform = ?", scope.TenantID, scope.ProductKey, scope.TargetPlatform).Take(&head).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return productasset.ImageSetInventory{Scope: scope}, nil
	}
	if err != nil {
		return productasset.ImageSetInventory{}, mapRepositoryError("read selected inventory head", err)
	}
	commit, err := r.ReadApprovalCommit(ctx, scope.TenantID, head.ActionID)
	if err != nil {
		return productasset.ImageSetInventory{}, err
	}
	if commit.ProductKey != scope.ProductKey || commit.TargetPlatform != scope.TargetPlatform {
		return productasset.ImageSetInventory{}, productasset.ErrRepositoryStateInvalid
	}
	hash, err := approvalPayloadHash(commit)
	if err != nil {
		return productasset.ImageSetInventory{}, productasset.ErrRepositoryStateInvalid
	}
	result := productasset.ImageSetInventory{Scope: scope, Head: productasset.ImageInventoryHead{ActionID: commit.ActionID, PayloadHash: hash}}
	if commit.SourceSnapshotVersion == scope.SourceSnapshotVersion {
		result.Assets = commit.Assets
	}
	return result, nil
}
