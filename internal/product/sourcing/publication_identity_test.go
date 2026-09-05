package sourcing

import (
	"math"
	"strings"
	"testing"
)

func TestPublicationIdentityKeyLimitsAndPayloadValidation(t *testing.T) {
	for _, id := range []string{strings.Repeat("x", 115), strings.Repeat("x", 116), strings.Repeat("商", 100)} {
		envelope := SourceEnvelope{Identity: SourceIdentity{SourceType: "crawler", SourcePlatform: "1688", SourceID: id}, ProductCandidate: ProductCandidate{Title: "Bottle"}}
		key, _, err := PublicationIdentity(envelope)
		if err != nil || len(key) > 128 {
			t.Fatalf("bounded identity: %s %v", key, err)
		}
		if len(id) == 115 && len(key) != 128 {
			t.Fatalf("exact 128-byte identity was changed: %q", key)
		}
		envelope.Identity.SourceVersion = "v2"
		next, _, err := PublicationIdentity(envelope)
		if err != nil || next != key {
			t.Fatal("version changes product identity")
		}
	}
	envelope := SourceEnvelope{Identity: SourceIdentity{SourceType: "crawler", SourcePlatform: "1688", SourceID: "123"}, ProductCandidate: ProductCandidate{Title: "Bottle"}, RawReference: RawSourceReference{Checksum: "same-raw"}}
	_, first, err := PublicationIdentity(envelope)
	if err != nil {
		t.Fatal(err)
	}
	envelope.ProductCandidate.Title = "Changed normalized facts"
	_, second, err := PublicationIdentity(envelope)
	if err != nil || first == second {
		t.Fatal("raw checksum concealed changed canonical payload")
	}
	envelope.ProductCandidate.Variants = []ProductVariantCandidate{{Price: math.Inf(1)}}
	if _, _, err := PublicationIdentity(envelope); err == nil {
		t.Fatal("unencodable publication accepted")
	}
}

func TestPublicationIdentityIsBoundedAndRepeatable(t *testing.T) {
	envelope := SourceEnvelope{Identity: SourceIdentity{SourceType: "crawler", SourcePlatform: "1688", SourceID: strings.Repeat("9", 200)}, ProductCandidate: ProductCandidate{Title: "Bottle"}}
	key, publication, err := PublicationIdentity(envelope)
	if err != nil || len(key) > 128 || len(publication) > 128 {
		t.Fatalf("identity = %q %q %v", key, publication, err)
	}
	againKey, againPublication, err := PublicationIdentity(envelope)
	if err != nil || againKey != key || againPublication != publication {
		t.Fatal("identity is unstable")
	}
	envelope.ProductCandidate.Title = "Changed"
	changedKey, changedPublication, err := PublicationIdentity(envelope)
	if err != nil || changedKey != key || changedPublication == publication {
		t.Fatal("content revision must preserve product identity and change publication")
	}
	envelope.Trace.SourceRunID = strings.Repeat("r", 200)
	_, runPublication, err := PublicationIdentity(envelope)
	if err != nil || len(runPublication) > 128 {
		t.Fatal("run identity is unbounded")
	}
	envelope.ProductCandidate.Title = "Changed again"
	_, replayPublication, _ := PublicationIdentity(envelope)
	if replayPublication != runPublication {
		t.Fatal("same run must retain idempotency key so Catalog detects changed payload")
	}
}

func TestPublicationIdentityTracksDurableLineageAndRejectsInvalidSource(t *testing.T) {
	envelope := SourceEnvelope{Identity: SourceIdentity{SourceType: "crawler", SourcePlatform: "1688", SourceID: "123"}, ProductCandidate: ProductCandidate{Title: "Bottle"}}
	_, first, err := PublicationIdentity(envelope)
	if err != nil {
		t.Fatal(err)
	}
	envelope.RawReference.ReferenceID = "evidence-2"
	_, second, err := PublicationIdentity(envelope)
	if err != nil || first == second {
		t.Fatal("changed durable lineage must be a distinct content publication")
	}
	envelope.Identity.SourceID = ""
	if _, _, err := PublicationIdentity(envelope); err == nil {
		t.Fatal("missing source identity accepted")
	}
}
