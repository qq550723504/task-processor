package enrichment

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

func validateRequest(request Request) error {
	if !canonicalRequired(request.Policy.Version) || !request.Source.Identity.Valid() {
		return ErrInputInvalid
	}
	minimumScore := request.Policy.MinimumQualityScore
	if math.IsNaN(minimumScore) || math.IsInf(minimumScore, 0) || minimumScore < 0 || minimumScore > 100 {
		return ErrInputInvalid
	}
	allowed, ok := canonicalUniqueSet(request.Policy.AllowedFields)
	if !ok {
		return ErrInputInvalid
	}
	required, ok := canonicalUniqueSet(request.Policy.RequiredFields)
	if !ok {
		return ErrInputInvalid
	}
	if len(allowed) > 0 {
		for field := range required {
			if _, exists := allowed[field]; !exists {
				return ErrInputInvalid
			}
		}
	}
	return nil
}

func validateEvidence(source sourcing.SourceEnvelope, changes []FieldChange) ([]Evidence, error) {
	raw := source.RawReference
	id := firstCanonical(raw.ReferenceID, raw.SnapshotID, raw.Checksum)
	if id == "" {
		return nil, ErrEvidenceInsufficient
	}
	for i := range changes {
		change := changes[i]
		if len(change.EvidenceIDs) == 0 {
			return nil, ErrEvidenceInsufficient
		}
		for _, evidenceID := range change.EvidenceIDs {
			if evidenceID != id {
				return nil, ErrEvidenceInsufficient
			}
		}
		changes[i].EvidenceIDs = []string{id}
	}
	return []Evidence{{
		ID:          id,
		ReferenceID: raw.ReferenceID,
		SnapshotID:  raw.SnapshotID,
		Checksum:    raw.Checksum,
		URL:         raw.URL,
		CapturedAt:  raw.CapturedAt,
		Metadata:    cloneStringMap(raw.Metadata),
	}}, nil
}

func validateProposal(proposal *Proposal, policy PolicySnapshot) error {
	if len(proposal.Changes) == 0 {
		proposal.Validation.Valid = false
		return ErrOutputValidation
	}
	for _, warning := range proposal.Warnings {
		if !canonicalRequired(warning.Code) || !canonicalRequired(warning.Message) {
			proposal.Validation.Valid = false
			return ErrOutputValidation
		}
	}
	for _, rejection := range proposal.Rejections {
		if !canonicalRequired(rejection.Code) || !canonicalRequired(rejection.Message) {
			proposal.Validation.Valid = false
			return ErrOutputValidation
		}
	}

	allowed, _ := canonicalUniqueSet(policy.AllowedFields)
	required, _ := canonicalUniqueSet(policy.RequiredFields)
	seen := make(map[string]struct{}, len(proposal.Changes))
	for _, change := range proposal.Changes {
		if !canonicalRequired(change.Field) || !canonicalRequired(change.Value) {
			proposal.Validation.Valid = false
			return ErrOutputValidation
		}
		if _, duplicate := seen[change.Field]; duplicate {
			proposal.Validation.Valid = false
			return ErrOutputValidation
		}
		seen[change.Field] = struct{}{}
		if len(allowed) > 0 {
			if _, permitted := allowed[change.Field]; !permitted {
				proposal.Rejections = append(proposal.Rejections, Rejection{
					Code:    "field_not_allowed",
					Field:   change.Field,
					Message: "field change is not allowed by policy",
				})
			}
		}
	}
	for field := range required {
		if _, exists := seen[field]; !exists {
			proposal.Rejections = append(proposal.Rejections, Rejection{
				Code:    "required_field_missing",
				Field:   field,
				Message: "required field change is missing",
			})
		}
	}
	if proposal.Quality.Overall < policy.MinimumQualityScore {
		proposal.Rejections = append(proposal.Rejections, Rejection{
			Code:    "quality_below_minimum",
			Message: "proposal quality is below policy minimum",
		})
	}
	proposal.Rejections = stableRejections(proposal.Rejections)
	proposal.Validation.Valid = len(proposal.Rejections) == 0
	if !proposal.Validation.Valid {
		return ErrPolicyRejected
	}
	return nil
}

func canonicalRequired(value string) bool {
	return value != "" && value == strings.TrimSpace(value)
}

func firstCanonical(values ...string) string {
	for _, value := range values {
		if canonicalRequired(value) {
			return value
		}
	}
	return ""
}

func canonicalUniqueSet(values []string) (map[string]struct{}, bool) {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !canonicalRequired(value) {
			return nil, false
		}
		if _, duplicate := set[value]; duplicate {
			return nil, false
		}
		set[value] = struct{}{}
	}
	return set, true
}

func stableWarnings(items []Warning) []Warning {
	if len(items) == 0 {
		return nil
	}
	out := make([]Warning, len(items))
	for i, item := range items {
		out[i] = Warning{
			Code:     strings.ToLower(strings.TrimSpace(item.Code)),
			Field:    strings.TrimSpace(item.Field),
			Message:  strings.TrimSpace(item.Message),
			Metadata: cloneStringMap(item.Metadata),
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return warningKey(out[i]) < warningKey(out[j])
	})
	return out
}

func stableRejections(items []Rejection) []Rejection {
	if len(items) == 0 {
		return nil
	}
	out := make([]Rejection, len(items))
	for i, item := range items {
		out[i] = Rejection{
			Code:     strings.ToLower(strings.TrimSpace(item.Code)),
			Field:    strings.TrimSpace(item.Field),
			Message:  strings.TrimSpace(item.Message),
			Metadata: cloneStringMap(item.Metadata),
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return rejectionKey(out[i]) < rejectionKey(out[j])
	})
	return out
}

