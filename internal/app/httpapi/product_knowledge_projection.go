package httpapi

import (
	"context"
	"task-processor/internal/authidentity"
	"task-processor/internal/knowledge"
	"task-processor/internal/product/review"
)

type productKnowledgeCitationDTO struct {
	ID         string `json:"id"`
	SourceID   string `json:"sourceId"`
	RevisionID string `json:"revisionId"`
	Name       string `json:"name"`
	Location   string `json:"location"`
	Excerpt    string `json:"excerpt"`
	State      string `json:"state"`
}
type productKnowledgeDTO struct {
	Status           string                        `json:"status"`
	OriginAgentRunID string                        `json:"originAgentRunId"`
	Citations        []productKnowledgeCitationDTO `json:"citations"`
}
type productReviewContextProjector func(context.Context, *review.ContextProvenanceRef) *productKnowledgeDTO

const productReviewContextProjectorKey = "product-review-knowledge-projector"

type productKnowledgeReader interface {
	knowledge.KnowledgeContextReader
	knowledge.KnowledgeCitationReader
}

func unavailableKnowledge(p *review.ContextProvenanceRef) *productKnowledgeDTO {
	if p == nil {
		return nil
	}
	return &productKnowledgeDTO{Status: "unavailable", OriginAgentRunID: p.OriginAgentRunID, Citations: []productKnowledgeCitationDTO{}}
}
func boundedKnowledgeLabel(text string, limit int) string {
	chars := []rune(text)
	if len(chars) > limit {
		chars = chars[:limit]
	}
	return string(chars)
}

// Display is a fresh authorized projection, never part of the durable Review.
// Failure hides all protected text while preserving the canonical review path.
func projectKnowledgeCitations(ctx context.Context, reader productKnowledgeReader, p *review.ContextProvenanceRef) *productKnowledgeDTO {
	fallback := unavailableKnowledge(p)
	if p == nil || reader == nil || !p.Valid() || p.Kind != knowledge.ContextKind {
		return fallback
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.TenantID != identity.EffectiveOrganizationID {
		return fallback
	}
	scope := knowledge.Scope{OrganizationID: identity.TenantID, ActorID: identity.UserID}
	ref := knowledge.ContextSnapshotRef{Kind: p.Kind, ID: p.BundleID, Digest: p.BundleDigest}
	if _, err := reader.ReadContext(ctx, scope, ref); err != nil {
		return fallback
	}
	projection := &productKnowledgeDTO{Status: "uncited", OriginAgentRunID: p.OriginAgentRunID, Citations: []productKnowledgeCitationDTO{}}
	for _, id := range p.CitationIDs {
		content, err := reader.ReadCitation(ctx, scope, ref, id)
		if err != nil || content.Citation.ID != id {
			return fallback
		}
		projection.Citations = append(projection.Citations, productKnowledgeCitationDTO{ID: id, SourceID: content.Citation.SourceID, RevisionID: content.Citation.RevisionID, Name: boundedKnowledgeLabel(content.Name, 512), Location: boundedKnowledgeLabel(content.Citation.Location, 256), Excerpt: boundedKnowledgeLabel(content.Text, 320), State: string(content.State)})
	}
	if len(projection.Citations) > 0 {
		projection.Status = "available"
	}
	return projection
}
func (a *productAgentApplication) projectKnowledge(ctx context.Context, p *review.ContextProvenanceRef) *productKnowledgeDTO {
	if p == nil {
		return nil
	}
	authorized, err := knowledgeRequestContext(ctx)
	if err != nil || a.context == nil {
		return unavailableKnowledge(p)
	}
	return projectKnowledgeCitations(authorized, a.context, p)
}
