package httpapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/agent"
	"task-processor/internal/commercetool"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/catalog/tools/canonicalinspect"
	"task-processor/internal/product/enrichment"
)

func TestAgentCandidateRequiresExactVisibleToolEvidence(t *testing.T) {
	b := agent.Binding{ProductKey: "crawler:1688:123", CatalogVersion: "1", PublicationID: "publication"}
	for _, mode := range []string{"exact", "no tool", "wrong tool", "wrong version", "missing call", "failed audit", "model observation", "wrong product", "wrong catalog", "wrong publication", "guessed ID", "extra ID", "omitted sources", "malformed output"} {
		t.Run(mode, func(t *testing.T) {
			candidate := enrichment.Candidate{Changes: []enrichment.FieldChange{{Field: "title", Value: "Title", EvidenceIDs: []string{"123"}}}}
			output := canonicalinspect.Output{ProductKey: b.ProductKey, CatalogVersion: b.CatalogVersion, CatalogPublicationID: b.PublicationID, Snapshot: catalog.ProductSnapshot{Title: "Saved title", Sources: []catalog.SourceRecord{{Detail: "123"}}}}
			item := agent.Observation{Tool: canonicalinspect.Definition().Ref, CallID: "run:step:2", AuditStatus: commercetool.AuditStatusRecorded}
			switch mode {
			case "wrong tool":
				item.Tool.ID = "product.asset.inspect"
			case "wrong version":
				item.Tool.Version = "v1.0.0"
			case "missing call":
				item.CallID = ""
			case "failed audit":
				item.AuditStatus = commercetool.AuditStatusRecordFailed
			case "model observation":
				item.InvocationID = "model-call"
			case "wrong product":
				output.ProductKey = "other-product"
			case "wrong catalog":
				output.CatalogVersion = "2"
			case "wrong publication":
				output.CatalogPublicationID = "other-publication"
			case "guessed ID":
				output.Snapshot.Sources = nil
			case "extra ID":
				candidate.Changes[0].EvidenceIDs = append(candidate.Changes[0].EvidenceIDs, "invented")
			case "omitted sources":
				output.Snapshot.Sources = append(output.Snapshot.Sources, catalog.SourceRecord{Detail: strings.Repeat("large", 1500)})
			}
			raw, err := json.Marshal(output)
			require.NoError(t, err)
			item.Output = raw
			if mode == "malformed output" {
				item.Output = json.RawMessage(`{`)
			}
			history := []agent.Observation{item}
			if mode == "no tool" {
				history = nil
			}
			require.Equal(t, mode == "exact", agentCandidateEvidenceObserved(b, candidate, history))
			require.Equal(t, mode == "exact", agentRunReviewable(agent.State{Phase: agent.HumanReviewRequired, Request: agent.Request{Binding: b}, Candidate: candidate, History: history, Validation: &agent.Validation{Valid: true}}), "read/review admission must recheck observed evidence")
		})
	}
}
