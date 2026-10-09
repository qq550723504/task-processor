package asset

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"time"
)

type Origin string

const (
	OriginHumanSource Origin = "human_source"
	OriginImageAgent  Origin = "image_agent"
)

// SourceApprovalProvenance is original evidence. InventoryScope separately
// binds the complete selected set to the effective Catalog version.
type SourceApprovalProvenance struct {
	OriginalPublicationID   string `json:"original_publication_id"`
	OriginalSnapshotVersion uint64 `json:"original_snapshot_version"`
	ActorID                 string `json:"actor_id"`
	MemberID                string `json:"member_id"`
	ReferenceHash           string `json:"reference_hash"`
}
type SelectionReceipt struct {
	ActionID string `json:"action_id"`
	AssetID  string `json:"asset_id"`
}

func (a ApprovedAsset) OriginKind() Origin {
	if a.SourceApproval != nil {
		return OriginHumanSource
	}
	return OriginImageAgent
}

// ApprovalIdentity is the canonical typed identity indexed by the one owner.
func (a ApprovedAsset) ApprovalIdentity() string {
	if a.SourceApproval != nil {
		return approvalDigest([]any{a.OriginKind(), a.SourceAssetID, a.Role, a.SourceApproval})
	}
	return approvalDigest([]any{a.OriginKind(), a.RunID, a.PlanRevision, a.SlotID, a.Attempt})
}
func approvalDigest(value any) string {
	encoded, _ := json.Marshal(value)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}
func ReferenceHash(id, imageURL string) string { return approvalDigest([]string{id, imageURL}) }

type SourceSelectionRequest struct {
	ItemID                  string `json:"itemId"`
	OriginalPublicationID   string `json:"originalPublicationId"`
	OriginalSnapshotVersion uint64 `json:"originalSnapshotVersion"`
	EffectiveCatalogVersion uint64 `json:"effectiveCatalogVersion"`
	ApplyReceiptID          string `json:"applyReceiptId,omitempty"`
	TargetPlatform          string `json:"targetPlatform"`
}
type SourceImage struct {
	ID, URL, ReferenceHash string
	Width, Height          int
}
type SourceSelection struct {
	TenantID, ActorID, MemberID, ItemID, ProductKey, OriginalPublicationID, TargetPlatform string
	OriginalSnapshotVersion, EffectiveCatalogVersion                                       uint64
	Images                                                                                 []SourceImage
}

// Implemented by the current selection owner, with live actor/member access.
// This port resolves URLs from immutable evidence, never from HTTP input.
type SourceSelectionReader interface {
	ReadSourceSelection(context.Context, SourceSelectionRequest) (SourceSelection, error)
}
type SourceImageChoice struct {
	ID   string `json:"id"`
	Role Role   `json:"role"`
}
type ApprovedImageChoice struct {
	ActionID string `json:"actionId"`
	AssetID  string `json:"assetId"`
}
type SourceApprovalCommand struct {
	ActionID  string                 `json:"actionId"`
	Selection SourceSelectionRequest `json:"selection"`
	Images    []SourceImageChoice    `json:"images"`
	Approved  []ApprovedImageChoice  `json:"approved"`
}
type SourceApprovalService struct {
	selections SourceSelectionReader
	repository Repository
	approvals  ApprovalCommitReader
}

