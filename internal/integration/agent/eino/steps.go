package einoruntime

import (
	"context"
	"encoding/json"
	"math"

	"github.com/cloudwego/eino/compose"

	"task-processor/internal/agent"
	"task-processor/internal/commercetool"
)

func (e *execution) model(ctx context.Context, s *flowState) {
	limits := s.State.Request.Limits
	if s.State.Usage.Steps >= limits.Steps {
		s.stop(agent.StopSteps)
		return
	}
	if s.State.Usage.ModelCalls >= limits.ModelCalls {
		s.stop(agent.StopModelCalls)
		return
	}
	in := agent.ModelInput{Binding: s.State.Request.Binding, PolicyVersion: s.State.Request.PolicyVersion, PromptVersion: s.State.Request.PromptVersion, History: s.State.History, Validation: s.State.Validation, UserFeedback: s.State.UserFeedback, AgentRunID: s.State.RunID, AgentID: e.runtime.config.Definition.ID, AgentVersion: e.runtime.config.Definition.Version, TraceID: s.State.TraceID}
	quote, err := e.runtime.config.Model.Quote(ctx, clone(in))
	if err != nil {
		s.stop(agent.StopDependency)
		return
	}
	if !e.guard(ctx, s) {
		return
	}
	if stop := s.State.Usage.Reserve(limits, quote); stop != "" {
		s.stop(stop)
		return
	}
	in.UpperBound = quote
	in.InvocationID = callID(s)
	// Record dispatch identity before invoking the port, including on error.
	s.State.History = append(s.State.History, agent.Observation{Step: s.State.Usage.Steps, CallID: in.InvocationID, InvocationID: in.InvocationID})
	s.State.PendingInvocationID = in.InvocationID
	result, err := e.runtime.config.Model.Decide(ctx, clone(in))
	s.State.History[len(s.State.History)-1].ObservedUsage = &result.Usage
	if err != nil || result.InvocationID != in.InvocationID {
		s.stop(agent.StopModelUnknown)
		return
	}
	s.State.PendingInvocationID = ""
	if stop := s.State.Usage.Settle(quote, result.Usage); stop != "" {
		s.stop(stop)
		return
	}
	if !e.guard(ctx, s) {
		return
	}
	raw, err := json.Marshal(result.Action)
	if err != nil || len(raw) > agent.MaxModelOutputBytes {
		s.stop(agent.StopInvalidOutput)
		return
	}
	s.Action = clone(result.Action)
	s.State.Unresolved = append([]string(nil), result.Action.Unresolved...)
	switch result.Action.Kind {
	case "tool":
		s.Next = "tool"
	case "propose":
		s.State.Candidate = clone(result.Action.Candidate)
		confidence, ok := fieldConfidence(result.Action)
		if !ok {
			s.stop(agent.StopInvalidOutput)
			return
		}
		s.State.Confidence = confidence
		s.State.Validation = nil
		s.Next = "validate"
	case "interrupt":
		s.Next = "resume"
	default:
		s.stop(agent.StopInvalidOutput)
	}
}

func (e *execution) tool(ctx context.Context, s *flowState) {
	def, exists := e.runtime.tools[s.Action.Tool]
	if !exists {
		s.stop(agent.StopTool)
		return
	}
	if stop := s.State.Usage.Step(s.State.Request.Limits); stop != "" {
		s.stop(stop)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, def.Timeout.Duration)
	defer cancel()
	meta := commercetool.CallMetadata{CallID: callID(s), AgentID: e.runtime.config.Definition.ID, AgentVersion: e.runtime.config.Definition.Version, AgentRunID: s.State.RunID, BusinessTaskID: s.State.Request.Binding.ContextID, TraceID: s.State.TraceID}
	result, err := e.runtime.config.Gateway.Invoke(ctx, def.Ref, meta, s.State.Request.Binding)
	s.State.History = append(s.State.History, agent.Observation{Step: s.State.Usage.Steps, Tool: def.Ref, CallID: meta.CallID, InvocationID: result.AIInvocationID, Output: append(json.RawMessage(nil), result.Output...), AuditStatus: result.AuditStatus})
	if result.AuditStatus != commercetool.AuditStatusRecorded {
		s.stop(agent.StopAudit)
		return
	}
	if err != nil || ctx.Err() != nil || result.AIInvocationID != "" || !json.Valid(result.Output) {
		s.stop(agent.StopTool)
		return
	}
	s.Action = agent.Action{}
	s.Next = "model"
}

func (e *execution) validate(ctx context.Context, s *flowState) {
	if stop := s.State.Usage.Step(s.State.Request.Limits); stop != "" {
		s.stop(stop)
		return
	}
	hash, err := digest(s.State.Candidate)
	if err != nil {
		s.stop(agent.StopInvalidOutput)
		return
	}
	result, err := e.runtime.config.Validator.Validate(ctx, s.State.Request.Binding, s.State.Request.PolicyVersion, clone(s.State.Candidate))
	if err != nil {
		s.stop(agent.StopDependency)
		return
	}
	if !e.guard(ctx, s) {
		return
	}
	if result.PolicyVersion != s.State.Request.PolicyVersion {
		s.stop(agent.StopInvalidOutput)
		return
	}
	result.CandidateHash = hash
	result = clone(result)
	s.State.Validation = &result
	if result.Valid || s.State.Repairs >= 2 {
		s.State.Phase = agent.HumanReviewRequired
		s.Next = compose.END
		if !result.Valid {
			s.State.StopReason = agent.StopRepairLimit
		}
		return
	}
	s.State.Repairs++
	s.Action = agent.Action{}
	s.Next = "model"
}

// All values passed here have already been bounded and JSON-validated. Copies
// keep caller-owned slices from rewriting earlier evidence or validation.
func clone[T any](value T) T {
	raw, err := json.Marshal(value)
	if err != nil {
		panic("invalid internal agent value")
	}
	var result T
	if err := json.Unmarshal(raw, &result); err != nil {
		panic("invalid internal agent copy")
	}
	return result
}

func fieldConfidence(action agent.Action) ([]agent.FieldConfidence, bool) {
	provided := make(map[string]agent.FieldConfidence)
	for _, c := range action.Confidence {
		if _, duplicate := provided[c.Field]; duplicate || !agent.ValidID(c.Field) || math.IsNaN(c.Value) || math.IsInf(c.Value, 0) || c.Value < 0 || c.Value > 1 || !c.Known && c.Value != 0 {
			return nil, false
		}
		provided[c.Field] = c
	}
	result := make([]agent.FieldConfidence, 0, len(action.Candidate.Changes))
	for _, change := range action.Candidate.Changes {
		c, exists := provided[change.Field]
		if !exists {
			c = agent.FieldConfidence{Field: change.Field}
		}
		result = append(result, c)
		delete(provided, change.Field)
	}
	return result, len(provided) == 0
}
