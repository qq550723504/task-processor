package imageagent

import (
	"fmt"
	"math"
	"strings"

	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
)

const ImageSetSchema = "product-image-set-v1"

type ImageSourceBinding struct {
	ProductID, OperationID, OriginalPublicationID string
	OriginalVersion, EffectiveVersion             uint64
	CatalogHash                                   string
}

// Target is an exact context, not a declaration that the generated image is
// publishable. The rule owner supplies its immutable requirement reference.
type ImageTarget struct {
	Platform, StoreID, Site, ApplicationID, ApplicationMode                 string
	CategoryID, ProductTypeID                                               int64
	AttributesDigest, VariantsDigest, RequirementDigest, RequirementVersion string
}

type ImageSetPlan struct {
	Schema                                                         string
	Source                                                         ImageSourceBinding
	Target                                                         ImageTarget
	Configuration                                                  agent.ConfigurationSnapshotRef
	ConfigurationEpoch, ParametersDigest, InputDigest, QuoteDigest string
	MaxPoints                                                      int64
}

type ImagePlacement struct {
	Group     string
	Order     int
	VariantID string `json:",omitempty"`
}

// Official placement remains independent of the UI presentation group.
type OfficialImagePlacement struct {
	Group                string
	SKC, SKU, Type, Sort int
	Site                 string
}

type ImageSourceObservation struct {
	AssetID, SHA256, MediaType string
	Bytes                      int64
	Width, Height              int
}

type ImageGenerationQuote struct {
	Provider, Model, Protocol, Resolution, Quality            string
	PriceVersion                                              string
	Points                                                    int64
	RouteReference, CredentialReference, ConfigurationVersion string
}

type ImageSlotRecipe struct {
	Purpose, Background, Language string
	Placement                     ImagePlacement
	OfficialPlacement             *OfficialImagePlacement `json:",omitempty"`
	PromptVersion, Prompt         string
	EvidenceDigest                string `json:",omitempty"`
	References                    []ImageSourceObservation
	Quote                         ImageGenerationQuote
}

type ImageGenerationProof struct {
	IntentID, Fingerprint, SettlementProofDigest string
	Points                                       int64
}

// A closure references the existing generation/resource owners. It is not a
// second settlement record and must be resolved by the worker/candidate reader.
type ImageSlotClosure struct {
	Kind                                         string
	IntentID, Fingerprint, SettlementProofDigest string
	Points                                       int64
}

func (q ImageGenerationQuote) Valid() bool {
	return q.Provider == "grsai" && q.Model == "gpt-image-2.5" && q.Protocol == "grsai-json-sync-v1" && q.Resolution == "1024x1024" && q.Quality == "auto" && q.Points > 0 && canonicalImageValue(q.PriceVersion) && canonicalImageValue(q.RouteReference) && canonicalImageValue(q.CredentialReference) && canonicalImageValue(q.ConfigurationVersion)
}

func canonicalImageValue(value string) bool {
	return value != "" && value == strings.TrimSpace(value) && len(value) <= 192 && agentconfig.ValidSetText(value, 192)
}

func SlotRoleForImagePurpose(purpose string) SlotRole {
	switch purpose {
	case "product_identity":
		return SlotRoleMain
	case "usage_scene", "use_scenario":
		return SlotRoleScene
	case "purchase_reason", "need_solution", "choice_reason", "key_benefits":
		return SlotRoleSellingPoint
	case "purchase_specs", "specification_dimensions":
		return SlotRoleSize
	default:
		return SlotRoleDetail
	}
}

