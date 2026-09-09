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
	db                        *gorm.DB
	maxApprovedInventoryBytes int
}

func NewRepository(db *gorm.DB) (productasset.Repository, error) {
	if db == nil {
		return nil, repositoryUnavailable("construct repository", errors.New("database is nil"))
	}
	return &repository{db: db}, nil
}

// NewBoundedApprovedInventoryReader creates the current exact-inventory read
// adapter with a persistence boundary. PostgreSQL measures both the largest
// raw JSON row and the complete inventory wire size before any matching
// payload is transferred into a Go slice or decoded.
func NewBoundedApprovedInventoryReader(db *gorm.DB, maxBytes int) (productasset.ApprovedInventoryReader, error) {
	if db == nil || maxBytes <= 0 {
		return nil, repositoryUnavailable("construct bounded inventory reader", errors.New("database and positive byte limit are required"))
	}
	return &repository{db: db, maxApprovedInventoryBytes: maxBytes}, nil
}

func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return repositoryUnavailable("migrate schema", errors.New("database is nil"))
	}
	return mapRepositoryError("migrate schema", db.AutoMigrate(&ApprovedAssetRecord{}, &ApprovalReceiptRecord{}, &ApprovedInventoryHeadRecord{}, &ApprovedInventoryVersionHeadRecord{}))
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
			AssetID: approved.ID, ProductKey: commit.ProductKey, TargetPlatform: commit.TargetPlatform,
			SourceSnapshotVersion: commit.SourceSnapshotVersion, PayloadJSON: payload,
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
		if err := advanceCurrentInventoryHead(tx, commit); err != nil {
			return err
		}
		if commit.SourceSnapshotVersion > 0 {
			if err := advanceVersionedInventoryHead(tx, commit); err != nil {
				return err
			}
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

	var actionID string
	var err error
	if scope.SourceSnapshotVersion > 0 {
		var head ApprovedInventoryVersionHeadRecord
		err := r.db.WithContext(ctx).
			Where("tenant_id = ? AND product_key = ? AND target_platform = ? AND source_snapshot_version = ?", scope.TenantID, scope.ProductKey, scope.TargetPlatform, scope.SourceSnapshotVersion).
			Take(&head).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return productasset.ApprovedAssetInventory{}, productasset.ErrApprovedAssetsNotReady
		} else if err != nil {
			return productasset.ApprovedAssetInventory{}, mapRepositoryError("load versioned approved inventory head", err)
		} else {
			actionID = head.ActionID
		}
	} else {
		var head ApprovedInventoryHeadRecord
		err := r.db.WithContext(ctx).
			Where("tenant_id = ? AND product_key = ? AND target_platform = ?", scope.TenantID, scope.ProductKey, scope.TargetPlatform).
			Take(&head).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return productasset.ApprovedAssetInventory{}, productasset.ErrApprovedAssetsNotReady
		}
		if err != nil {
			return productasset.ApprovedAssetInventory{}, mapRepositoryError("load approved inventory head", err)
		}
		actionID = head.ActionID
	}
	if r.maxApprovedInventoryBytes > 0 {
		if err := r.enforceApprovedInventoryReadBound(ctx, scope, actionID); err != nil {
			return productasset.ApprovedAssetInventory{}, err
		}
	}

	var records []ApprovedAssetRecord
	recordQuery := r.db.WithContext(ctx).
		Where("tenant_id = ? AND product_key = ? AND target_platform = ? AND action_id = ?", scope.TenantID, scope.ProductKey, scope.TargetPlatform, actionID)
	if scope.SourceSnapshotVersion > 0 {
		recordQuery = recordQuery.Where("source_snapshot_version = ?", scope.SourceSnapshotVersion)
	}
	err = recordQuery.Order("slot_id ASC, attempt ASC, asset_id ASC").Find(&records).Error
	if err != nil {
		return productasset.ApprovedAssetInventory{}, mapRepositoryError("load approved asset inventory", err)
	}
	if len(records) == 0 {
		return productasset.ApprovedAssetInventory{}, repositoryStateInvalid("load approved asset inventory", errors.New("inventory head has no approved assets"))
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

type approvedInventorySize struct {
	AssetCount        int64
	MaxPayloadBytes   int64
	TotalPayloadBytes int64
}

func (r *repository) enforceApprovedInventoryReadBound(ctx context.Context, scope productasset.InventoryScope, actionID string) error {
	scopeJSON, err := json.Marshal(scope)
	if err != nil {
		return repositoryStateInvalid("encode approved inventory scope", err)
	}
	query := r.db.WithContext(ctx).Model(&ApprovedAssetRecord{}).
		Where("tenant_id = ? AND product_key = ? AND target_platform = ? AND action_id = ?", scope.TenantID, scope.ProductKey, scope.TargetPlatform, actionID)
	if scope.SourceSnapshotVersion > 0 {
		query = query.Where("source_snapshot_version = ?", scope.SourceSnapshotVersion)
	}
	var size approvedInventorySize
	result := query.Select(`COUNT(*) AS asset_count, COALESCE(MAX(octet_length(CAST(payload_json AS text))), 0) AS max_payload_bytes, COALESCE(SUM(octet_length(CAST(payload_json AS text))), 0) AS total_payload_bytes`).Scan(&size)
	if result.Error != nil {
		return mapRepositoryError("measure approved asset inventory", result.Error)
	}
	limit := int64(r.maxApprovedInventoryBytes)
	// json.Marshal(ApprovedAssetInventory) emits this fixed envelope around the
	// already-canonical persisted asset JSON values. Commas add count-1 bytes.
	envelopeBytes := int64(len(`{"scope":,"assets":[]}`) + len(scopeJSON))
	commaBytes := size.AssetCount - 1
	if commaBytes < 0 {
		commaBytes = 0
	}
	if size.MaxPayloadBytes > limit || size.TotalPayloadBytes > limit || envelopeBytes > limit || commaBytes > limit ||
		size.TotalPayloadBytes > limit-envelopeBytes || commaBytes > limit-envelopeBytes-size.TotalPayloadBytes {
		return productasset.ErrInventoryTooLarge
	}
	return nil
}

func advanceCurrentInventoryHead(tx *gorm.DB, commit productasset.ApprovalCommit) error {
	head := ApprovedInventoryHeadRecord{TenantID: commit.TenantID, ProductKey: commit.ProductKey, TargetPlatform: commit.TargetPlatform, ActionID: commit.ActionID}
	updated := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "product_key"}, {Name: "target_platform"}},
		DoUpdates: clause.AssignmentColumns([]string{"action_id"}),
	}).Create(&head)
	if updated.Error != nil {
		return mapRepositoryError("advance approved inventory head", updated.Error)
	}
	return nil
}

