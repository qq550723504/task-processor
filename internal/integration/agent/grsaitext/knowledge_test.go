package grsaitext

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"task-processor/internal/agent"
	"task-processor/internal/aicapability"
	k "task-processor/internal/knowledge"
)

func TestKnowledgeRefWithoutReaderCannotQuoteOrDispatch(t *testing.T) {
	model, ctx, input, ledger, sends, _ := agentModelFixture(t, `{"Kind":"interrupt"}`, `{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}`)
	input.ContextSnapshotRef = agent.ContextSnapshotRef{Kind: "knowledge", ID: uuid.NewString(), Digest: strings.Repeat("a", 64)}
	if _, err := model.Quote(ctx, input); err == nil || ledger.claims != 0 || sends.Load() != 0 {
		t.Fatal("opaque knowledge ref was quoted without a content admission reader")
	}
}

type knowledgeModelStub struct {
	bundle                    k.ContextBundle
	reads, acquires, releases int
	denyReadAt                int
	acquireErr, releaseErr    error
	afterAcquire              func()
	releasedAfterCancel       bool
}

func (s *knowledgeModelStub) ReadContext(_ context.Context, scope k.Scope, _ k.ContextSnapshotRef) (k.ContextBundle, error) {
	s.reads++
	if scope != (k.Scope{OrganizationID: "org", ActorID: "actor"}) {
		return k.ContextBundle{}, k.ErrForbidden
	}
	if s.denyReadAt > 0 && s.reads >= s.denyReadAt {
		return k.ContextBundle{}, k.ErrInactive
	}
	return s.bundle, nil
}
func (s *knowledgeModelStub) AcquireDispatchPermit(_ context.Context, scope k.Scope, ref k.ContextSnapshotRef, invocation string) (k.DispatchPermit, error) {
	s.acquires++
	if s.acquireErr != nil {
		return k.DispatchPermit{}, s.acquireErr
	}
	permit := k.DispatchPermit{ID: uuid.NewString(), OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, Ref: ref, InvocationID: invocation, ExpiresAt: time.Now().Add(k.DispatchPermitLease)}
	if s.afterAcquire != nil {
		s.afterAcquire()
	}
	return permit, nil
}
func (s *knowledgeModelStub) ReleaseDispatchPermit(ctx context.Context, permit k.DispatchPermit) error {
	s.releases++
	_, bounded := ctx.Deadline()
	s.releasedAfterCancel = ctx.Err() == nil && bounded && permit.ID != ""
	return s.releaseErr
}
func knowledgeModelInput(t *testing.T, model *AgentTextModel, input agent.ModelInput) (agent.ModelInput, *knowledgeModelStub) {
	t.Helper()
	base, source, revision, citation := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	text := "Protected synthetic brand guidance: prefer clear, concise wording."
	stub := &knowledgeModelStub{bundle: k.ContextBundle{BaseID: base, Binding: input.Binding, Entries: []k.ContextEntry{{
		SourceID: source, RevisionID: revision, Name: "Synthetic brand guide", State: k.Available, Text: text, ContentDigest: k.Digest([]byte(text)),
		Citation: k.Citation{ID: citation, BaseID: base, SourceID: source, RevisionID: revision, ContentDigest: k.Digest([]byte(text)), Location: "text"},
	}}}}
	raw, err := k.EncodeContextBundle(stub.bundle)
	if err != nil {
		t.Fatal(err)
	}
	input.ContextSnapshotRef = agent.ContextSnapshotRef{Kind: k.ContextKind, ID: uuid.NewString(), Digest: k.Digest(raw)}
	model.knowledge = stub
	return input, stub
}

