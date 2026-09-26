package sourceevidenceinspect

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"task-processor/internal/product/sourcing"
)

func projectionFixture() sourcing.PersistedPublication {
	_, _, _, s := bindingFixture()
	p := s.value
	p.Receipt.Producer = sourcing.ProducerDescriptor{Kind: sourcing.ControlledSnapshotProducerKind, Version: sourcing.ControlledSnapshotProducerVersion}
	p.Receipt.InputHash = strings.Repeat("a", 64)
	p.Receipt.EnvelopeHash = strings.Repeat("b", 64)
	p.Receipt.SnapshotHash = strings.Repeat("c", 64)
	p.Receipt.PublishedAt = time.Date(2026, 9, 12, 1, 0, 0, 0, time.UTC)
	p.Envelope.Identity = sourcing.SourceIdentity{SourceType: sourcing.SourceTypeCrawler, SourcePlatform: "1688", SourceID: "123456", SourceFingerprint: strings.Repeat("d", 64)}
	p.Envelope.RawReference.CapturedAt = p.Receipt.PublishedAt
	p.Envelope.RawReference.Checksum = "sha256:" + strings.Repeat("e", 64)
	p.Envelope.Warnings = []sourcing.SourceWarning{{Code: "missing_title", Field: "title", Message: "raw warning text"}}
	p.Envelope.MissingFacts = []sourcing.MissingFact{{Field: "images", Reason: "raw missing reason"}}
	return p
}

func TestProjectionPreservesExactStructuredEvidence(t *testing.T) {
	p := projectionFixture()
	before, _ := json.Marshal(p)
	raw, err := Project(p)
	if err != nil {
		t.Fatal(err)
	}
	var got Output
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.ProductKey != "product" || got.CatalogVersion != "9007199254740993" || got.PublicationID != "publication" || got.CatalogPublicationID != "publication" || got.Producer.Kind != p.Receipt.Producer.Kind || got.SourceIdentity.SourceID != "123456" || got.Lineage.EnvelopeHash != p.Receipt.EnvelopeHash || got.Lineage.SnapshotHash != p.Receipt.SnapshotHash {
		t.Fatalf("wrong structured binding: %s", raw)
	}
	if len(got.Warnings) != 1 || got.Warnings[0].Kind != "source_warning" || got.Warnings[0].Field != "title" || len(got.MissingFacts) != 1 || got.MissingFacts[0].Field != "images" {
		t.Fatalf("diagnostic record lost: %s", raw)
	}
	if got.Disclosure.RawTextReturned || got.Disclosure.SourceURLProvided || got.Disclosure.WarningCodesOmitted != 1 || got.Disclosure.WarningsTotal != 1 || got.Disclosure.MissingFactsOmitted != 0 {
		t.Fatalf("disclosure: %s", raw)
	}
	after, _ := json.Marshal(p)
	if string(before) != string(after) {
		t.Fatal("projection mutated source fact")
	}
	again, _ := Project(p)
	if string(again) != string(raw) {
		t.Fatal("projection is not deterministic")
	}
}

func TestProjectionNeverReturnsRawPayloads(t *testing.T) {
	p := projectionFixture()
	secret := "<html>private-cookie-and-token-UNIQUE</html>"
	p.Envelope.Identity.SourceURL = "https://example.test/path?token=" + secret
	p.Envelope.Identity.SourceVersion = secret
	p.Envelope.RawReference.URL = secret
	p.Envelope.RawReference.ReferenceID = secret
	p.Envelope.RawReference.SnapshotID = secret
	p.Envelope.RawReference.ReferenceType = secret
	p.Envelope.RawReference.Metadata = map[string]string{"cookie": secret, "innocent": secret}
	p.Envelope.Trace = sourcing.SourceTrace{RequestID: secret, SourceRunID: secret, Notes: []string{secret}}
	p.Envelope.ProductCandidate.Title = secret
	p.Envelope.ProductCandidate.Description = secret
	p.Envelope.Warnings[0].Message = secret
	p.Envelope.MissingFacts[0].Reason = secret
	p.Snapshot.Title = secret
	raw, err := Project(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "UNIQUE") || strings.Contains(string(raw), "example.test") || strings.Contains(string(raw), "raw warning") {
		t.Fatalf("raw payload leaked: %s", raw)
	}
}

