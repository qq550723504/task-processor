package httpapi

import (
	"encoding/json"

	"task-processor/internal/agent"
	"task-processor/internal/commercetool"
	"task-processor/internal/integration/agent/grsaitext"
	"task-processor/internal/product/catalog/tools/canonicalinspect"
	"task-processor/internal/product/enrichment"
)

// Only canonical inspection discloses raw captured evidence fields for the
// current title policy. Asset/source identity/readiness are not raw evidence.
func agentCandidateEvidenceObserved(b agent.Binding, candidate enrichment.Candidate, history []agent.Observation) bool {
	observed := make(map[string]bool)
	for _, item := range history {
		if item.Tool != canonicalinspect.Definition().Ref || item.CallID == "" || item.AuditStatus != commercetool.AuditStatusRecorded || item.InvocationID != "" {
			continue
		}
		raw, err := grsaitext.EvidenceForPrompt(item.Output)
		if err != nil {
			continue
		}
		var view struct {
			Projection string          `json:"projection"`
			Evidence   json.RawMessage `json:"evidence"`
		}
		if json.Unmarshal(raw, &view) != nil {
			continue
		}
		if view.Projection == "title-evidence-v1" {
			raw = view.Evidence
		}
		var output canonicalinspect.Output
		if json.Unmarshal(raw, &output) != nil || output.ProductKey != b.ProductKey || output.CatalogVersion != b.CatalogVersion || output.CatalogPublicationID != b.PublicationID {
			continue
		}
		for _, source := range output.Snapshot.Sources {
			for _, id := range []string{source.Detail, source.SnapshotID, source.Checksum} {
				if id != "" {
					observed[id] = true
				}
			}
		}
	}
	if len(candidate.Changes) == 0 {
		return false
	}
	for _, change := range candidate.Changes {
		if len(change.EvidenceIDs) == 0 {
			return false
		}
		for _, id := range change.EvidenceIDs {
			if !observed[id] {
				return false
			}
		}
	}
	return true
}
