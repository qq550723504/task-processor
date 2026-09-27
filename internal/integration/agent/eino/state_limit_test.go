package einoruntime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"task-processor/internal/agent"
	"task-processor/internal/commercetool"
	"task-processor/internal/product/catalog/tools/canonicalinspect"
)

func assertBoundedRecord(t *testing.T, record agent.Record) {
	t.Helper()
	raw, err := json.Marshal(record)
	if err != nil || len(raw) > agent.MaxStateBytes {
		t.Fatalf("invalid committed record: bytes=%d err=%v", len(raw), err)
	}
}

func TestAccumulatedToolOutputPreservesCallEvidence(t *testing.T) {
	action := agent.Action{Kind: "tool", Tool: canonicalinspect.Definition().Ref}
	r, req, model, tools, _, _, _ := fixture(t, action, action, action)
	tools.output = json.RawMessage(`"` + strings.Repeat("a", 700<<10) + `"`)
	out, err := r.Start(context.Background(), req)
	if err != nil || out.State.StopReason != agent.StopTooLarge || model.calls != 3 || tools.calls != 3 {
		t.Fatalf("wrong terminal: reason=%s model=%d tools=%d err=%v", out.State.StopReason, model.calls, tools.calls, err)
	}
	if len(out.State.History) != 6 {
		t.Fatalf("lost completed call evidence: got %d observations", len(out.State.History))
	}
	for i, observation := range out.State.History {
		if observation.CallID == "" || i%2 == 0 && observation.InvocationID == "" || i%2 == 1 && observation.AuditStatus != commercetool.AuditStatusRecorded {
			t.Fatalf("lost call reference at observation %d", i)
		}
	}
	if len(out.State.History[1].Output) == 0 || len(out.State.History[3].Output) == 0 || len(out.State.History[5].Output) != 0 {
		t.Fatal("must preserve earlier outputs and reject only the new oversized payload")
	}
	if rejected := out.State.RejectedPayload; rejected == nil || rejected.CallID != out.State.History[5].CallID || len(rejected.SHA256) != 64 || rejected.Bytes != len(tools.output) || rejected.Reason != "state_limit" {
		t.Fatal("missing explicit rejected-payload reference")
	}
	assertBoundedRecord(t, out)
}

func TestOversizedValidationDoesNotCommitUnboundedOrEraseCandidate(t *testing.T) {
	r, req, _, _, validator, _, _ := fixture(t, proposal("supported title"))
	validator.unresolved = []string{strings.Repeat("x", agent.MaxStateBytes)}
	out, err := r.Start(context.Background(), req)
	if err != nil || out.State.StopReason != agent.StopTooLarge || len(out.State.History) != 1 {
		t.Fatalf("lost invocation or wrong stop: history=%d reason=%s err=%v", len(out.State.History), out.State.StopReason, err)
	}
	if len(out.State.Candidate.Changes) != 1 || out.State.Validation != nil {
		t.Fatal("oversized validation must leave candidate intact and unvalidated")
	}
	if out.State.RejectedPayload == nil || out.State.RejectedPayload.Kind != "validation" {
		t.Fatal("missing rejected validation summary")
	}
	assertBoundedRecord(t, out)
}

func TestMalformedToolJSONPreservesFailureAndAuditReference(t *testing.T) {
	for _, auditFail := range []bool{false, true} {
		r, req, _, tools, _, _, _ := fixture(t, agent.Action{Kind: "tool", Tool: canonicalinspect.Definition().Ref})
		tools.output = json.RawMessage(`{"broken":`)
		tools.auditFail = auditFail
		out, err := r.Start(context.Background(), req)
		want := agent.StopTool
		if auditFail {
			want = agent.StopAudit
		}
		if err != nil || out.State.StopReason != want || len(out.State.History) != 2 {
			t.Fatalf("encoding failure erased source failure: reason=%s observations=%d err=%v", out.State.StopReason, len(out.State.History), err)
		}
		last := out.State.History[1]
		if last.CallID == "" || last.AuditStatus == "" || len(last.Output) != 0 {
			t.Fatal("invalid JSON retained or audit reference erased")
		}
		assertBoundedRecord(t, out)
	}
}

func TestOverflowPreservesCandidateValidationAssociation(t *testing.T) {
	action := agent.Action{Kind: "tool", Tool: canonicalinspect.Definition().Ref}
	r, req, _, tools, _, _, _ := fixture(t, proposal("bad"), action, action, action)
	tools.output = json.RawMessage(`"` + strings.Repeat("a", 700<<10) + `"`)
	out, err := r.Start(context.Background(), req)
	if err != nil || out.State.StopReason != agent.StopTooLarge || len(out.State.Candidate.Changes) != 1 || out.State.Validation == nil {
		t.Fatal("overflow erased the candidate or its validation", err)
	}
	hash, err := digest(out.State.Candidate)
	if err != nil || hash != out.State.Validation.CandidateHash || out.State.Validation.Valid {
		t.Fatal("validation no longer refers to the retained candidate")
	}
	assertBoundedRecord(t, out)
}

func TestOversizedCheckpointStopsWithoutLosingCallEvidence(t *testing.T) {
	action := agent.Action{Kind: "tool", Tool: canonicalinspect.Definition().Ref}
	r, req, model, tools, _, _, _ := fixture(t, action, agent.Action{Kind: "interrupt"})
	tools.output = json.RawMessage(`"` + strings.Repeat("a", 900<<10) + `"`)
	out, err := r.Start(context.Background(), req)
	if err != nil || out.State.StopReason != agent.StopTooLarge || out.State.Phase != agent.Stopped || len(out.Checkpoint) != 0 {
		t.Fatalf("oversized checkpoint exposed as resumable: phase=%s reason=%s bytes=%d err=%v", out.State.Phase, out.State.StopReason, len(out.Checkpoint), err)
	}
	if len(out.State.History) != 3 || len(out.State.History[1].Output) != len(tools.output) || model.calls != 2 {
		t.Fatal("SDK checkpoint size failure lost call evidence")
	}
	assertBoundedRecord(t, out)
}
