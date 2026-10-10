package asset

import (
	"context"
	"encoding/hex"
	"fmt"
)

const ImageSetSelectionSchema = "product-image-selection-v1"

type ImageSetInventory struct {
	Scope InventoryScope `json:"scope"`
	// Head is the global selection CAS fence, even for a version-specific read.
	Head ImageInventoryHead `json:"head"`
	// ApprovalActionID identifies the exact receipt that owns Assets.
	ApprovalActionID string          `json:"approval_action_id"`
	Assets           []ApprovedAsset `json:"assets"`
}

type ImageSetInventoryReader interface {
	ReadImageSetInventory(context.Context, InventoryScope) (ImageSetInventory, error)
}

type ImagePresentation struct {
	Group     string `json:"group"`
	Order     int    `json:"order"`
	VariantID string `json:"variant_id,omitempty"`
}

type ImageOfficialPlacement struct {
	Group string `json:"group"`
	SKC   int    `json:"skc"`
	SKU   int    `json:"sku"`
	Type  int    `json:"type"`
	Sort  int    `json:"sort"`
	Site  string `json:"site"`
}

// GenerationEvidence references terminal facts owned by the existing generation
// and resource protocols. It neither creates nor settles a financial effect.
type GenerationEvidence struct {
	IntentID              string `json:"intent_id"`
	Fingerprint           string `json:"fingerprint"`
	SettlementProofDigest string `json:"settlement_proof_digest"`
	ArtifactHash          string `json:"artifact_hash"`
}

type ImageInventoryHead struct {
	ActionID    string `json:"action_id"`
	PayloadHash string `json:"payload_hash"`
}

type ImageSetResultBinding struct {
	RunID        string `json:"run_id"`
	PlanRevision int64  `json:"plan_revision"`
	ResultDigest string `json:"result_digest"`
}

type ImageSetSelection struct {
	Schema            string                  `json:"schema"`
	Source            SourceSelectionRequest  `json:"source"`
	ExpectedHead      ImageInventoryHead      `json:"expected_head"`
	RequirementDigest string                  `json:"requirement_digest,omitempty"`
	Results           []ImageSetResultBinding `json:"results"`
	Digest            string                  `json:"digest"`
	RequestDigest     string                  `json:"request_digest"`
	Target            *ImageSetTarget         `json:"target,omitempty"`
}

type ImageSetTarget struct {
	RecordID         string `json:"record_id,omitempty"`
	StoreID          string `json:"store_id"`
	Site             string `json:"site"`
	ApplicationID    string `json:"application_id"`
	ApplicationMode  string `json:"application_mode"`
	CategoryID       int64  `json:"category_id"`
	ProductTypeID    int64  `json:"product_type_id"`
	AttributesDigest string `json:"attributes_digest"`
	VariantsDigest   string `json:"variants_digest"`
}

func (t ImageSetTarget) Valid() bool {
	return validIdentityPart(t.RecordID) && validIdentityPart(t.StoreID) && validIdentityPart(t.Site) && validIdentityPart(t.ApplicationID) && validIdentityPart(t.ApplicationMode) && t.CategoryID > 0 && t.ProductTypeID > 0 && imageSetDigest(t.AttributesDigest) && imageSetDigest(t.VariantsDigest)
}

func (s ImageInventoryHead) Valid() bool {
	return s == (ImageInventoryHead{}) || validIdentityPart(s.ActionID) && imageSetDigest(s.PayloadHash)
}
func imageSetDigest(value string) bool {
	bytes, err := hex.DecodeString(value)
	return err == nil && len(bytes) == 32 && hex.EncodeToString(bytes) == value
}

func ImageSetSelectionDigest(commit ApprovalCommit) string {
	copy := CloneApprovalCommit(commit)
	if copy.ImageSet != nil {
		copy.ImageSet.Digest = ""
	}
	return approvalDigest(copy)
}