func NewSourceApprovalService(selections SourceSelectionReader, repository Repository, approvals ApprovalCommitReader) (*SourceApprovalService, error) {
	if selections == nil || repository == nil || approvals == nil {
		return nil, ErrRepositoryUnavailable
	}
	return &SourceApprovalService{selections, repository, approvals}, nil
}
func (s *SourceApprovalService) Approve(ctx context.Context, input SourceApprovalCommand) (ApprovalReceipt, error) {
	if ctx == nil || !validIdentityPart(input.ActionID) || len(input.Images)+len(input.Approved) < 1 || len(input.Images)+len(input.Approved) > 40 || !validIdentityPart(input.Selection.ItemID) || !validIdentityPart(input.Selection.OriginalPublicationID) || input.Selection.OriginalSnapshotVersion == 0 || input.Selection.EffectiveCatalogVersion == 0 || !validIdentityPart(input.Selection.TargetPlatform) {
		return ApprovalReceipt{}, ErrInvalidApproval
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	selection, err := s.selections.ReadSourceSelection(ctx, input.Selection)
	if err != nil {
		return ApprovalReceipt{}, err
	}
	if selection.ItemID != input.Selection.ItemID || selection.OriginalPublicationID != input.Selection.OriginalPublicationID || selection.OriginalSnapshotVersion != input.Selection.OriginalSnapshotVersion || selection.EffectiveCatalogVersion != input.Selection.EffectiveCatalogVersion || selection.TargetPlatform != input.Selection.TargetPlatform {
		return ApprovalReceipt{}, ErrApprovalConflict
	}
	if !validIdentityPart(selection.TenantID) || !validIdentityPart(selection.ActorID) || !validIdentityPart(selection.MemberID) || !validIdentityPart(selection.ProductKey) {
		return ApprovalReceipt{}, ErrSourceApprovalForbidden
	}
	commit := ApprovalCommit{TenantID: selection.TenantID, ProductKey: selection.ProductKey, TargetPlatform: selection.TargetPlatform, SourceSnapshotVersion: selection.EffectiveCatalogVersion, ActionID: input.ActionID}
	existing, existingErr := s.approvals.ReadApprovalCommit(ctx, selection.TenantID, input.ActionID)
	if existingErr != nil && !errors.Is(existingErr, ErrApprovedAssetsNotReady) {
		return ApprovalReceipt{}, existingErr
	}
	if existingErr == nil && (existing.ActionID != input.ActionID || existing.TenantID != selection.TenantID || existing.ProductKey != selection.ProductKey || existing.TargetPlatform != selection.TargetPlatform || existing.SourceSnapshotVersion != selection.EffectiveCatalogVersion) {
		return ApprovalReceipt{}, ErrApprovalConflict
	}
	byID := map[string]SourceImage{}
	for _, image := range selection.Images {
		if image.ID == "" || image.ReferenceHash != ReferenceHash(image.ID, image.URL) {
			return ApprovalReceipt{}, ErrRepositoryStateInvalid
		}
		if _, exists := byID[image.ID]; exists {
			return ApprovalReceipt{}, ErrRepositoryStateInvalid
		}
		byID[image.ID] = image
	}
	selectedSources := map[string]bool{}
	for _, choice := range input.Images {
		image, exists := byID[choice.ID]
		if !exists || !choice.Role.valid() || selectedSources[choice.ID] {
			return ApprovalReceipt{}, ErrInvalidApproval
		}
		selectedSources[choice.ID] = true
		approved := ApprovedAsset{ID: "source-" + approvalDigest([]string{selection.TenantID, input.ActionID, image.ID, string(choice.Role)}), Role: choice.Role, URL: image.URL, SourceAssetID: image.ID, Width: image.Width, Height: image.Height, SourceApproval: &SourceApprovalProvenance{OriginalPublicationID: selection.OriginalPublicationID, OriginalSnapshotVersion: selection.OriginalSnapshotVersion, ActorID: selection.ActorID, MemberID: selection.MemberID, ReferenceHash: image.ReferenceHash}}
		commit.Assets = append(commit.Assets, approved)
	}
	if len(input.Approved) > 0 {
		inventory := ApprovedAssetInventory{Assets: existing.Assets}
		if existingErr != nil {
			current, readErr := s.repository.GetApprovedInventory(ctx, InventoryScope{TenantID: selection.TenantID, ProductKey: selection.ProductKey, TargetPlatform: selection.TargetPlatform, SourceSnapshotVersion: selection.EffectiveCatalogVersion})
			if readErr != nil {
				return ApprovalReceipt{}, readErr
			}
			inventory = current
		}
		for _, choice := range input.Approved {
			approvedCommit, readErr := s.approvals.ReadApprovalCommit(ctx, selection.TenantID, choice.ActionID)
			if readErr != nil {
				return ApprovalReceipt{}, readErr
			}
			if approvedCommit.TenantID != selection.TenantID || approvedCommit.ProductKey != selection.ProductKey || approvedCommit.TargetPlatform != selection.TargetPlatform || approvedCommit.SourceSnapshotVersion != selection.EffectiveCatalogVersion {
				return ApprovalReceipt{}, ErrApprovalConflict
			}
			var selected *ApprovedAsset
			for _, candidate := range approvedCommit.Assets {
				if candidate.ID == choice.AssetID && candidate.OriginKind() == OriginImageAgent {
					exact := false
					for _, current := range inventory.Assets {
						if (current.ID == candidate.ID || current.SelectionReceipt != nil && current.SelectionReceipt.ActionID == choice.ActionID && current.SelectionReceipt.AssetID == candidate.ID) && current.ApprovalIdentity() == candidate.ApprovalIdentity() && current.URL == candidate.URL && current.Role == candidate.Role {
							exact = true
						}
					}
					if exact {
						copied := CloneApprovalCommit(ApprovalCommit{Assets: []ApprovedAsset{candidate}}).Assets[0]
						selected = &copied
					}
				}
			}
			if selected == nil {
				return ApprovalReceipt{}, ErrApprovalConflict
			}
			selected.SelectionReceipt = &SelectionReceipt{ActionID: choice.ActionID, AssetID: choice.AssetID}
			selected.ID = "selected-" + approvalDigest([]string{selection.TenantID, input.ActionID, choice.ActionID, choice.AssetID})
			commit.Assets = append(commit.Assets, *selected)
		}
	}
	if err := ValidateApprovalCommit(commit); err != nil {
		return ApprovalReceipt{}, err
	}
	return s.repository.CommitApproval(ctx, commit)
}
func validateSourceApproval(a ApprovedAsset) bool {
	p := a.SourceApproval
	address, err := url.Parse(a.URL)
	return p != nil && a.RunID == "" && a.PlanRevision == 0 && a.SlotID == "" && a.Attempt == 0 && a.SourceAssetID != "" &&
		validIdentityPart(p.OriginalPublicationID) && p.OriginalSnapshotVersion > 0 && validIdentityPart(p.ActorID) && validIdentityPart(p.MemberID) &&
		len(p.ReferenceHash) == 64 && p.ReferenceHash == ReferenceHash(a.SourceAssetID, a.URL) && err == nil && address.Scheme == "https" && address.Hostname() != "" && address.User == nil && address.Fragment == "" && len(a.URL) <= 2048 && a.SelectionReceipt == nil
}
