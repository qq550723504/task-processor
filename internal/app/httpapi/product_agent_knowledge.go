package httpapi

import (
	"context"
	"encoding/json"

	sigjson "sigs.k8s.io/json"
	"task-processor/internal/agent"
	"task-processor/internal/authidentity"
	"task-processor/internal/integration/commercetoolauth"
	"task-processor/internal/knowledge"
	"task-processor/internal/product/review"
)

type productAgentRequestBody struct {
	Revision           string          `json:"revision,omitempty"`
	Feedback           string          `json:"feedback,omitempty"`
	TargetPlatform     string          `json:"targetPlatform,omitempty"`
	KnowledgeSelection json.RawMessage `json:"knowledgeSelection,omitempty"`
}

func (b productAgentRequestBody) knowledgeSelection(action string) (string, error) {
	if len(b.KnowledgeSelection) == 0 {
		return "", nil
	}
	if action != "start" {
		return "", agent.ErrInvalid
	}
	var selection struct {
		KnowledgeBaseID string `json:"knowledgeBaseId"`
	}
	violations, err := sigjson.UnmarshalStrict(b.KnowledgeSelection, &selection)
	if err != nil || len(violations) != 0 || !knowledge.ValidID(selection.KnowledgeBaseID) {
		return "", agent.ErrInvalid
	}
	return selection.KnowledgeBaseID, nil
}

// Reuse the existing verified, request-local bearer capability. No credential
// is copied into the Agent request, provenance, response or persistence.
func knowledgeRequestContext(ctx context.Context) (context.Context, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	capability, bound := ctx.Value(productReviewCapabilityContextKey{}).(productReviewRequestCapability)
	if !ok || !bound || capability.actorID != identity.UserID || capability.effectiveOrganizationID != identity.TenantID ||
		identity.TenantID != identity.EffectiveOrganizationID {
		return ctx, knowledge.ErrForbidden
	}
	return commercetoolauth.WithOrganizationRequest(ctx, commercetoolauth.OrganizationRequest{Identity: identity, BearerToken: capability.bearerToken, RequestedOrganizationID: identity.TenantID}), nil
}

func (a *productAgentApplication) startRequest(ctx context.Context, binding agent.Binding, key, baseID string) (agent.Request, error) {
	request := agent.Request{Key: key, Binding: binding, PolicyVersion: "title-review-v1", PromptVersion: "product-title-agent-v1", Limits: a.config.Limits}
	if baseID == "" {
		return request, nil
	}
	if a.context == nil {
		return agent.Request{}, knowledge.ErrUnavailable
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok {
		return agent.Request{}, knowledge.ErrForbidden
	}
	ref, err := a.context.Materialize(ctx, knowledge.ContextRequest{
		Scope: knowledge.Scope{OrganizationID: identity.TenantID, ActorID: identity.UserID}, Binding: binding, Key: key,
		Selection: "knowledge-base:" + baseID, PolicyVersion: knowledge.ContextPolicyVersion,
	})
	if err != nil {
		return agent.Request{}, err
	}
	request.ContextSnapshotRef = agent.ContextSnapshotRef{Kind: ref.Kind, ID: ref.ID, Digest: ref.Digest}
	request.PromptVersion = "product-title-agent-knowledge-v1"
	return request, nil
}

func agentContextProvenance(state agent.State) (*review.ContextProvenanceRef, error) {
	ref := state.Request.ContextSnapshotRef
	if !agent.ValidContextCitations(ref, state.ContextCitationRefs) {
		return nil, agent.ErrConflict
	}
	if ref.Absent() {
		return nil, nil
	}
	if ref.Kind != knowledge.ContextKind || !knowledge.ValidID(ref.ID) {
		return nil, agent.ErrConflict
	}
	provenance := &review.ContextProvenanceRef{Kind: ref.Kind, BundleID: ref.ID, BundleDigest: ref.Digest, OriginAgentRunID: state.RunID, CitationIDs: []string{}}
	for _, citation := range state.ContextCitationRefs {
		provenance.CitationIDs = append(provenance.CitationIDs, citation.ID)
	}
	if !provenance.Valid() {
		return nil, agent.ErrConflict
	}
	return provenance, nil
}
