package assetpersistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	productasset "task-processor/internal/product/asset"
)

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) (productasset.Repository, error) {
	if db == nil {
		return nil, repositoryUnavailable("construct repository", errors.New("database is nil"))
	}
	return &repository{db: db}, nil
}

func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return repositoryUnavailable("migrate schema", errors.New("database is nil"))
	}
	return mapRepositoryError("migrate schema", db.AutoMigrate(&ApprovedAssetRecord{}, &ApprovalReceiptRecord{}))
}

func (r *repository) CommitApproval(ctx context.Context, commit productasset.ApprovalCommit) (productasset.ApprovalReceipt, error) {
	if err := ctx.Err(); err != nil {
		return productasset.ApprovalReceipt{}, err
	}
	if err := productasset.ValidateApprovalCommit(commit); err != nil {
		return productasset.ApprovalReceipt{}, err
	}

	payloadHash, err := approvalPayloadHash(commit)
	if err != nil {
		return productasset.ApprovalReceipt{}, repositoryStateInvalid("hash approval payload", err)
	}
	assetIDs := make([]string, len(commit.Assets))
	assetRecords := make([]ApprovedAssetRecord, len(commit.Assets))
	for index, approved := range commit.Assets {
		payload, marshalErr := json.Marshal(canonicalApprovedAssetFromDomain(approved))
		if marshalErr != nil {
			return productasset.ApprovalReceipt{}, repositoryStateInvalid("marshal approved asset "+approved.ID, marshalErr)
		}
		assetIDs[index] = approved.ID
		assetRecords[index] = ApprovedAssetRecord{
			TenantID: commit.TenantID, RunID: approved.RunID, PlanRevision: approved.PlanRevision,
			SlotID: approved.SlotID, Attempt: approved.Attempt, ActionID: commit.ActionID,
			AssetID: approved.ID, ProductKey: commit.ProductKey, PayloadJSON: payload,
		}
	}
	assetIDsJSON, err := json.Marshal(assetIDs)
	if err != nil {
		return productasset.ApprovalReceipt{}, repositoryStateInvalid("marshal approval receipt", err)
	}

	receipt := productasset.ApprovalReceipt{}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		candidate := ApprovalReceiptRecord{
			TenantID: commit.TenantID, ActionID: commit.ActionID,
			PayloadHash: payloadHash, AssetIDsJSON: assetIDsJSON,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate)
		if created.Error != nil {
			return mapRepositoryError("create approval receipt", created.Error)
		}
		if created.RowsAffected == 0 {
			return loadExistingReceipt(tx, commit, payloadHash, &receipt)
		}

		inserted := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&assetRecords)
		if inserted.Error != nil {
			return mapRepositoryError("insert approved asset batch", inserted.Error)
		}
		if inserted.RowsAffected != int64(len(assetRecords)) {
			return productasset.ErrApprovalConflict
		}
		receipt = productasset.ApprovalReceipt{ActionID: commit.ActionID, AssetIDs: append([]string(nil), assetIDs...)}
		return nil
	})
	if err != nil {
		return productasset.ApprovalReceipt{}, mapRepositoryError("commit approval transaction", err)
	}
	return productasset.CloneApprovalReceipt(receipt), nil
}

func loadExistingReceipt(tx *gorm.DB, commit productasset.ApprovalCommit, payloadHash string, receipt *productasset.ApprovalReceipt) error {
	var existing ApprovalReceiptRecord
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND action_id = ?", commit.TenantID, commit.ActionID).
		Take(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return repositoryStateInvalid("load existing approval receipt", errors.New("receipt disappeared after action conflict"))
		}
		return mapRepositoryError("load existing approval receipt", err)
	}
	decodedHash, err := hex.DecodeString(existing.PayloadHash)
	if err != nil || len(decodedHash) != sha256.Size || hex.EncodeToString(decodedHash) != existing.PayloadHash {
		return repositoryStateInvalid("decode existing approval receipt hash", errors.New("payload hash is not canonical SHA-256 hex"))
	}
	if existing.PayloadHash != payloadHash {
		return productasset.ErrApprovalConflict
	}
	var assetIDs []string
	if err := json.Unmarshal(existing.AssetIDsJSON, &assetIDs); err != nil {
		return repositoryStateInvalid("decode existing approval receipt", err)
	}
	if len(assetIDs) != len(commit.Assets) {
		return repositoryStateInvalid("decode existing approval receipt", errors.New("receipt asset ids do not match committed payload"))
	}
	for index, approved := range commit.Assets {
		if assetIDs[index] != approved.ID {
			return repositoryStateInvalid("decode existing approval receipt", errors.New("receipt asset ids do not match committed payload"))
		}
	}
	*receipt = productasset.ApprovalReceipt{ActionID: existing.ActionID, AssetIDs: assetIDs}
	return nil
}