func ValidateImageSetCommit(commit ApprovalCommit) error {
	s := commit.ImageSet
	if s == nil || s.Schema != ImageSetSelectionSchema || !s.ExpectedHead.Valid() || len(commit.Assets) < 1 || len(commit.Assets) > 40 || s.Digest != ImageSetSelectionDigest(commit) || !imageSetDigest(s.RequestDigest) || s.Source.TargetPlatform != commit.TargetPlatform || s.Source.EffectiveCatalogVersion != commit.SourceSnapshotVersion || !validIdentityPart(s.Source.ItemID) || !validIdentityPart(s.Source.OriginalPublicationID) || s.Source.OriginalSnapshotVersion == 0 {
		return ErrInvalidApproval
	}
	if commit.TargetPlatform != "product" && (!imageSetDigest(s.RequirementDigest) || s.Target == nil || !s.Target.Valid()) || commit.TargetPlatform == "product" && (s.RequirementDigest != "" || s.Target != nil) || s.Source.ApplyReceiptID != "" && !validIdentityPart(s.Source.ApplyReceiptID) {
		return ErrInvalidApproval
	}
	if len(s.Results) > 40 {
		return ErrInvalidApproval
	}
	resultIDs := map[string]bool{}
	for _, result := range s.Results {
		key := fmt.Sprintf("%s/%d", result.RunID, result.PlanRevision)
		if !validIdentityPart(result.RunID) || result.PlanRevision < 1 || !imageSetDigest(result.ResultDigest) || resultIDs[key] {
			return ErrInvalidApproval
		}
		resultIDs[key] = true
	}
	origins := map[string]bool{}
	positions := map[string]bool{}
	officialPositions := map[string]bool{}
	selectedResults := map[string]bool{}
	counts := map[string]int{}
	maxOrder := map[string]int{}
	for _, a := range commit.Assets {
		if a.Width <= 0 || a.Height <= 0 {
			return ErrInvalidApproval
		}
		p := a.Presentation
		if p == nil || (p.Group != "carousel" && p.Group != "detail") || p.Order < 1 || p.Order > 40 || p.VariantID != "" && !validIdentityPart(p.VariantID) {
			return ErrInvalidApproval
		}
		group := p.Group + "/" + p.VariantID
		position := fmt.Sprintf("%s/%d", group, p.Order)
		if positions[position] || origins[a.ApprovalIdentity()] {
			return ErrInvalidApproval
		}
		positions[position] = true
		origins[a.ApprovalIdentity()] = true
		counts[group]++
		if p.Order > maxOrder[group] {
			maxOrder[group] = p.Order
		}
		if commit.TargetPlatform == "product" && a.OfficialPlacement != nil || commit.TargetPlatform != "product" && a.OfficialPlacement == nil {
			return ErrInvalidApproval
		}
		if official := a.OfficialPlacement; official != nil {
			key := fmt.Sprintf("%s/%s/%d/%d/%d", official.Site, official.Group, official.SKC, official.SKU, official.Sort)
			if !validIdentityPart(official.Group) || official.Site != s.Target.Site || official.SKC < 0 || official.SKU < 0 || official.Type < 1 || official.Sort < 1 || officialPositions[key] {
				return ErrInvalidApproval
			}
			officialPositions[key] = true
		}
		if original := a.SourceApproval; original != nil {
			if original.OriginalPublicationID != s.Source.OriginalPublicationID || original.OriginalSnapshotVersion != s.Source.OriginalSnapshotVersion || a.GenerationEvidence != nil {
				return ErrInvalidApproval
			}
		}
		if a.SourceApproval == nil && a.SelectionReceipt == nil {
			g := a.GenerationEvidence
			if g == nil || !imageSetDigest(g.IntentID) || !imageSetDigest(g.Fingerprint) || !imageSetDigest(g.SettlementProofDigest) || !imageSetDigest(g.ArtifactHash) {
				return ErrInvalidApproval
			}
			key := fmt.Sprintf("%s/%d", a.RunID, a.PlanRevision)
			if !resultIDs[key] {
				return ErrInvalidApproval
			}
			selectedResults[key] = true
		}
	}
	if len(selectedResults) != len(resultIDs) {
		return ErrInvalidApproval
	}
	for group, count := range counts {
		if maxOrder[group] != count {
			return ErrInvalidApproval
		}
	}
	return nil
}

func CloneImageSetSelection(s *ImageSetSelection) *ImageSetSelection {
	if s == nil {
		return nil
	}
	copy := *s
	copy.Results = append([]ImageSetResultBinding(nil), s.Results...)
	if s.Target != nil {
		target := *s.Target
		copy.Target = &target
	}
	return &copy
}
