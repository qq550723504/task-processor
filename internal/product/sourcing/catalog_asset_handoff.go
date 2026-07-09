package sourcing

import (
	"task-processor/internal/asset"
	"task-processor/internal/catalog"
)

// CatalogProductFactsFromEnvelope converts a normalized source envelope into
// platform-neutral catalog facts. It intentionally keeps ListingKit and target
// marketplace payload decisions out of the handoff.
func CatalogProductFactsFromEnvelope(envelope SourceEnvelope) catalog.ProductFacts {
	envelope = envelope.Normalize()
	identity := envelope.Identity
	candidate := envelope.ProductCandidate

	return catalog.ProductFacts{
		SourceKey:      identity.SourceKey(),
		SourceType:     identity.SourceType,
		SourcePlatform: identity.SourcePlatform,
		SourceID:       identity.SourceID,
		SourceURL:      identity.SourceURL,
		Title:          candidate.Title,
		Description:    candidate.Description,
		Brand:          candidate.Brand,
		Attributes:     copyStringMap(candidate.Attributes),
		Variants:       catalogVariantFacts(candidate.Variants),
		Warnings:       catalogWarnings(envelope.Warnings),
	}
}

// AssetFactsFromEnvelope converts a normalized source envelope into
// platform-neutral asset facts. It preserves source lineage and warnings without
// making product sourcing own downstream asset storage or marketplace usage.
func AssetFactsFromEnvelope(envelope SourceEnvelope) asset.Facts {
	envelope = envelope.Normalize()
	identity := envelope.Identity

	return asset.Facts{
		SourceKey:      identity.SourceKey(),
		SourceType:     identity.SourceType,
		SourcePlatform: identity.SourcePlatform,
		SourceID:       identity.SourceID,
		Items:          assetItemFacts(envelope.AssetCandidates),
		Warnings:       assetWarnings(envelope.Warnings),
	}
}

func catalogVariantFacts(candidates []ProductVariantCandidate) []catalog.VariantFacts {
	if len(candidates) == 0 {
		return nil
	}
	facts := make([]catalog.VariantFacts, 0, len(candidates))
	for _, candidate := range candidates {
		facts = append(facts, catalog.VariantFacts{
			SourceID:   candidate.SourceID,
			Title:      candidate.Title,
			SKU:        candidate.SKU,
			Attributes: copyStringMap(candidate.Attributes),
		})
	}
	return facts
}

func assetItemFacts(candidates []AssetCandidate) []asset.ItemFacts {
	if len(candidates) == 0 {
		return nil
	}
	facts := make([]asset.ItemFacts, 0, len(candidates))
	for _, candidate := range candidates {
		facts = append(facts, asset.ItemFacts{
			SourceID:  candidate.SourceID,
			URL:       candidate.URL,
			MediaType: candidate.MediaType,
			Role:      candidate.Role,
			Checksum:  candidate.Checksum,
		})
	}
	return facts
}

func catalogWarnings(warnings []SourceWarning) []catalog.FactWarning {
	if len(warnings) == 0 {
		return nil
	}
	out := make([]catalog.FactWarning, 0, len(warnings))
	for _, warning := range warnings {
		warning = warning.Normalize()
		out = append(out, catalog.FactWarning{Code: warning.Code, Message: warning.Message, Field: warning.Field})
	}
	return out
}

func assetWarnings(warnings []SourceWarning) []asset.FactWarning {
	if len(warnings) == 0 {
		return nil
	}
	out := make([]asset.FactWarning, 0, len(warnings))
	for _, warning := range warnings {
		warning = warning.Normalize()
		out = append(out, asset.FactWarning{Code: warning.Code, Message: warning.Message, Field: warning.Field})
	}
	return out
}

func copyStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