func advanceVersionedInventoryHead(tx *gorm.DB, commit productasset.ApprovalCommit) error {
	head := ApprovedInventoryVersionHeadRecord{
		TenantID: commit.TenantID, ProductKey: commit.ProductKey,
		TargetPlatform:        commit.TargetPlatform,
		SourceSnapshotVersion: commit.SourceSnapshotVersion, ActionID: commit.ActionID,
	}
	updated := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "product_key"}, {Name: "target_platform"}, {Name: "source_snapshot_version"}},
		DoUpdates: clause.AssignmentColumns([]string{"action_id"}),
	}).Create(&head)
	if updated.Error != nil {
		return mapRepositoryError("advance versioned approved inventory head", updated.Error)
	}
	return nil
}

type canonicalApprovalPayload struct {
	TenantID              string                   `json:"tenant_id"`
	ProductKey            string                   `json:"product_key"`
	TargetPlatform        string                   `json:"target_platform,omitempty"`
	ActionID              string                   `json:"action_id"`
	SourceSnapshotVersion uint64                   `json:"source_snapshot_version,omitempty"`
	Assets                []canonicalApprovedAsset `json:"assets"`
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
		TenantID: commit.TenantID, ProductKey: commit.ProductKey, TargetPlatform: commit.TargetPlatform, ActionID: commit.ActionID,
		SourceSnapshotVersion: commit.SourceSnapshotVersion,
		Assets:                make([]canonicalApprovedAsset, len(commit.Assets)),
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
		productasset.ErrInventoryTooLarge,
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
		TenantID: record.TenantID, ProductKey: record.ProductKey, TargetPlatform: record.TargetPlatform, ActionID: record.ActionID,
		SourceSnapshotVersion: record.SourceSnapshotVersion,
		Assets:                []productasset.ApprovedAsset{approved},
	}
	if err := productasset.ValidateApprovalCommit(commit); err != nil {
		return fmt.Errorf("persisted payload violates domain contract: %v", err)
	}
	return nil
}

var _ productasset.Repository = (*repository)(nil)
var _ productasset.ApprovedInventoryReader = (*repository)(nil)