func (r *repository) GetApprovedInventory(ctx context.Context, scope productasset.InventoryScope) (productasset.ApprovedAssetInventory, error) {
	if err := ctx.Err(); err != nil {
		return productasset.ApprovedAssetInventory{}, err
	}
	if err := productasset.ValidateInventoryScope(scope); err != nil {
		return productasset.ApprovedAssetInventory{}, err
	}

	var records []ApprovedAssetRecord
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND product_key = ?", scope.TenantID, scope.ProductKey).
		Order("run_id ASC, plan_revision ASC, slot_id ASC, attempt ASC, action_id ASC, asset_id ASC").
		Find(&records).Error
	if err != nil {
		return productasset.ApprovedAssetInventory{}, mapRepositoryError("load approved asset inventory", err)
	}
	if len(records) == 0 {
		return productasset.ApprovedAssetInventory{}, productasset.ErrApprovedAssetsNotReady
	}
	approved := make([]productasset.ApprovedAsset, len(records))
	for index, record := range records {
		var persisted canonicalApprovedAsset
		if err := json.Unmarshal(record.PayloadJSON, &persisted); err != nil {
			return productasset.ApprovedAssetInventory{}, repositoryStateInvalid("decode approved asset "+record.AssetID, err)
		}
		approved[index] = persisted.domainAsset()
		if err := validatePersistedAsset(record, approved[index]); err != nil {
			return productasset.ApprovedAssetInventory{}, repositoryStateInvalid("validate approved asset "+record.AssetID, err)
		}
	}
	return productasset.CloneApprovedAssetInventory(productasset.ApprovedAssetInventory{Scope: scope, Assets: approved}), nil
}

type canonicalApprovalPayload struct {
	TenantID   string                   `json:"tenant_id"`
	ProductKey string                   `json:"product_key"`
	ActionID   string                   `json:"action_id"`
	Assets     []canonicalApprovedAsset `json:"assets"`
}

type canonicalApprovedAsset struct {
	ID            string            `json:"id"`
	RunID         string            `json:"run_id"`
	PlanRevision  int64             `json:"plan_revision"`
	SlotID        string            `json:"slot_id"`
	Attempt       int               `json:"attempt"`
	Role          productasset.Role `json:"role"`
	URL           string            `json:"url"`
	SourceAssetID string            `json:"source_asset_id"`
	Width         int               `json:"width"`
	Height        int               `json:"height"`
	Operations    []string          `json:"operations"`
}

func canonicalApprovedAssetFromDomain(approved productasset.ApprovedAsset) canonicalApprovedAsset {
	return canonicalApprovedAsset{
		ID: approved.ID, RunID: approved.RunID, PlanRevision: approved.PlanRevision,
		SlotID: approved.SlotID, Attempt: approved.Attempt, Role: approved.Role,
		URL: approved.URL, SourceAssetID: approved.SourceAssetID,
		Width: approved.Width, Height: approved.Height, Operations: approved.Operations,
	}
}

func (approved canonicalApprovedAsset) domainAsset() productasset.ApprovedAsset {
	return productasset.ApprovedAsset{
		ID: approved.ID, RunID: approved.RunID, PlanRevision: approved.PlanRevision,
		SlotID: approved.SlotID, Attempt: approved.Attempt, Role: approved.Role,
		URL: approved.URL, SourceAssetID: approved.SourceAssetID,
		Width: approved.Width, Height: approved.Height, Operations: approved.Operations,
	}
}

func approvalPayloadHash(commit productasset.ApprovalCommit) (string, error) {
	payload := canonicalApprovalPayload{
		TenantID: commit.TenantID, ProductKey: commit.ProductKey, ActionID: commit.ActionID,
		Assets: make([]canonicalApprovedAsset, len(commit.Assets)),
	}
	for index, approved := range commit.Assets {
		payload.Assets[index] = canonicalApprovedAssetFromDomain(approved)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func mapRepositoryError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	for _, stable := range []error{
		productasset.ErrInvalidApproval,
		productasset.ErrInvalidInventoryScope,
		productasset.ErrApprovalConflict,
		productasset.ErrApprovedAssetsNotReady,
		productasset.ErrRepositoryUnavailable,
		productasset.ErrRepositoryStateInvalid,
	} {
		if errors.Is(err, stable) {
			return err
		}
	}
	return repositoryUnavailable(operation, err)
}

func repositoryUnavailable(operation string, cause error) error {
	return fmt.Errorf("%w: %s: %v", productasset.ErrRepositoryUnavailable, operation, cause)
}

func repositoryStateInvalid(operation string, cause error) error {
	return fmt.Errorf("%w: %s: %v", productasset.ErrRepositoryStateInvalid, operation, cause)
}

func validatePersistedAsset(record ApprovedAssetRecord, approved productasset.ApprovedAsset) error {
	if approved.ID != record.AssetID ||
		approved.RunID != record.RunID ||
		approved.PlanRevision != record.PlanRevision ||
		approved.SlotID != record.SlotID ||
		approved.Attempt != record.Attempt {
		return errors.New("payload identity does not match indexed record identity")
	}
	commit := productasset.ApprovalCommit{
		TenantID: record.TenantID, ProductKey: record.ProductKey, ActionID: record.ActionID,
		Assets: []productasset.ApprovedAsset{approved},
	}
	if err := productasset.ValidateApprovalCommit(commit); err != nil {
		return fmt.Errorf("persisted payload violates domain contract: %v", err)
	}
	return nil
}

var _ productasset.Repository = (*repository)(nil)
