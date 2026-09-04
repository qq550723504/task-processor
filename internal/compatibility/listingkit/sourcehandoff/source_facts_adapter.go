package sourcehandoff

import (
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

// catalogProductFactsFromEnvelope converts a normalized source envelope into
// platform-neutral catalog facts. It intentionally keeps ListingKit and target
// marketplace payload decisions out of the handoff.
func catalogProductFactsFromEnvelope(envelope sourcing.SourceEnvelope) catalog.ProductFacts {
	snapshot, err := sourcing.ToSnapshot(envelope)
	if err != nil {
		return catalog.ProductFacts{}
	}
	normalized, err := sourcing.Normalize(envelope)
	if err != nil {
		return catalog.ProductFacts{}
	}
	return catalogProductFactsFromSnapshot(normalized.Identity, snapshot, normalized.Warnings)
}

func catalogProductFactsFromSnapshot(identity sourcing.SourceIdentity, snapshot catalog.ProductSnapshot, warnings []sourcing.SourceWarning) catalog.ProductFacts {
	return catalog.ProductFacts{
		SourceKey:      identity.SourceKey(),
		SourceType:     identity.SourceType,
		SourcePlatform: identity.SourcePlatform,
		SourceID:       identity.SourceID,
		SourceURL:      identity.SourceURL,
		Title:          snapshot.Title,
		Description:    snapshot.Description,
		Brand:          snapshot.Brand,
		Attributes:     catalogAttributeFacts(snapshot.Attributes),
		Variants:       catalogVariantFacts(snapshot.Variants),
		Warnings:       catalogWarnings(warnings),
	}
}

func catalogVariantFacts(variants []catalog.Variant) []catalog.VariantFacts {
	if len(variants) == 0 {
		return nil
	}
	facts := make([]catalog.VariantFacts, 0, len(variants))
	for _, variant := range variants {
		facts = append(facts, catalog.VariantFacts{
			SourceID:   variant.SourceID,
			Title:      variant.Title,
			SKU:        variant.SKU,
			Attributes: catalogAttributeFacts(variant.Attributes),
		})
	}
	return facts
}

func catalogAttributeFacts(attributes []catalog.Attribute) map[string]string {
	if len(attributes) == 0 {
		return nil
	}
	facts := make(map[string]string, len(attributes))
	for _, attribute := range attributes {
		facts[attribute.Name] = attribute.Value
	}
	return facts
}

func catalogWarnings(warnings []sourcing.SourceWarning) []catalog.FactWarning {
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