func ValidateImageSetPlan(plan Plan) error {
	s := plan.Set
	if s == nil || s.Schema != ImageSetSchema || !canonicalImageValue(s.Source.ProductID) || !canonicalImageValue(s.Source.OperationID) || !canonicalImageValue(s.Source.OriginalPublicationID) || s.Source.OriginalVersion == 0 || s.Source.EffectiveVersion == 0 || !agentconfig.ImageDigest(s.Source.CatalogHash) || s.Configuration.Kind != agentconfig.SnapshotKind || !agentconfig.UUID(s.Configuration.ID) || !agentconfig.ImageDigest(s.Configuration.Digest) || !canonicalImageValue(s.ConfigurationEpoch) || !agentconfig.ImageDigest(s.ParametersDigest) || !agentconfig.ImageDigest(s.InputDigest) || !agentconfig.ImageDigest(s.QuoteDigest) || len(plan.Slots) > MaxPlanSlots || len(plan.StyleReferenceIDs) > 0 {
		return fmt.Errorf("%w: incomplete image set binding", ErrValidation)
	}
	if s.Target.Platform != "product" {
		if !agentconfig.Platform(s.Target.Platform) || !canonicalImageValue(s.Target.StoreID) || !canonicalImageValue(s.Target.Site) || !canonicalImageValue(s.Target.ApplicationID) || !canonicalImageValue(s.Target.ApplicationMode) || s.Target.CategoryID <= 0 || s.Target.ProductTypeID <= 0 || !agentconfig.ImageDigest(s.Target.AttributesDigest) || !agentconfig.ImageDigest(s.Target.VariantsDigest) || !agentconfig.ImageDigest(s.Target.RequirementDigest) || !canonicalImageValue(s.Target.RequirementVersion) {
			return fmt.Errorf("%w: target image requirements are unavailable", ErrValidation)
		}
	} else if s.Target != (ImageTarget{Platform: "product"}) {
		return fmt.Errorf("%w: generic material cannot claim platform requirements", ErrValidation)
	}
	positions := map[string]bool{}
	for _, slot := range plan.Slots {
		r := slot.Recipe
		if r == nil || !r.Quote.Valid() || r.PromptVersion != ImageSetSchema || strings.TrimSpace(r.Prompt) == "" || !agentconfig.ValidSetText(r.Prompt, 16<<10) || !agentconfig.ValidSetText(r.Background, agentconfig.MaxSetBackgroundBytes) || strings.TrimSpace(r.Background) == "" || !agentconfig.ValidSetText(slot.Brief, agentconfig.MaxSetBriefBytes) || len(slot.StyleReferenceIDs) > 0 || len(r.References) < 1 || len(r.References) > agentconfig.MaxSetSourceReferences || len(r.References) != len(slot.SourceAssetIDs) {
			return fmt.Errorf("%w: invalid image slot recipe", ErrValidation)
		}
		if r.Placement.Group != "carousel" && r.Placement.Group != "detail" || r.Placement.Order < 1 || r.Placement.Order > MaxPlanSlots || r.Placement.VariantID != "" && !canonicalImageValue(r.Placement.VariantID) {
			return fmt.Errorf("%w: invalid image presentation position", ErrValidation)
		}
		key := fmt.Sprintf("%s/%s/%d", r.Placement.Group, r.Placement.VariantID, r.Placement.Order)
		if positions[key] {
			return fmt.Errorf("%w: repeated image presentation position", ErrValidation)
		}
		positions[key] = true
		template := agentconfig.SetTemplate{Schema: agentconfig.ImageParameterSchema, Mode: "standard", Background: r.Background, Language: r.Language}
		task := agentconfig.ContentTask{ID: slot.ID, Purpose: r.Purpose, Brief: slot.Brief}
		if r.Purpose == "custom" {
			template.Mode = "custom"
		}
		if r.Placement.Group == "carousel" {
			template.Carousel = []agentconfig.ContentTask{task}
		} else {
			template.Detail = []agentconfig.ContentTask{task}
		}
		if template.Validate() != nil || r.Purpose != "custom" && slot.Role != SlotRoleForImagePurpose(r.Purpose) {
			return fmt.Errorf("%w: image purpose does not match its recipe", ErrValidation)
		}
		if evidence := agentconfig.RequiredTaskEvidence(r.Purpose); evidence != "" && !agentconfig.ImageDigest(r.EvidenceDigest) {
			return fmt.Errorf("%w: %s evidence is required", ErrValidation, evidence)
		}
		if s.Target.Platform == "product" && r.OfficialPlacement != nil || s.Target.Platform != "product" && r.OfficialPlacement == nil {
			return fmt.Errorf("%w: platform placement must come from actual requirements", ErrValidation)
		}
		seen := map[string]bool{}
		var sourceBytes int64
		for i, ref := range r.References {
			if ref.AssetID != slot.SourceAssetIDs[i] || !canonicalImageValue(ref.AssetID) || seen[ref.AssetID] || !agentconfig.ImageDigest(ref.SHA256) || ref.Bytes <= 0 || ref.Bytes > agentconfig.MaxSetSourceAggregateBytes || ref.MediaType != "image/png" && ref.MediaType != "image/jpeg" && ref.MediaType != "image/webp" || ref.Width <= 0 || ref.Height <= 0 {
				return fmt.Errorf("%w: invalid exact source observation", ErrValidation)
			}
			seen[ref.AssetID] = true
			sourceBytes += ref.Bytes
			if sourceBytes > agentconfig.MaxSetSourceAggregateBytes {
				return fmt.Errorf("%w: source image aggregate is too large", ErrValidation)
			}
		}
	}
	points, err := ImageSetPoints(plan)
	if err != nil || points != s.MaxPoints {
		return fmt.Errorf("%w: image set points do not match the frozen slot quotes", ErrValidation)
	}
	quoteDigest, err := ImageSetQuoteDigest(plan)
	if err != nil || quoteDigest != s.QuoteDigest {
		return fmt.Errorf("%w: image set quote digest does not match", ErrRevisionConflict)
	}
	return nil
}

func ImageSetPoints(plan Plan) (int64, error) {
	var total int64
	for _, slot := range plan.Slots {
		if slot.Recipe == nil || !slot.Recipe.Quote.Valid() || total > math.MaxInt64-slot.Recipe.Quote.Points {
			return 0, ErrValidation
		}
		total += slot.Recipe.Quote.Points
	}
	if total <= 0 {
		return 0, ErrValidation
	}
	return total, nil
}

func ImageSetQuoteDigest(plan Plan) (string, error) {
	type quotedSlot struct {
		SlotID string
		Quote  ImageGenerationQuote
	}
	quotes := make([]quotedSlot, 0, len(plan.Slots))
	for _, slot := range plan.Slots {
		if slot.Recipe == nil || !slot.Recipe.Quote.Valid() {
			return "", ErrValidation
		}
		quotes = append(quotes, quotedSlot{slot.ID, slot.Recipe.Quote})
	}
	return resultDigestSHA256(quotes)
}

func ImageSetPlanDigest(plan Plan) (string, error) {
	if err := ValidateSubmittedPlan(plan); err != nil {
		return "", err
	}
	return resultDigestSHA256(plan)
}

func CloneImageSlotRecipe(recipe *ImageSlotRecipe) *ImageSlotRecipe {
	if recipe == nil {
		return nil
	}
	clone := *recipe
	clone.References = append([]ImageSourceObservation(nil), recipe.References...)
	if recipe.OfficialPlacement != nil {
		position := *recipe.OfficialPlacement
		clone.OfficialPlacement = &position
	}
	return &clone
}

func CloneImageSetPlan(set *ImageSetPlan) *ImageSetPlan {
	if set == nil {
		return nil
	}
	clone := *set
	return &clone
}