func TestKnowledgePromptAndCitationRemainSupplemental(t *testing.T) {
	model, ctx, input, ledger, sends, _ := agentModelFixture(t, `{"Kind":"interrupt"}`, `{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}`)
	ordinary, err := model.Quote(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	input, stub := knowledgeModelInput(t, model, input)
	prepared, err := model.prepare(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prepared.request.Prompt, stub.bundle.Entries[0].Text) || ordinary.Reference == prepared.quote.Reference {
		t.Fatal("Knowledge text excluded from actual prompt/quote identity")
	}
	// Use the existing local fixture transport with an exact model citation.
	content, _ := json.Marshal(agent.Action{Kind: "propose", ContextCitationIDs: []string{stub.bundle.Entries[0].Citation.ID}})
	// The fixture response can be changed through a fresh fixture; the frozen
	// synthetic bundle remains the same and carries no production credentials.
	citedModel, citedCtx, citedInput, citedLedger, citedSends, _ := agentModelFixture(t, string(content), `{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}`)
	citedModel.knowledge = stub
	citedInput.ContextSnapshotRef = input.ContextSnapshotRef
	citedInput.InvocationID = "knowledge-citation"
	citedInput.UpperBound, err = citedModel.Quote(citedCtx, citedInput)
	if err != nil {
		t.Fatal(err)
	}
	result, err := citedModel.Decide(citedCtx, citedInput)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ContextCitationRefs) != 1 || result.ContextCitationRefs[0].Snapshot != input.ContextSnapshotRef ||
		result.ContextCitationRefs[0].ID != stub.bundle.Entries[0].Citation.ID || len(result.Action.ContextCitationIDs) != 0 {
		t.Fatal("model IDs were not admitted as exact opaque provenance")
	}
	if citedSends.Load() != 1 || citedLedger.rows[citedInput.InvocationID].Outcome != aicapability.InvocationSucceeded || stub.acquires != 1 || stub.releases != 1 {
		t.Fatal("wrong Knowledge/provider admission lifecycle")
	}
	if ledger.claims != 0 || sends.Load() != 0 {
		t.Fatal("quote performed provider effects")
	}
}

func TestKnowledgeUnavailableAtDispatchSendsNothingAndReleasesExistingReservation(t *testing.T) {
	for _, stage := range []string{"read", "permit"} {
		t.Run(stage, func(t *testing.T) {
			model, ctx, input, ledger, sends, _ := agentModelFixture(t, `{"Kind":"interrupt"}`, `{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}`)
			input, stub := knowledgeModelInput(t, model, input)
			if stage == "read" {
				stub.denyReadAt = 3
			} else {
				stub.acquireErr = k.ErrInactive
			}
			var err error
			input.UpperBound, err = model.Quote(ctx, input)
			if err != nil {
				t.Fatal(err)
			}
			input.InvocationID = "blocked-" + stage
			result, err := model.Decide(ctx, input)
			if !errors.Is(err, agent.ErrModelNotDispatched) || sends.Load() != 0 || !result.Usage.Known || result.Usage.Tokens != 0 ||
				ledger.rows[input.InvocationID].Outcome != aicapability.InvocationFailed || ledger.terminals != 1 || stub.releases != 0 {
				t.Fatal("Knowledge revocation/disable did not preserve known zero-send owner behavior", err)
			}
		})
	}
}

func TestKnowledgePermitDrainsEvenWhenUsageOrCleanupUnknown(t *testing.T) {
	for _, usage := range []string{`{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}`, `null`} {
		t.Run(usage, func(t *testing.T) {
			model, ctx, input, ledger, sends, _ := agentModelFixture(t, `{"Kind":"interrupt"}`, usage)
			input, stub := knowledgeModelInput(t, model, input)
			stub.releaseErr = errors.New("cleanup unavailable")
			var err error
			input.UpperBound, err = model.Quote(ctx, input)
			if err != nil {
				t.Fatal(err)
			}
			input.InvocationID = "drain"
			_, err = model.Decide(ctx, input)
			if usage == "null" && (err == nil || ledger.terminals != 0) {
				t.Fatal("Knowledge cleanup converted unknown provider usage into a terminal outcome")
			}
			if usage != "null" && err != nil {
				t.Fatal(err)
			}
			if sends.Load() != 1 || stub.acquires != 1 || stub.releases != 1 || !stub.releasedAfterCancel {
				t.Fatal("permit not cleaned up with a bounded independent context")
			}
		})
	}
}

func TestKnowledgeRejectsOutOfBundleCitationWithoutLosingObservedUsage(t *testing.T) {
	for _, ids := range [][]string{{uuid.NewString()}, {"not-a-citation"}} {
		t.Run(ids[0], func(t *testing.T) {
			content, _ := json.Marshal(agent.Action{Kind: "propose", ContextCitationIDs: ids})
			model, ctx, input, ledger, sends, _ := agentModelFixture(t, string(content), `{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}`)
			input, stub := knowledgeModelInput(t, model, input)
			var err error
			input.UpperBound, err = model.Quote(ctx, input)
			if err != nil {
				t.Fatal(err)
			}
			input.InvocationID = "invalid-citation"
			result, err := model.Decide(ctx, input)
			if err != nil {
				t.Fatal(err)
			}
			if result.Action.Kind != "" || len(result.ContextCitationRefs) != 0 || !result.Usage.Known || result.Usage.Tokens != 5 ||
				ledger.rows[input.InvocationID].Outcome != aicapability.InvocationUsageObservedFailed || sends.Load() != 1 || stub.releases != 1 {
				t.Fatal("invalid citation became reviewable or lost paid usage")
			}
		})
	}
}
