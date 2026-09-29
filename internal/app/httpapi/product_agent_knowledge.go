package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"go.opentelemetry.io/otel/trace"

	sigjson "sigs.k8s.io/json"
	"task-processor/internal/agent"
	"task-processor/internal/authidentity"
	"task-processor/internal/integration/commercetoolauth"
	"task-processor/internal/integration/openai"
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
	command := knowledge.ContextRequest{
		Scope: knowledge.Scope{OrganizationID: identity.TenantID, ActorID: identity.UserID}, Binding: binding, Key: key,
		Selection: "knowledge-base:" + baseID, PolicyVersion: knowledge.ContextPolicyVersion,
	}
	if record, readErr := a.store.Read(ctx, agent.Scope{OrganizationID: identity.TenantID, ActorID: identity.UserID}, binding.ContextID, key); readErr == nil {
		ref := record.State.Request.ContextSnapshotRef
		if record.State.Request.Binding != binding || ref.Absent() || ref.Kind != knowledge.ContextKind {
			return agent.Request{}, agent.ErrConflict
		}
		if err := a.context.ValidateMaterializedRequest(ctx, command, knowledge.ContextSnapshotRef{Kind: ref.Kind, ID: ref.ID, Digest: ref.Digest}); err != nil {
			return agent.Request{}, err
		}
		request.ContextSnapshotRef = ref
		request.PromptVersion = "product-title-agent-knowledge-v1"
		return request, nil // Runtime.Start/Claim still compares the complete request.
	}
	ref, err := a.context.Materialize(ctx, command)
	if err != nil {
		return agent.Request{}, err
	}
	request.ContextSnapshotRef = agent.ContextSnapshotRef{Kind: ref.Kind, ID: ref.ID, Digest: ref.Digest}
	request.PromptVersion = "product-title-agent-knowledge-v1"
	if a.model == nil {
		return agent.Request{}, agent.ErrUnavailable
	}
	// Reserve the real UUID width without creating a run. Trace and the empty
	// first state match execution; the quote is never reserved or dispatched.
	initial := agent.State{Request: request, RunID: strings.Repeat("0", 36)}
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		initial.TraceID = span.TraceID().String()
	}
	if _, err := a.model.Quote(ctx, initial.ModelInput(a.definition)); err != nil {
		if errors.Is(err, openai.ErrTextInput) {
			return agent.Request{}, knowledge.ErrContextTooLarge
		}
		return agent.Request{}, err
	}
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
