package asset

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type ImageSetChoice struct {
	Kind              string                  `json:"kind"`
	SourceID          string                  `json:"source_id,omitempty"`
	ApprovalActionID  string                  `json:"approval_action_id,omitempty"`
	AssetID           string                  `json:"asset_id,omitempty"`
	RunID             string                  `json:"run_id,omitempty"`
	PlanRevision      int64                   `json:"plan_revision,omitempty"`
	SlotID            string                  `json:"slot_id,omitempty"`
	Attempt           int                     `json:"attempt,omitempty"`
	ResultDigest      string                  `json:"result_digest,omitempty"`
	Presentation      ImagePresentation       `json:"presentation"`
	OfficialPlacement *ImageOfficialPlacement `json:"official_placement,omitempty"`
}

type ImageSetCommand struct {
	ActionID        string                 `json:"action_id"`
	Source          SourceSelectionRequest `json:"source"`
	Target          *ImageSetTarget        `json:"target,omitempty"`
	ExpectedHead    ImageInventoryHead     `json:"expected_head"`
	Choices         []ImageSetChoice       `json:"choices"`
	SelectionDigest string                 `json:"selection_digest"`
}

type ImageSetCandidate struct {
	Asset  ApprovedAsset
	Result ImageSetResultBinding
}
type ImageCandidateSelectionReader interface {
	ReadImageSetCandidate(context.Context, SourceSelection, ImageSetChoice) (ImageSetCandidate, error)
}

type ImageSetTargetResolution struct {
	Target            *ImageSetTarget
	RequirementDigest string
	Placements        []*ImageOfficialPlacement
}
type ImageSetTargetResolver interface {
	ResolveImageSetTarget(context.Context, SourceSelection, *ImageSetTarget, []ApprovedAsset) (ImageSetTargetResolution, error)
}

type ImageSetPreview struct {
	Digest string             `json:"digest"`
	Head   ImageInventoryHead `json:"head"`
	Assets []ApprovedAsset    `json:"assets"`
}

type ImageSetService struct {
	sources     SourceSelectionReader
	repository  Repository
	inventories ImageSetInventoryReader
	approvals   ApprovalCommitReader
	candidates  ImageCandidateSelectionReader
	targets     ImageSetTargetResolver
}

func NewImageSetService(sources SourceSelectionReader, repository Repository, inventories ImageSetInventoryReader, approvals ApprovalCommitReader, candidates ImageCandidateSelectionReader, targets ImageSetTargetResolver) (*ImageSetService, error) {
	if sources == nil || repository == nil || inventories == nil || approvals == nil || candidates == nil {
		return nil, ErrRepositoryUnavailable
	}
	return &ImageSetService{sources, repository, inventories, approvals, candidates, targets}, nil
}

func (s *ImageSetService) Preview(ctx context.Context, input ImageSetCommand) (ImageSetPreview, error) {
	commit, err := s.prepare(ctx, input)
	if err != nil {
		return ImageSetPreview{}, err
	}
	return ImageSetPreview{Digest: commit.ImageSet.Digest, Head: commit.ImageSet.ExpectedHead, Assets: commit.Assets}, nil
}

func (s *ImageSetService) Select(ctx context.Context, input ImageSetCommand) (ApprovalReceipt, error) {
	if !imageSetDigest(input.SelectionDigest) {
		return ApprovalReceipt{}, ErrInvalidApproval
	}
	commit, err := s.prepare(ctx, input)
	if err != nil {
		return ApprovalReceipt{}, err
	}
	if commit.ImageSet.Digest != input.SelectionDigest {
		return ApprovalReceipt{}, ErrApprovalConflict
	}
	return s.repository.CommitApproval(ctx, commit)
}

func imageSetRequestDigest(input ImageSetCommand) string {
	input.SelectionDigest = ""
	return approvalDigest(input)
}

