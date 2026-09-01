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
		return nil, errors.New("product asset repository database is nil")
	}
	return &repository{db: db}, nil
}

func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return errors.New("product asset persistence database is nil")
	}
	return db.AutoMigrate(&ApprovedAssetRecord{}, &ApprovalReceiptRecord{})
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
		return productasset.ApprovalReceipt{}, fmt.Errorf("hash approval payload: %w", err)
	}
	assetIDs := make([]string, len(commit.Assets))
	assetRecords := make([]ApprovedAssetRecord, len(commit.Assets))
	for index, approved := range commit.Assets {
		payload, marshalErr := json.Marshal(canonicalApprovedAssetFromDomain(approved))
		if marshalErr != nil {
			return productasset.ApprovalReceipt{}, fmt.Errorf("marshal approved asset %q: %w", approved.ID, marshalErr)
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
		return productasset.ApprovalReceipt{}, fmt.Errorf("marshal approval receipt: %w", err)
	}

	receipt := productasset.ApprovalReceipt{}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		candidate := ApprovalReceiptRecord{
			TenantID: commit.TenantID, ActionID: commit.ActionID,
			PayloadHash: payloadHash, AssetIDsJSON: assetIDsJSON,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate)
		if created.Error != nil {
			return fmt.Errorf("create approval receipt: %w", created.Error)
		}
		if created.RowsAffected == 0 {
			return loadExistingReceipt(tx, commit, payloadHash, &receipt)
		}

		inserted := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&assetRecords)
		if inserted.Error != nil {
			return fmt.Errorf("insert approved asset batch: %w", inserted.Error)
		}
		if inserted.RowsAffected != int64(len(assetRecords)) {
			return productasset.ErrApprovalConflict
		}
		receipt = productasset.ApprovalReceipt{ActionID: commit.ActionID, AssetIDs: append([]string(nil), assetIDs...)}
		return nil
	})
	if err != nil {
		return productasset.ApprovalReceipt{}, err
	}
	return productasset.CloneApprovalReceipt(receipt), nil
}

func loadExistingReceipt(tx *gorm.DB, commit productasset.ApprovalCommit, payloadHash string, receipt *productasset.ApprovalReceipt) error {
	var existing ApprovalReceiptRecord
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND action_id = ?", commit.TenantID, commit.ActionID).
		Take(&existing).Error
	if err != nil {
		return fmt.Errorf("load existing approval receipt: %w", err)
	}
	if existing.PayloadHash != payloadHash {
		return productasset.ErrApprovalConflict
	}
	var assetIDs []string
	if err := json.Unmarshal(existing.AssetIDsJSON, &assetIDs); err != nil {
		return fmt.Errorf("decode existing approval receipt: %w", err)
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
		return productasset.ApprovedAssetInventory{}, fmt.Errorf("load approved asset inventory: %w", err)
	}
	if len(records) == 0 {
		return productasset.ApprovedAssetInventory{}, productasset.ErrApprovedAssetsNotReady
	}
	approved := make([]productasset.ApprovedAsset, len(records))
	for index, record := range records {
		var persisted canonicalApprovedAsset
		if err := json.Unmarshal(record.PayloadJSON, &persisted); err != nil {
			return productasset.ApprovedAssetInventory{}, fmt.Errorf("decode approved asset %q: %w", record.AssetID, err)
		}
		approved[index] = persisted.domainAsset()
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

var _ productasset.Repository = (*repository)(nil)
