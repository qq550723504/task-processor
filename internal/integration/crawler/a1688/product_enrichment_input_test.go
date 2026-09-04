package a1688

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"task-processor/internal/product/enrichment"
	"task-processor/internal/product/sourcing"
)

func TestProductEnrichmentRequestUsesCanonicalSourcingFactsWithoutAliasingInput(t *testing.T) {
	t.Parallel()

	envelope := sourcing.SourceEnvelope{
		Identity: sourcing.SourceIdentity{
			SourceType:     sourcing.SourceTypeCrawler,
			SourcePlatform: " 1688 ",
			SourceID:       " 1001 ",
		},
		RawReference: sourcing.RawSourceReference{
			ReferenceID: "raw-1001",
			Metadata:    map[string]string{"etag": "v1"},
		},
		ProductCandidate: sourcing.ProductCandidate{
			Title:       "Steel Bottle",
			Description: "Insulated steel bottle",
			Attributes:  map[string]string{"material": "steel"},
			Variants: []sourcing.ProductVariantCandidate{{
				SourceID:   "sku-red",
				SKU:        "RED-1",
				Attributes: map[string]string{"color": "red"},
			}},
		},
		Warnings: []sourcing.SourceWarning{{Code: " SOURCE_LIMITED ", Message: "One source"}},
		Trace:    sourcing.SourceTrace{Notes: []string{"captured by crawler"}},
	}
	policy := enrichment.PolicySnapshot{
		Version:             "v1",
		AllowedFields:       []string{"description"},
		RequiredFields:      []string{"description"},
		MinimumQualityScore: 80,
	}
	before := mustMarshalSourceEnvelope(t, envelope)

	request, err := ProductEnrichmentRequest(envelope, policy)
	if err != nil {
		t.Fatalf("ProductEnrichmentRequest() error = %v", err)
	}
	if request.Snapshot.Title != "Steel Bottle" || request.Snapshot.Description != "Insulated steel bottle" {
		t.Fatalf("ProductEnrichmentRequest() snapshot = %#v", request.Snapshot)
	}
	if request.Source.Identity.SourcePlatform != "1688" || request.Source.Identity.SourceID != "1001" {
		t.Fatalf("ProductEnrichmentRequest() source identity = %#v", request.Source.Identity)
	}
	if !reflect.DeepEqual(request.Policy, policy) {
		t.Fatalf("ProductEnrichmentRequest() policy = %#v, want %#v", request.Policy, policy)
	}
	if after := mustMarshalSourceEnvelope(t, envelope); !reflect.DeepEqual(after, before) {
		t.Fatalf("ProductEnrichmentRequest() mutated source\nbefore: %s\nafter:  %s", before, after)
	}

	request.Source.RawReference.Metadata["etag"] = "request-mutated"
	request.Source.ProductCandidate.Attributes["material"] = "plastic"
	request.Source.ProductCandidate.Variants[0].Attributes["color"] = "blue"
	request.Source.Trace.Notes[0] = "request-mutated"
	request.Policy.AllowedFields[0] = "title"
	if after := mustMarshalSourceEnvelope(t, envelope); !reflect.DeepEqual(after, before) {
		t.Fatalf("returned request aliases source input\nbefore: %s\nafter:  %s", before, after)
	}
	if policy.AllowedFields[0] != "description" {
		t.Fatalf("returned request aliases policy input = %v", policy.AllowedFields)
	}
}

func TestProductEnrichmentRequestMapsInvalidSourceToStableDomainError(t *testing.T) {
	t.Parallel()

	_, err := ProductEnrichmentRequest(sourcing.SourceEnvelope{}, enrichment.PolicySnapshot{Version: "v1"})
	if err != enrichment.ErrInputInvalid {
		t.Fatalf("ProductEnrichmentRequest() error = %v, want ErrInputInvalid", err)
	}
	if errors.Is(err, sourcing.ErrSourceIdentityRequired) {
		t.Fatalf("ProductEnrichmentRequest() exposes sourcing implementation error %v", err)
	}
}

func mustMarshalSourceEnvelope(t *testing.T, envelope sourcing.SourceEnvelope) []byte {
	t.Helper()
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("json.Marshal(envelope): %v", err)
	}
	return data
}
