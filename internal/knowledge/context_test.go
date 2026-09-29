package knowledge

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"task-processor/internal/agent"
)

func contextRequest() ContextRequest {
	return ContextRequest{Scope: Scope{"org", "actor"}, Binding: agent.Binding{ContextKind: "acquisition", ContextID: "operation", ProductKey: "product", CatalogVersion: "1", PublicationID: "publication", TargetPlatform: "shein"}, Key: "request", Selection: "knowledge-base:11111111-1111-4111-8111-111111111111", PolicyVersion: ContextPolicyVersion}
}

func TestKnowledgeMaterializationFingerprintBindsClientCommand(t *testing.T) {
	original := contextRequest()
	fp, err := MaterializationFingerprint(original)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*ContextRequest){
		func(r *ContextRequest) { r.Binding.ProductKey = "other" },
		func(r *ContextRequest) { r.Binding.CatalogVersion = "2" },
		func(r *ContextRequest) { r.Binding.PublicationID = "other" },
		func(r *ContextRequest) { r.Binding.TargetPlatform = "temu" },
		func(r *ContextRequest) { r.Selection = "knowledge-base:22222222-2222-4222-8222-222222222222" },
		func(r *ContextRequest) { r.PolicyVersion = "v2" },
	} {
		changed := original
		mutate(&changed)
		other, err := MaterializationFingerprint(changed)
		if err != nil || other == fp {
			t.Fatalf("changed command adopted: %v", err)
		}
	}
	for _, selection := range []string{"", "none", "knowledge-base:11111111111141118111111111111111", original.Selection + ",other"} {
		changed := original
		changed.Selection = selection
		if _, err := MaterializationFingerprint(changed); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid selection %q: %v", selection, err)
		}
	}
}

func TestKnowledgeContextBoundsRejectInsteadOfTruncate(t *testing.T) {
	entry := ContextEntry{SourceID: "11111111-1111-4111-8111-111111111111", RevisionID: "22222222-2222-4222-8222-222222222222", Name: "Guide", State: Available, Text: strings.Repeat("a", MaxContextRevisionBytes)}
	entry.ContentDigest = Digest([]byte(entry.Text))
	entry.Citation = Citation{ID: "33333333-3333-4333-8333-333333333333", BaseID: "44444444-4444-4444-8444-444444444444", SourceID: entry.SourceID, RevisionID: entry.RevisionID, ContentDigest: entry.ContentDigest, Location: "text"}
	bundle := ContextBundle{BaseID: entry.Citation.BaseID, Binding: contextRequest().Binding, Entries: []ContextEntry{entry}}
	if _, err := EncodeContextBundle(bundle); err != nil {
		t.Fatal(err)
	}
	bundle.Entries[0].Text += "a"
	if _, err := EncodeContextBundle(bundle); !errors.Is(err, ErrContextTooLarge) {
		t.Fatalf("revision overflow: %v", err)
	}
	bundle.Entries = []ContextEntry{entry, entry, entry, entry}
	if _, err := EncodeContextBundle(bundle); !errors.Is(err, ErrContextTooLarge) {
		t.Fatalf("total overflow: %v", err)
	}
	bundle.Entries = []ContextEntry{entry}
	bundle.Entries[0].Text = strings.Repeat("<", MaxContextRevisionBytes)
	bundle.Entries[0].ContentDigest = Digest([]byte(bundle.Entries[0].Text))
	bundle.Entries[0].Citation.ContentDigest = bundle.Entries[0].ContentDigest
	if _, err := EncodeContextBundle(bundle); !errors.Is(err, ErrContextTooLarge) {
		t.Fatalf("serialized overflow: %v", err)
	}
	ref, _ := json.Marshal(ContextSnapshotRef{Kind: ContextKind, ID: entry.Citation.ID, Digest: entry.ContentDigest})
	if strings.Contains(string(ref), "text") || strings.Contains(string(ref), "Guide") {
		t.Fatal("opaque ref contains protected data")
	}
}