func (s *ImageSetService) prepare(ctx context.Context, input ImageSetCommand) (ApprovalCommit, error) {
	if ctx == nil || !validIdentityPart(input.ActionID) || len(input.Choices) < 1 || len(input.Choices) > 40 || !input.ExpectedHead.Valid() {
		return ApprovalCommit{}, ErrInvalidApproval
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	source, err := s.sources.ReadSourceSelection(ctx, input.Source)
	if err != nil {
		return ApprovalCommit{}, err
	}
	if source.ItemID != input.Source.ItemID || source.OriginalPublicationID != input.Source.OriginalPublicationID || source.OriginalSnapshotVersion != input.Source.OriginalSnapshotVersion || source.EffectiveCatalogVersion != input.Source.EffectiveCatalogVersion || source.ApplyReceiptID != input.Source.ApplyReceiptID || source.TargetPlatform != input.Source.TargetPlatform {
		return ApprovalCommit{}, ErrApprovalConflict
	}
	if !validIdentityPart(source.TenantID) || !validIdentityPart(source.ActorID) || !validIdentityPart(source.MemberID) || !validIdentityPart(source.ProductKey) {
		return ApprovalCommit{}, ErrSourceApprovalForbidden
	}
	requestDigest := imageSetRequestDigest(input)
	existing, err := s.approvals.ReadApprovalCommit(ctx, source.TenantID, input.ActionID)
	if err == nil {
		if existing.ImageSet == nil || existing.ImageSet.RequestDigest != requestDigest || existing.ProductKey != source.ProductKey || existing.TargetPlatform != source.TargetPlatform || existing.SourceSnapshotVersion != source.EffectiveCatalogVersion {
			return ApprovalCommit{}, ErrApprovalConflict
		}
		return existing, nil
	}
	if !errors.Is(err, ErrApprovedAssetsNotReady) {
		return ApprovalCommit{}, err
	}
	inventory, err := s.inventories.ReadImageSetInventory(ctx, InventoryScope{TenantID: source.TenantID, ProductKey: source.ProductKey, TargetPlatform: source.TargetPlatform, SourceSnapshotVersion: source.EffectiveCatalogVersion})
	if err != nil {
		return ApprovalCommit{}, err
	}
	if inventory.Head != input.ExpectedHead {
		return ApprovalCommit{}, ErrApprovalConflict
	}
	commit := ApprovalCommit{TenantID: source.TenantID, ProductKey: source.ProductKey, TargetPlatform: source.TargetPlatform, ActionID: input.ActionID, SourceSnapshotVersion: source.EffectiveCatalogVersion, ImageSet: &ImageSetSelection{Schema: ImageSetSelectionSchema, Source: input.Source, ExpectedHead: input.ExpectedHead, RequestDigest: requestDigest}}
	seenResults := map[string]string{}
	for index, choice := range input.Choices {
		asset, result, err := s.resolveChoice(ctx, source, inventory, choice)
		if err != nil {
			return ApprovalCommit{}, err
		}
		asset.ID = "selected-" + approvalDigest([]any{source.TenantID, input.ActionID, index, choice})
		position := choice.Presentation
		asset.Presentation = &position
		asset.OfficialPlacement = nil
		if choice.OfficialPlacement != nil {
			if source.TargetPlatform == "product" {
				return ApprovalCommit{}, ErrInvalidApproval
			}
			requested := *choice.OfficialPlacement
			asset.OfficialPlacement = &requested
		}
		commit.Assets = append(commit.Assets, asset)
		if result.RunID != "" {
			key := fmt.Sprintf("%s/%d", result.RunID, result.PlanRevision)
			if original := seenResults[key]; original != "" && original != result.ResultDigest {
				return ApprovalCommit{}, ErrApprovalConflict
			}
			if seenResults[key] == "" {
				commit.ImageSet.Results = append(commit.ImageSet.Results, result)
				seenResults[key] = result.ResultDigest
			}
		}
	}
	if source.TargetPlatform == "product" {
		if input.Target != nil {
			return ApprovalCommit{}, ErrInvalidApproval
		}
	} else {
		if s.targets == nil || input.Target == nil {
			return ApprovalCommit{}, ErrApprovedAssetsNotReady
		}
		resolved, err := s.targets.ResolveImageSetTarget(ctx, source, input.Target, commit.Assets)
		if err != nil {
			return ApprovalCommit{}, err
		}
		if resolved.Target == nil || !imageSetDigest(resolved.RequirementDigest) || len(resolved.Placements) != len(commit.Assets) {
			return ApprovalCommit{}, ErrRepositoryStateInvalid
		}
		commit.ImageSet.Target = resolved.Target
		commit.ImageSet.RequirementDigest = resolved.RequirementDigest
		for i, position := range resolved.Placements {
			commit.Assets[i].OfficialPlacement = position
		}
	}
	commit.ImageSet.Digest = ImageSetSelectionDigest(commit)
	if err := ValidateApprovalCommit(commit); err != nil {
		return ApprovalCommit{}, err
	}
	return commit, nil
}

func (s *ImageSetService) resolveChoice(ctx context.Context, source SourceSelection, inventory ImageSetInventory, choice ImageSetChoice) (ApprovedAsset, ImageSetResultBinding, error) {
	bad := func() (ApprovedAsset, ImageSetResultBinding, error) {
		return ApprovedAsset{}, ImageSetResultBinding{}, ErrInvalidApproval
	}
	if choice.Kind == "source" {
		if !validIdentityPart(choice.SourceID) || choice.AssetID != "" || choice.RunID != "" || choice.ApprovalActionID != "" || choice.PlanRevision != 0 || choice.Attempt != 0 || choice.SlotID != "" || choice.ResultDigest != "" {
			return bad()
		}
		for _, image := range source.Images {
			if image.ID == choice.SourceID && image.ReferenceHash == ReferenceHash(image.ID, image.URL) {
				return ApprovedAsset{Role: RoleGallery, URL: image.URL, SourceAssetID: image.ID, Width: image.Width, Height: image.Height, SourceApproval: &SourceApprovalProvenance{OriginalPublicationID: source.OriginalPublicationID, OriginalSnapshotVersion: source.OriginalSnapshotVersion, ActorID: source.ActorID, MemberID: source.MemberID, ReferenceHash: image.ReferenceHash}}, ImageSetResultBinding{}, nil
			}
		}
		return bad()
	}
	if choice.Kind == "approved" {
		if !validIdentityPart(choice.ApprovalActionID) || !validIdentityPart(choice.AssetID) || choice.SourceID != "" || choice.RunID != "" || choice.PlanRevision != 0 || choice.Attempt != 0 || choice.SlotID != "" || choice.ResultDigest != "" {
			return bad()
		}
		approved, err := s.approvals.ReadApprovalCommit(ctx, source.TenantID, choice.ApprovalActionID)
		if err != nil {
			return ApprovedAsset{}, ImageSetResultBinding{}, err
		}
		if approved.ProductKey != source.ProductKey || approved.TargetPlatform != source.TargetPlatform || approved.SourceSnapshotVersion != source.EffectiveCatalogVersion {
			return ApprovedAsset{}, ImageSetResultBinding{}, ErrApprovalConflict
		}
		for _, asset := range approved.Assets {
			if asset.ID == choice.AssetID {
				for _, current := range inventory.Assets {
					if current.ID == asset.ID || current.SelectionReceipt != nil && current.SelectionReceipt.ActionID == choice.ApprovalActionID && current.SelectionReceipt.AssetID == asset.ID {
						if current.ApprovalIdentity() != asset.ApprovalIdentity() || current.URL != asset.URL {
							return ApprovedAsset{}, ImageSetResultBinding{}, ErrApprovalConflict
						}
						copy := CloneApprovalCommit(ApprovalCommit{Assets: []ApprovedAsset{asset}}).Assets[0]
						if copy.SourceApproval == nil {
							copy.SelectionReceipt = &SelectionReceipt{ActionID: choice.ApprovalActionID, AssetID: choice.AssetID}
						}
						return copy, ImageSetResultBinding{}, nil
					}
				}
			}
		}
		return ApprovedAsset{}, ImageSetResultBinding{}, ErrApprovalConflict
	}
	if choice.Kind != "generated" || choice.SourceID != "" || choice.ApprovalActionID != "" || !validIdentityPart(choice.AssetID) || !validIdentityPart(choice.RunID) || !validIdentityPart(choice.SlotID) || choice.PlanRevision < 1 || choice.Attempt < 1 || !imageSetDigest(choice.ResultDigest) {
		return bad()
	}
	resolved, err := s.candidates.ReadImageSetCandidate(ctx, source, choice)
	if err != nil {
		return ApprovedAsset{}, ImageSetResultBinding{}, err
	}
	a := resolved.Asset
	if a.SourceApproval != nil || a.SelectionReceipt != nil || a.ID != choice.AssetID || a.RunID != choice.RunID || a.SlotID != choice.SlotID || a.PlanRevision != choice.PlanRevision || a.Attempt != choice.Attempt || resolved.Result.RunID != choice.RunID || resolved.Result.PlanRevision != choice.PlanRevision || resolved.Result.ResultDigest != choice.ResultDigest {
		return ApprovedAsset{}, ImageSetResultBinding{}, ErrApprovalConflict
	}
	return CloneApprovalCommit(ApprovalCommit{Assets: []ApprovedAsset{a}}).Assets[0], resolved.Result, nil
}
