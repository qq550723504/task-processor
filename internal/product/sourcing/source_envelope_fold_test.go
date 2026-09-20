package sourcing

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// A real 1688 product repeats the same class of fact once per variant (missing
// SKU, missing currency, ...). The envelope collection budget is a fixed bound,
// so per-variant repetition must not scale the recorded facts.
func TestAcquisitionFoldsRepeatedPerInstanceFactsBelowEnvelopeLimit(t *testing.T) {
	evidence := acquisitionEvidenceFixture()
	evidence.Description = nil
	evidence.Images = nil
	evidence.PriceFacts = nil
	evidence.Variants = nil
	const variants = 200
	for i := 0; i < variants; i++ {
		sourceID := fmt.Sprintf("source-%d", i)
		evidence.Variants = append(evidence.Variants, AcquisitionVariant{SourceID: &sourceID})
		evidence.MissingFacts = append(evidence.MissingFacts, MissingFact{Field: fmt.Sprintf("variants[%d].sku", i), Reason: "not_observed"})
		evidence.Warnings = append(evidence.Warnings, SourceWarning{Code: "MISSING_FACT", Field: fmt.Sprintf("variants[%d].sku", i)})
	}
	envelope, err := MapAcquisitionEvidence(AcquisitionSource{OfferID: evidence.OfferID, URL: evidence.SourceURL}, evidence, "browser_capture", "fold-operation")
	require.NoError(t, err)

	// Bounded by schema, not by variant count: the server-derived facts also
	// repeat once per variant before folding.
	require.Less(t, len(envelope.MissingFacts), variants)
	require.Less(t, len(envelope.Warnings), variants)
	require.LessOrEqual(t, len(envelope.MissingFacts), MaxSourceEnvelopeCollectionItems)
	require.LessOrEqual(t, len(envelope.Warnings), MaxSourceEnvelopeCollectionItems)

	require.Contains(t, envelope.MissingFacts, MissingFact{Field: "variants[].sku", Reason: "not_observed", Count: variants})
	require.Contains(t, envelope.MissingFacts, MissingFact{Field: "variants[].sku", Reason: "source SKU absent", Count: variants})
	require.Contains(t, envelope.Warnings, SourceWarning{Code: "missing_fact", Field: "variants[].sku", Message: "source reported MISSING_FACT", Count: variants})
}

// Folding must be deterministic and must not rewrite single-occurrence facts:
// existing evidence (and its Go/wire goldens) keeps its exact shape.
func TestFoldSourceEnvelopeRepeatsIsDeterministicAndPreservesSingletons(t *testing.T) {
	envelope := SourceEnvelope{
		MissingFacts: []MissingFact{
			{Field: "variants.3.sku", Reason: "source SKU absent"},
			{Field: "variants.0.sku", Reason: "source SKU absent"},
			{Field: "description", Reason: "source did not supply description text"},
			{Field: "variants[1].sku", Reason: "not_observed"},
			{Field: "variants[0].sku", Reason: "not_observed"},
		},
		Warnings: []SourceWarning{
			{Code: "missing_source_fact", Field: "variants.3.sku", Message: "source SKU absent"},
			{Code: "missing_source_fact", Field: "variants.0.sku", Message: "source SKU absent"},
			{Code: "source_price_precision", Field: "variants.7.price", Message: "original decimal retained without a rounded Catalog price"},
		},
	}
	foldSourceEnvelopeRepeats(&envelope)
	require.Equal(t, []MissingFact{
		{Field: "variants[].sku", Reason: "source SKU absent", Count: 2},
		{Field: "description", Reason: "source did not supply description text"},
		{Field: "variants[].sku", Reason: "not_observed", Count: 2},
	}, envelope.MissingFacts)
	require.Equal(t, []SourceWarning{
		{Code: "missing_source_fact", Field: "variants[].sku", Message: "source SKU absent", Count: 2},
		{Code: "source_price_precision", Field: "variants.7.price", Message: "original decimal retained without a rounded Catalog price"},
	}, envelope.Warnings)
}
