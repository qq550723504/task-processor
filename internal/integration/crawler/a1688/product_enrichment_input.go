package a1688

import (
	"task-processor/internal/product/enrichment"
	"task-processor/internal/product/sourcing"
)

// ProductEnrichmentRequest maps an already-migrated source envelope into the
// pure enrichment input. It performs no crawl, provider selection, persistence,
// or task submission.
func ProductEnrichmentRequest(source sourcing.SourceEnvelope, policy enrichment.PolicySnapshot) (enrichment.Request, error) {
	normalized, err := sourcing.Normalize(source)
	if err != nil {
		return enrichment.Request{}, enrichment.ErrInputInvalid
	}
	snapshot, err := sourcing.ToSnapshot(normalized)
	if err != nil {
		return enrichment.Request{}, enrichment.ErrInputInvalid
	}
	return enrichment.Request{
		Snapshot: snapshot,
		Source:   cloneEnrichmentSource(normalized),
		Policy: enrichment.PolicySnapshot{
			Version:             policy.Version,
			AllowedFields:       append([]string(nil), policy.AllowedFields...),
			RequiredFields:      append([]string(nil), policy.RequiredFields...),
			MinimumQualityScore: policy.MinimumQualityScore,
		},
	}, nil
}

func cloneEnrichmentSource(source sourcing.SourceEnvelope) sourcing.SourceEnvelope {
	out := source
	out.RawReference.Metadata = cloneEnrichmentStringMap(source.RawReference.Metadata)
	out.ProductCandidate.Attributes = cloneEnrichmentStringMap(source.ProductCandidate.Attributes)
	out.ProductCandidate.Variants = append([]sourcing.ProductVariantCandidate(nil), source.ProductCandidate.Variants...)
	for index := range out.ProductCandidate.Variants {
		out.ProductCandidate.Variants[index].Attributes = cloneEnrichmentStringMap(source.ProductCandidate.Variants[index].Attributes)
	}
	out.AssetCandidates = append([]sourcing.AssetCandidate(nil), source.AssetCandidates...)
	out.SupplierOrCostFacts.Facts = cloneEnrichmentStringMap(source.SupplierOrCostFacts.Facts)
	out.Warnings = append([]sourcing.SourceWarning(nil), source.Warnings...)
	out.Trace.Notes = append([]string(nil), source.Trace.Notes...)
	return out
}

func cloneEnrichmentStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
