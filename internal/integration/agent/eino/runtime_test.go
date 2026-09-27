package einoruntime

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"task-processor/internal/agent"
	"task-processor/internal/commercetool"
	"task-processor/internal/product/catalog/tools/canonicalinspect"
	"task-processor/internal/product/enrichment"
)

type fakeAuth struct{ denied bool }

func (a *fakeAuth) Authorize(context.Context, agent.Binding) (agent.Scope, error) {
	if a.denied {
		return agent.Scope{}, errors.New("revoked")
	}
	return agent.Scope{OrganizationID: "org-1", ActorID: "actor-1"}, nil
}

type fakeModel struct {
	actions            []agent.Action
	calls              int
	quote              agent.Quote
	unknown            bool
	fail               bool
	mutateHistory      bool
	lastFeedback       string
	panicAfterDispatch bool
	waitStarted        chan struct{}
	notDispatchedAt    int
}

func (m *fakeModel) Quote(context.Context, agent.ModelInput) (agent.Quote, error) {
	return m.quote, nil
}
func (m *fakeModel) Decide(ctx context.Context, in agent.ModelInput) (agent.ModelResult, error) {
	m.calls++
	if m.calls == m.notDispatchedAt {
		return agent.ModelResult{InvocationID: in.InvocationID, Usage: agent.ObservedUsage{Known: true, Currency: in.UpperBound.Currency}}, agent.ErrModelNotDispatched
	}
	if m.panicAfterDispatch {
		panic("provider adapter failed after dispatch")
	}
	if m.waitStarted != nil {
		close(m.waitStarted)
		<-ctx.Done()
		return agent.ModelResult{}, ctx.Err()
	}
	m.lastFeedback = in.UserFeedback
	if m.mutateHistory {
		for i := range in.History {
			if len(in.History[i].Output) > 0 {
				in.History[i].Output[0] = '!'
			}
		}
	}
	if m.fail {
		return agent.ModelResult{}, errors.New("lost response")
	}
	a := m.actions[0]
	m.actions = m.actions[1:]
	return agent.ModelResult{Action: a, InvocationID: in.InvocationID, Usage: agent.ObservedUsage{Tokens: 2, CostMicros: 1, Currency: "USD", Known: !m.unknown}}, nil
}

type fakeTools struct {
	calls     int
	auditFail bool
	last      agent.Binding
	output    json.RawMessage
}

func (f *fakeTools) Invoke(_ context.Context, _ commercetool.ToolRef, meta commercetool.CallMetadata, binding agent.Binding) (commercetool.Result, error) {
	f.calls++
	f.last = binding
	if meta.AgentRunID == "" || meta.BusinessTaskID != binding.ContextID || meta.CallID == "" {
		return commercetool.Result{}, errors.New("missing trusted metadata")
	}
	status := commercetool.AuditStatusRecorded
	if f.auditFail {
		status = commercetool.AuditStatusRecordFailed
	}
	output := f.output
	if output == nil {
		output = json.RawMessage(`{"title":"stored fact"}`)
	}
	return commercetool.Result{Output: output, AuditStatus: status}, nil
}

type fakeValidator struct {
	calls      int
	unresolved []string
	history    []agent.Observation
}

func (v *fakeValidator) Validate(_ context.Context, _ agent.Binding, policy string, c enrichment.Candidate, history []agent.Observation) (agent.Validation, error) {
	v.calls++
	v.history = history
	valid := len(c.Changes) == 1 && c.Changes[0].Value == "supported title"
	unresolved := v.unresolved
	if unresolved == nil {
		unresolved = []string{"image approval not evaluated"}
	}
	return agent.Validation{Valid: valid, PolicyVersion: policy, Unresolved: unresolved}, nil
}

// A test-only atomic store. This is not a production persistence/recovery claim.
type fakeStore struct {
	mu         sync.Mutex
	record     agent.Record
	exists     bool
	failCommit bool
}

