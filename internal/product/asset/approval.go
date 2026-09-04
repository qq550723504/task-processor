package asset

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxIdentityLength is the character limit for Asset domain identity facts.
// Persistence adapters use the same limit for their indexed varchar columns.
const MaxIdentityLength = 128

type ApprovalCommit struct {
	TenantID              string          `json:"tenant_id"`
	ProductKey            string          `json:"product_key"`
	TargetPlatform        string          `json:"target_platform,omitempty"`
	ActionID              string          `json:"action_id"`
	SourceSnapshotVersion uint64          `json:"source_snapshot_version,omitempty"`
	Assets                []ApprovedAsset `json:"assets"`
}

type ApprovalReceipt struct {
	ActionID string   `json:"action_id"`
	AssetIDs []string `json:"asset_ids"`
}

func ValidateApprovalCommit(commit ApprovalCommit) error {
	if !validIdentityPart(commit.TenantID) {
		return fmt.Errorf("%w: tenant id must be non-empty and canonical", ErrInvalidApproval)
	}
	if !validIdentityPart(commit.ProductKey) {
		return fmt.Errorf("%w: product key must be non-empty and canonical", ErrInvalidApproval)
	}
	if commit.TargetPlatform != "" && !validIdentityPart(commit.TargetPlatform) {
		return fmt.Errorf("%w: target platform must be canonical when provided", ErrInvalidApproval)
	}
	if !validIdentityPart(commit.ActionID) {
		return fmt.Errorf("%w: action id must be non-empty and canonical", ErrInvalidApproval)
	}
	if len(commit.Assets) == 0 {
		return fmt.Errorf("%w: at least one approved asset is required", ErrInvalidApproval)
	}

	assetIDs := make(map[string]struct{}, len(commit.Assets))
	identities := make(map[approvalIdentity]struct{}, len(commit.Assets))
	for index, approved := range commit.Assets {
		if err := validateApprovedAsset(approved); err != nil {
			return fmt.Errorf("%w: asset %d: %v", ErrInvalidApproval, index, err)
		}
		if _, exists := assetIDs[approved.ID]; exists {
			return fmt.Errorf("%w: duplicate asset id %q", ErrInvalidApproval, approved.ID)
		}
		assetIDs[approved.ID] = struct{}{}
		identity := identityFor(commit.ActionID, approved)
		if _, exists := identities[identity]; exists {
			return fmt.Errorf("%w: duplicate run/revision/slot/attempt/action identity", ErrInvalidApproval)
		}
		identities[identity] = struct{}{}
	}
	return nil
}

func ValidateInventoryScope(scope InventoryScope) error {
	if !validIdentityPart(scope.TenantID) || !validIdentityPart(scope.ProductKey) ||
		(scope.TargetPlatform != "" && !validIdentityPart(scope.TargetPlatform)) {
		return ErrInvalidInventoryScope
	}
	return nil
}

type approvalIdentity struct {
	runID        string
	planRevision int64
	slotID       string
	attempt      int
	actionID     string
}

func identityFor(actionID string, approved ApprovedAsset) approvalIdentity {
	return approvalIdentity{
		runID:        approved.RunID,
		planRevision: approved.PlanRevision,
		slotID:       approved.SlotID,
		attempt:      approved.Attempt,
		actionID:     actionID,
	}
}

func validateApprovedAsset(approved ApprovedAsset) error {
	for name, value := range map[string]string{
		"asset id": approved.ID,
		"run id":   approved.RunID,
		"slot id":  approved.SlotID,
	} {
		if !validIdentityPart(value) {
			return fmt.Errorf("%s must be non-empty and canonical", name)
		}
	}
	if !validCanonicalExternalFact(approved.URL) {
		return errors.New("url must be non-empty and canonical")
	}
	if approved.SourceAssetID != "" && !validIdentityPart(approved.SourceAssetID) {
		return errors.New("source asset id must be canonical when provided")
	}
	if approved.PlanRevision <= 0 {
		return errors.New("plan revision must be positive")
	}
	if approved.Attempt <= 0 {
		return errors.New("attempt must be positive")
	}
	if !approved.Role.valid() {
		return fmt.Errorf("unsupported role %q", approved.Role)
	}
	if approved.Width < 0 || approved.Height < 0 {
		return errors.New("dimensions cannot be negative")
	}
	return nil
}

func validIdentityPart(value string) bool {
	return validCanonicalExternalFact(value) &&
		utf8.ValidString(value) &&
		utf8.RuneCountInString(value) <= MaxIdentityLength
}

func validCanonicalExternalFact(value string) bool {
	return value != "" && strings.TrimSpace(value) == value
}

func CloneApprovalCommit(commit ApprovalCommit) ApprovalCommit {
	out := commit
	out.Assets = cloneApprovedAssets(commit.Assets)
	return out
}

func CloneApprovalReceipt(receipt ApprovalReceipt) ApprovalReceipt {
	out := receipt
	out.AssetIDs = append([]string(nil), receipt.AssetIDs...)
	return out
}

func CloneApprovedAssetInventory(inventory ApprovedAssetInventory) ApprovedAssetInventory {
	out := inventory
	out.Assets = cloneApprovedAssets(inventory.Assets)
	return out
}

func cloneApprovedAssets(assets []ApprovedAsset) []ApprovedAsset {
	if assets == nil {
		return nil
	}
	out := make([]ApprovedAsset, len(assets))
	for index, approved := range assets {
		out[index] = approved
		if approved.Operations != nil {
			out[index].Operations = make([]string, len(approved.Operations))
			copy(out[index].Operations, approved.Operations)
		}
	}
	return out
}