func warningKey(item Warning) string {
	return fmt.Sprintf("%s\x00%s\x00%s", item.Code, item.Field, item.Message)
}

func rejectionKey(item Rejection) string {
	return fmt.Sprintf("%s\x00%s\x00%s", item.Code, item.Field, item.Message)
}

func cloneRequest(request Request) Request {
	return Request{
		Snapshot: cloneSnapshot(request.Snapshot),
		Source:   cloneSource(request.Source),
		Policy:   clonePolicy(request.Policy),
	}
}

func cloneSnapshot(snapshot catalog.ProductSnapshot) catalog.ProductSnapshot {
	out := snapshot
	out.CategoryPath = cloneStrings(snapshot.CategoryPath)
	out.SellingPoints = cloneStrings(snapshot.SellingPoints)
	out.SEOKeywords = cloneStrings(snapshot.SEOKeywords)
	out.Attributes = cloneAttributes(snapshot.Attributes)
	out.Specifications = cloneSpecifications(snapshot.Specifications)
	out.Variants = cloneVariants(snapshot.Variants)
	out.Images = cloneImages(snapshot.Images)
	if snapshot.Review != nil {
		review := *snapshot.Review
		review.Reasons = cloneStrings(snapshot.Review.Reasons)
		out.Review = &review
	}
	out.Sources = append([]catalog.SourceRecord(nil), snapshot.Sources...)
	return out
}

func cloneAttributes(items []catalog.Attribute) []catalog.Attribute {
	if len(items) == 0 {
		return nil
	}
	out := make([]catalog.Attribute, len(items))
	for i, item := range items {
		out[i] = item
		out[i].Trace.Sources = append([]catalog.SourceRecord(nil), item.Trace.Sources...)
	}
	return out
}

func cloneSpecifications(specifications *catalog.Specifications) *catalog.Specifications {
	if specifications == nil {
		return nil
	}
	out := *specifications
	if specifications.Dimensions != nil {
		value := *specifications.Dimensions
		out.Dimensions = &value
	}
	if specifications.Weight != nil {
		value := *specifications.Weight
		out.Weight = &value
	}
	if specifications.Package != nil {
		value := *specifications.Package
		if specifications.Package.Dimensions != nil {
			dimensions := *specifications.Package.Dimensions
			value.Dimensions = &dimensions
		}
		if specifications.Package.Weight != nil {
			weight := *specifications.Package.Weight
			value.Weight = &weight
		}
		out.Package = &value
	}
	out.Technical = cloneStringMap(specifications.Technical)
	return &out
}

func cloneVariants(items []catalog.Variant) []catalog.Variant {
	if len(items) == 0 {
		return nil
	}
	out := make([]catalog.Variant, len(items))
	for i, item := range items {
		out[i] = item
		out[i].Attributes = cloneAttributes(item.Attributes)
		if item.Price != nil {
			price := *item.Price
			out[i].Price = &price
		}
		out[i].Images = cloneImages(item.Images)
		out[i].Trace.Sources = append([]catalog.SourceRecord(nil), item.Trace.Sources...)
	}
	return out
}

func cloneImages(items []catalog.Image) []catalog.Image {
	if len(items) == 0 {
		return nil
	}
	out := make([]catalog.Image, len(items))
	for i, item := range items {
		out[i] = item
		out[i].Trace.Sources = append([]catalog.SourceRecord(nil), item.Trace.Sources...)
	}
	return out
}

func cloneSource(source sourcing.SourceEnvelope) sourcing.SourceEnvelope {
	out := source
	out.RawReference.Metadata = cloneStringMap(source.RawReference.Metadata)
	out.ProductCandidate.Attributes = cloneStringMap(source.ProductCandidate.Attributes)
	out.ProductCandidate.Variants = append([]sourcing.ProductVariantCandidate(nil), source.ProductCandidate.Variants...)
	for i := range out.ProductCandidate.Variants {
		out.ProductCandidate.Variants[i].Attributes = cloneStringMap(source.ProductCandidate.Variants[i].Attributes)
	}
	out.AssetCandidates = append([]sourcing.AssetCandidate(nil), source.AssetCandidates...)
	out.SupplierOrCostFacts.Facts = cloneStringMap(source.SupplierOrCostFacts.Facts)
	out.Warnings = append([]sourcing.SourceWarning(nil), source.Warnings...)
	out.Trace.Notes = cloneStrings(source.Trace.Notes)
	return out
}

func clonePolicy(policy PolicySnapshot) PolicySnapshot {
	out := policy
	out.AllowedFields = cloneStrings(policy.AllowedFields)
	out.RequiredFields = cloneStrings(policy.RequiredFields)
	return out
}

func cloneCandidate(candidate Candidate) Candidate {
	return Candidate{
		Changes:    cloneFieldChanges(candidate.Changes),
		Warnings:   stableWarnings(candidate.Warnings),
		Rejections: stableRejections(candidate.Rejections),
	}
}

func cloneFieldChanges(items []FieldChange) []FieldChange {
	if len(items) == 0 {
		return nil
	}
	out := make([]FieldChange, len(items))
	for i, item := range items {
		out[i] = item
		out[i].EvidenceIDs = cloneStrings(item.EvidenceIDs)
	}
	return out
}

func cloneEvidence(items []Evidence) []Evidence {
	if len(items) == 0 {
		return nil
	}
	out := make([]Evidence, len(items))
	for i, item := range items {
		out[i] = item
		out[i].Metadata = cloneStringMap(item.Metadata)
	}
	return out
}

func cloneStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	return append([]string(nil), items...)
}

func cloneStringMap(items map[string]string) map[string]string {
	if len(items) == 0 {
		return nil
	}
	out := make(map[string]string, len(items))
	for key, value := range items {
		out[key] = value
	}
	return out
}