func copyRecord(r agent.Record) agent.Record {
	raw, _ := json.Marshal(r)
	var c agent.Record
	_ = json.Unmarshal(raw, &c)
	return c
}
func (s *fakeStore) Claim(_ context.Context, input agent.Record, expected uint64) (agent.Record, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.exists {
		if expected != 0 {
			return agent.Record{}, false, agent.ErrConflict
		}
		input.State.Revision = 1
		s.record = copyRecord(input)
		s.exists = true
		return copyRecord(s.record), true, nil
	}
	if s.record.State.Fingerprint != input.State.Fingerprint || s.record.State.Scope != input.State.Scope {
		return agent.Record{}, false, agent.ErrConflict
	}
	if expected == 0 {
		return copyRecord(s.record), false, nil
	}
	if s.record.State.Revision != expected || s.record.State.Phase != agent.Interrupted {
		return agent.Record{}, false, agent.ErrConflict
	}
	s.record.State.Revision++
	s.record.State.Phase = agent.Running
	return copyRecord(s.record), true, nil
}
func (s *fakeStore) Commit(ctx context.Context, input agent.Record, expected uint64) (agent.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return agent.Record{}, err
	}
	if s.failCommit {
		return agent.Record{}, agent.ErrUnavailable
	}
	if s.record.State.Revision != expected {
		return agent.Record{}, agent.ErrConflict
	}
	input.State.Revision++
	s.record = copyRecord(input)
	return copyRecord(s.record), nil
}
func fixture(t *testing.T, actions ...agent.Action) (*Runtime, agent.Request, *fakeModel, *fakeTools, *fakeValidator, *fakeAuth, *fakeStore) {
	t.Helper()
	m := &fakeModel{actions: actions, quote: agent.Quote{Tokens: 5, CostMicros: 2, Currency: "USD", Known: true}}
	tools := &fakeTools{}
	validator := &fakeValidator{}
	auth := &fakeAuth{}
	store := &fakeStore{}
	definition := canonicalinspect.Definition()
	runtime, err := New(Config{Definition: commercetool.AgentDefinition{ID: "product.diagnose", Version: "v1.0.0", AllowedTools: []commercetool.ToolRef{definition.Ref}}, Tools: []commercetool.Definition{definition}, Model: m, Gateway: tools, Validator: validator, Authorizer: auth, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	r := agent.Request{Key: "request-1", Binding: agent.Binding{ContextKind: "acquisition", ContextID: "operation-1", ProductKey: "product-1", CatalogVersion: "1", PublicationID: "pub-1", TargetPlatform: "shein"}, PolicyVersion: "title-v1", PromptVersion: "prompt-v1", Limits: agent.Limits{Steps: 30, ModelCalls: 10, Tokens: 100, CostMicros: 100, Currency: "USD", Runtime: time.Minute}}
	return runtime, r, m, tools, validator, auth, store
}
func proposal(value string) agent.Action {
	return agent.Action{Kind: "propose", Candidate: enrichment.Candidate{Changes: []enrichment.FieldChange{{Field: "title", Value: value, EvidenceIDs: []string{"source-1"}}}}}
}

func TestGraphReadsProposesRepairsAndStopsForHuman(t *testing.T) {
	r, req, model, tools, validator, _, _ := fixture(t, agent.Action{Kind: "tool", Tool: canonicalinspect.Definition().Ref}, proposal("bad"), proposal("still bad"), proposal("supported title"))
	out, err := r.Start(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if out.State.Phase != agent.HumanReviewRequired || out.State.Validation == nil || !out.State.Validation.Valid || out.State.Repairs != 2 {
		t.Fatalf("unexpected outcome: %+v", out.State)
	}
	if model.calls != 4 || tools.calls != 1 || validator.calls != 3 || tools.last != req.Binding {
		t.Fatal("wrong consumer path")
	}
	if out.State.Validation.CandidateHash == "" {
		t.Fatal("candidate validation not bound")
	}
	if _, err = r.Start(context.Background(), req); err != nil || model.calls != 4 {
		t.Fatal("replay regenerated a proposal", err)
	}
}
func TestInvalidPatchAfterTwoRepairsStaysInvalid(t *testing.T) {
	r, req, m, _, _, _, _ := fixture(t, proposal("bad"), proposal("bad"), proposal("bad"))
	out, err := r.Start(context.Background(), req)
	if err != nil || m.calls != 3 || out.State.StopReason != agent.StopRepairLimit || out.State.Validation.Valid {
		t.Fatalf("wrong repair terminal: %+v %v", out.State, err)
	}
}
func TestInterruptResumePreservesBudgetAndPlatform(t *testing.T) {
	r, req, model, _, _, auth, _ := fixture(t, agent.Action{Kind: "interrupt", Unresolved: []string{"need user input"}}, proposal("supported title"))
	out, err := r.Start(context.Background(), req)
	if err != nil || out.State.Phase != agent.Interrupted || len(out.Checkpoint) == 0 {
		t.Fatalf("interrupt failed: %+v %v", out.State, err)
	}
	auth.denied = true
	if _, err := r.Resume(context.Background(), req, out.State.Revision, "answer"); err == nil || model.calls != 1 {
		t.Fatal("revoked caller resumed")
	}
	auth.denied = false
	changed := req
	changed.Binding.TargetPlatform = "amazon"
	if _, err := r.Resume(context.Background(), changed, out.State.Revision, "answer"); !errors.Is(err, agent.ErrConflict) || model.calls != 1 {
		t.Fatal("platform drift resumed", err)
	}
	resumed, err := r.Resume(context.Background(), req, out.State.Revision, "user clarification")
	if err != nil || resumed.State.Phase != agent.HumanReviewRequired || resumed.State.Usage.ModelCalls != 2 || resumed.State.Deadline != out.State.Deadline {
		t.Fatalf("bad resume: %+v %v", resumed.State, err)
	}
	if model.lastFeedback != "user clarification" {
		t.Fatal("resume lost the user input")
	}
	if _, err := r.Resume(context.Background(), req, out.State.Revision, "changed answer"); !errors.Is(err, agent.ErrConflict) || model.calls != 2 {
		t.Fatal("duplicate resume executed", err)
	}
}
func TestModelUnknownAndAuditFailureNeverRetry(t *testing.T) {
	t.Run("model response lost", func(t *testing.T) {
		r, req, m, _, _, _, _ := fixture(t)
		m.fail = true
		out, err := r.Start(context.Background(), req)
		if err != nil || out.State.StopReason != agent.StopModelUnknown || out.State.Usage.Tokens != 5 {
			t.Fatalf("unknown became free: %+v %v", out.State, err)
		}
		_, _ = r.Start(context.Background(), req)
		if m.calls != 1 {
			t.Fatal("model retried")
		}
	})
	t.Run("audit result", func(t *testing.T) {
		r, req, m, tools, _, _, _ := fixture(t, agent.Action{Kind: "tool", Tool: canonicalinspect.Definition().Ref})
		tools.auditFail = true
		out, err := r.Start(context.Background(), req)
		if err != nil || out.State.StopReason != agent.StopAudit || m.calls != 1 || tools.calls != 1 {
			t.Fatalf("audit failure ignored: %+v %v", out.State, err)
		}
		if out.State.History[len(out.State.History)-1].AuditStatus != commercetool.AuditStatusRecordFailed {
			t.Fatal("audit status discarded")
		}
	})
}
func TestRejectMissingPlatformAndUnallowedTool(t *testing.T) {
	r, req, model, tools, _, _, _ := fixture(t, agent.Action{Kind: "tool", Tool: commercetool.ToolRef{ID: "product.publish", Version: "v1.0.0"}})
	bad := req
	bad.Binding.TargetPlatform = ""
	if _, err := r.Start(context.Background(), bad); !errors.Is(err, agent.ErrInvalid) || model.calls != 0 {
		t.Fatal("missing platform executed")
	}
	out, err := r.Start(context.Background(), req)
	if err != nil || tools.calls != 0 || out.State.StopReason != agent.StopTool {
		t.Fatal("unallowed tool executed", err)
	}
}

func TestEveryBudgetStopsWithoutAnExtraCall(t *testing.T) {
	for _, test := range []struct {
		name         string
		change       func(*agent.Request, *fakeModel)
		action       agent.Action
		stop         agent.StopReason
		calls, tools int
	}{
		{"steps", func(r *agent.Request, _ *fakeModel) { r.Limits.Steps = 1 }, proposal("supported title"), agent.StopSteps, 1, 0},
		{"model calls", func(r *agent.Request, _ *fakeModel) { r.Limits.ModelCalls = 1 }, agent.Action{Kind: "tool", Tool: canonicalinspect.Definition().Ref}, agent.StopModelCalls, 1, 1},
		{"tokens", func(r *agent.Request, _ *fakeModel) { r.Limits.Tokens = 4 }, proposal("supported title"), agent.StopTokens, 0, 0},
		{"cost", func(r *agent.Request, _ *fakeModel) { r.Limits.CostMicros = 1 }, proposal("supported title"), agent.StopCost, 0, 0},
		{"runtime", func(r *agent.Request, _ *fakeModel) { r.Limits.Runtime = time.Nanosecond }, proposal("supported title"), agent.StopRuntime, 0, 0},
		{"currency", func(_ *agent.Request, m *fakeModel) { m.quote.Currency = "EUR" }, proposal("supported title"), agent.StopUsageUnknown, 0, 0},
		{"unknown quote", func(_ *agent.Request, m *fakeModel) { m.quote.Known = false }, proposal("supported title"), agent.StopUsageUnknown, 0, 0},
		{"unknown result usage", func(_ *agent.Request, m *fakeModel) { m.unknown = true }, proposal("supported title"), agent.StopUsageUnknown, 1, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, req, m, tools, _, _, _ := fixture(t, test.action)
			test.change(&req, m)
			if test.stop == agent.StopRuntime {
				// Let the durable deadline elapse even on clocks whose resolution
				// is coarser than the one-nanosecond budget.
				r.config.Store = afterClaimStore{Store: r.config.Store, afterClaim: func() { time.Sleep(time.Millisecond) }}
			}
			out, err := r.Start(context.Background(), req)
			if err != nil || out.State.StopReason != test.stop || m.calls != test.calls || tools.calls != test.tools {
				t.Fatalf("out=%+v calls=%d tools=%d err=%v", out.State, m.calls, tools.calls, err)
			}
		})
	}
}

func TestFailedCheckpointCommitCannotBeResumedOrRegenerated(t *testing.T) {
	r, req, m, _, _, _, store := fixture(t, agent.Action{Kind: "interrupt"})
	store.failCommit = true
	if _, err := r.Start(context.Background(), req); !errors.Is(err, agent.ErrUnavailable) {
		t.Fatal("save failure acknowledged", err)
	}
	store.failCommit = false
	out, err := r.Start(context.Background(), req)
	if err != nil || out.State.Phase != agent.Running || m.calls != 1 || len(out.Checkpoint) != 0 {
		t.Fatal("ambiguous run replayed", err)
	}
	if _, err := r.Resume(context.Background(), req, out.State.Revision, "answer"); !errors.Is(err, agent.ErrConflict) {
		t.Fatal("uncommitted checkpoint resumed", err)
	}
}

func TestResumeKeepsOriginalRuntimeDeadline(t *testing.T) {
	r, req, m, _, _, _, store := fixture(t, agent.Action{Kind: "interrupt"})
	out, err := r.Start(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	// Fake durable clock condition: both control and SDK state retain the old
	// deadline; an already-expired caller deadline must prevent any new model.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, _ = r.Resume(ctx, req, out.State.Revision, "answer")
	if m.calls != 1 || store.record.State.Deadline != out.State.Deadline {
		t.Fatal("resume reset elapsed budget")
	}
}

func TestConcurrentResumeHasOneExecution(t *testing.T) {
	r, req, m, _, _, _, _ := fixture(t, agent.Action{Kind: "interrupt"}, proposal("supported title"))
	out, err := r.Start(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for range 2 {
		go func() { _, err := r.Resume(context.Background(), req, out.State.Revision, "answer"); results <- err }()
	}
	first, second := <-results, <-results
	if (first == nil) == (second == nil) || m.calls != 2 {
		t.Fatalf("duplicate execution: %v %v calls=%d", first, second, m.calls)
	}
}

func TestResultKeepsUnknownConfidenceAndImmutableEvidence(t *testing.T) {
	r, req, model, _, _, _, _ := fixture(t, agent.Action{Kind: "tool", Tool: canonicalinspect.Definition().Ref}, proposal("supported title"))
	model.mutateHistory = true
	out, err := r.Start(context.Background(), req)
	if err != nil || out.State.Phase != agent.HumanReviewRequired || !out.State.HumanReviewRequired {
		t.Fatal("proposal lost human gate", err)
	}
	if len(out.State.Confidence) != 1 || out.State.Confidence[0].Known || out.State.Confidence[0].Field != "title" {
		t.Fatal("unknown confidence fabricated")
	}
	for _, observation := range out.State.History {
		if len(observation.Output) > 0 && !json.Valid(observation.Output) {
			t.Fatal("model mutated stored tool evidence")
		}
	}
}

func TestAdapterPanicAfterDispatchKeepsUnknownOutcome(t *testing.T) {
	r, req, m, _, _, _, _ := fixture(t)
	m.panicAfterDispatch = true
	out, err := r.Start(context.Background(), req)
	if err != nil || out.State.StopReason != agent.StopModelUnknown || m.calls != 1 {
		t.Fatalf("panic lost paid outcome: %+v %v", out.State, err)
	}
}

func TestCancellationDuringModelDoesNotRepeatDispatch(t *testing.T) {
	r, req, m, _, _, _, _ := fixture(t)
	m.waitStarted = make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan agent.Record, 1)
	go func() { out, _ := r.Start(ctx, req); result <- out }()
	<-m.waitStarted
	cancel()
	out := <-result
	if m.calls != 1 || out.State.StopReason != agent.StopModelUnknown {
		t.Fatalf("cancel lost invocation: %+v", out.State)
	}
}

type afterClaimStore struct {
	agent.Store
	afterClaim func()
}

func (s afterClaimStore) Claim(ctx context.Context, record agent.Record, expected uint64) (agent.Record, bool, error) {
	out, acquired, err := s.Store.Claim(ctx, record, expected)
	if err == nil && acquired {
		s.afterClaim()
	}
	return out, acquired, err
}

type checkingCommitStore struct {
	agent.Store
	check func(context.Context)
}

func (s checkingCommitStore) Commit(ctx context.Context, record agent.Record, expected uint64) (agent.Record, error) {
	s.check(ctx)
	return s.Store.Commit(ctx, record, expected)
}

func TestCancelledRunCommitIsBoundedAndDoesNotAcknowledgeFailure(t *testing.T) {
	r, req, _, _, _, _, store := fixture(t)
	store.failCommit = true
	type contextKey struct{}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), contextKey{}, "trace-value"))
	defer cancel()
	checked := false
	r.config.Store = afterClaimStore{Store: checkingCommitStore{Store: store, check: func(commitCtx context.Context) {
		checked = true
		if commitCtx.Err() != nil || commitCtx.Value(contextKey{}) != "trace-value" {
			t.Fatal("final commit must preserve values without inheriting cancellation")
		}
		deadline, ok := commitCtx.Deadline()
		if remaining := time.Until(deadline); !ok || remaining <= 0 || remaining > 2*time.Second {
			t.Fatal("final commit must have its own short deadline")
		}
	}}, afterClaim: cancel}
	out, err := r.Start(ctx, req)
	if !checked || !errors.Is(err, agent.ErrUnavailable) || out.State.RunID != "" {
		t.Fatalf("failed final commit acknowledged: %+v %v", out, err)
	}
	if store.record.State.Phase != agent.Running || store.record.State.Revision != 1 {
		t.Fatal("failed commit changed durable state")
	}
}

func TestNotDispatchedDecisionReleasesOnlyCurrentQuote(t *testing.T) {
	r, req, model, _, _, _, _ := fixture(t, agent.Action{Kind: "tool", Tool: canonicalinspect.Definition().Ref})
	model.notDispatchedAt = 2
	out, err := r.Start(context.Background(), req)
	if err != nil || out.State.StopReason != agent.StopDependency || out.State.PendingInvocationID != "" {
		t.Fatalf("known unexecuted call remained pending: %+v %v", out.State, err)
	}
	if out.State.Usage.Tokens != 2 || out.State.Usage.CostMicros != 1 || model.calls != 2 {
		t.Fatalf("changed prior consumption or retried: %+v calls=%d", out.State.Usage, model.calls)
	}
	last := out.State.History[len(out.State.History)-1]
	if last.InvocationID == "" || last.ObservedUsage == nil || !last.ObservedUsage.Known || last.ObservedUsage.Tokens != 0 {
		t.Fatal("lost known no-dispatch evidence")
	}
	_, err = r.Start(context.Background(), req)
	if err != nil || model.calls != 2 {
		t.Fatal("terminal no-dispatch replayed", err)
	}
}