func TestProjectionUnknownDiagnosticsAreOmittedAndCounted(t *testing.T) {
	p := projectionFixture()
	p.Envelope.Warnings = append(p.Envelope.Warnings, sourcing.SourceWarning{Code: "unreviewed-secret-code", Field: "title"}, sourcing.SourceWarning{Code: "missing_assets", Field: "private-field"})
	p.Envelope.MissingFacts = append(p.Envelope.MissingFacts, sourcing.MissingFact{Field: "private-field", Reason: "whatever"})
	raw, err := Project(p)
	if err != nil {
		t.Fatal(err)
	}
	var got Output
	_ = json.Unmarshal(raw, &got)
	if len(got.Warnings) != 3 || got.Warnings[2].Field != "" || len(got.MissingFacts) != 1 || got.Disclosure.WarningCodesOmitted != 3 || got.Disclosure.WarningsTotal != 3 || got.Disclosure.MissingFactsTotal != 2 || got.Disclosure.WarningFieldsOmitted != 1 || got.Disclosure.MissingFactsOmitted != 1 || !got.Disclosure.DiagnosticsPartial {
		t.Fatalf("dishonest omission counts: %s", raw)
	}
	if strings.Contains(string(raw), "unreviewed-secret-code") || strings.Contains(string(raw), "private-field") {
		t.Fatalf("unknown diagnostic leaked: %s", raw)
	}
}

func TestProjectionOmitsMalformedOptionalIdentifiers(t *testing.T) {
	p := projectionFixture()
	p.Envelope.Identity.SourceID = "raw-id?cookie=private"
	p.Envelope.Identity.SourcePlatform = strings.Repeat("x", 129)
	p.Envelope.Identity.SourceType = "unreviewed-type"
	p.Envelope.Identity.SourceFingerprint = "private-fingerprint"
	p.Envelope.RawReference.Checksum = "private-checksum"
	raw, err := Project(p)
	if err != nil {
		t.Fatal(err)
	}
	var got Output
	_ = json.Unmarshal(raw, &got)
	if got.SourceIdentity.SourceID != "" || got.SourceIdentity.SourcePlatform != "" || got.SourceIdentity.SourceType != "" || got.SourceIdentity.SourceFingerprint != "" || got.Capture.Checksum != "" || len(got.Disclosure.IdentityFieldsOmitted) != 4 {
		t.Fatalf("unsafe identifier: %s", raw)
	}
	for _, value := range []string{"private", "unreviewed-type", strings.Repeat("x", 129)} {
		if strings.Contains(string(raw), value) {
			t.Fatalf("unsafe value leaked: %s", raw)
		}
	}
}

func TestProjectionRejectsCorruptBindingAndOversizedCollections(t *testing.T) {
	for _, mutate := range []func(*sourcing.PersistedPublication){
		func(p *sourcing.PersistedPublication) { p.Receipt.InputHash = "corrupt" },
		func(p *sourcing.PersistedPublication) { p.Receipt.Producer.Kind = "<html>" },
		func(p *sourcing.PersistedPublication) { p.Receipt.PublicationID = "secret?cookie=x" },
		func(p *sourcing.PersistedPublication) { p.Receipt.CatalogVersion = 0 },
		func(p *sourcing.PersistedPublication) { p.Envelope.Warnings = make([]sourcing.SourceWarning, 257) },
		func(p *sourcing.PersistedPublication) { p.Envelope.MissingFacts = make([]sourcing.MissingFact, 257) },
	} {
		p := projectionFixture()
		mutate(&p)
		raw, err := Project(p)
		if err == nil || len(raw) != 0 {
			t.Fatalf("invalid evidence emitted: %s %v", raw, err)
		}
	}
	p := projectionFixture()
	p.Envelope.Warnings = make([]sourcing.SourceWarning, 256)
	p.Envelope.MissingFacts = make([]sourcing.MissingFact, 256)
	for i := range p.Envelope.Warnings {
		p.Envelope.Warnings[i] = sourcing.SourceWarning{Code: "missing_source_id", Field: "description"}
		p.Envelope.MissingFacts[i] = sourcing.MissingFact{Field: "description"}
	}
	raw, err := Project(p)
	if err != nil || len(raw) > MaxOutputBytes {
		t.Fatalf("bounded max collections failed: bytes=%d %v", len(raw), err)
	}
	if reflect.DeepEqual(raw, []byte{}) {
		t.Fatal("empty output")
	}
}
