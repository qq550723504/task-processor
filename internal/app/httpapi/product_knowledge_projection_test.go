package httpapi

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/knowledge"
	"task-processor/internal/product/review"
	"testing"
)

type citationReaderFixture struct {
	fail  bool
	calls int
}

func (r *citationReaderFixture) ReadContext(context.Context, knowledge.Scope, knowledge.ContextSnapshotRef) (knowledge.ContextBundle, error) {
	return knowledge.ContextBundle{}, nil
}
func (r *citationReaderFixture) ReadCitation(_ context.Context, _ knowledge.Scope, ref knowledge.ContextSnapshotRef, id string) (knowledge.CitationContent, error) {
	r.calls++
	if r.fail || r.calls == 2 {
		return knowledge.CitationContent{}, knowledge.ErrForbidden
	}
	return knowledge.CitationContent{Citation: knowledge.Citation{ID: id, BaseID: "22345678-1234-4234-8234-123456789abc", SourceID: "32345678-1234-4234-8234-123456789abc", RevisionID: "42345678-1234-4234-8234-123456789abc", Location: "document"}, Name: "Protected brand", Text: strings.Repeat("字", 1000), State: knowledge.Partial}, nil
}
func TestKnowledgeProjectionDropsProtectedContentWhenAnyCitationUnavailable(t *testing.T) {
	p := &review.ContextProvenanceRef{Kind: "knowledge", BundleID: "12345678-1234-4234-8234-123456789abc", BundleDigest: strings.Repeat("a", 64), OriginAgentRunID: "52345678-1234-4234-8234-123456789abc", CitationIDs: []string{"62345678-1234-4234-8234-123456789abc"}}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "actor", TenantID: "org", EffectiveOrganizationID: "org"})
	r := &citationReaderFixture{}
	projection := projectKnowledgeCitations(ctx, r, p)
	require.Equal(t, "available", projection.Status)
	require.Len(t, []rune(projection.Citations[0].Excerpt), 320)
	require.Equal(t, "PARTIAL", projection.Citations[0].State)
	p.CitationIDs = append(p.CitationIDs, "72345678-1234-4234-8234-123456789abc")
	r.calls = 0
	projection = projectKnowledgeCitations(ctx, r, p)
	require.Equal(t, "unavailable", projection.Status)
	wire, err := json.Marshal(projection)
	require.NoError(t, err)
	require.NotContains(t, string(wire), "Protected brand")
	require.NotContains(t, string(wire), "excerpt")
	require.Empty(t, projection.Citations)
}
func TestReviewWithOpaqueKnowledgeHasUnavailableProjectionWithoutReader(t *testing.T) {
	p := &review.ContextProvenanceRef{Kind: "knowledge", BundleID: "12345678-1234-4234-8234-123456789abc", BundleDigest: strings.Repeat("a", 64), OriginAgentRunID: "52345678-1234-4234-8234-123456789abc"}
	wire, err := marshalProductReviewView(review.View{ID: "82345678-1234-4234-8234-123456789abc", Owner: "actor", Input: review.CreateInput{ProductKey: "product", BaseVersion: 1}, Policy: "title-review-v1", State: "pending", Revision: 1, ContextProvenance: p})
	require.NoError(t, err)
	require.Contains(t, string(wire), `"knowledge":{"status":"unavailable"`)
	require.NotContains(t, string(wire), p.BundleDigest)
}
