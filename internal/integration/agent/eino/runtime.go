// Package einoruntime adapts the bounded Agent contract to Eino's graph and
// checkpoint engine. It has no model client, database, HTTP route or defaults.
package einoruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"task-processor/internal/agent"
	"task-processor/internal/commercetool"
)

type Config struct {
	Definition commercetool.AgentDefinition
	Tools      []commercetool.Definition
	Model      agent.GovernedModel
	Gateway    agent.ToolGateway
	Validator  agent.Validator
	Authorizer agent.Authorizer
	Store      agent.Store
}
type Runtime struct {
	config Config
	tools  map[commercetool.ToolRef]commercetool.Definition
}

func New(config Config) (*Runtime, error) {
	for _, port := range []any{config.Model, config.Gateway, config.Validator, config.Authorizer, config.Store} {
		if nilPort(port) {
			return nil, agent.ErrUnavailable
		}
	}
	if err := (commercetool.AgentRef{ID: config.Definition.ID, Version: config.Definition.Version}).Validate(); err != nil {
		return nil, agent.ErrInvalid
	}
	refs := make(map[commercetool.ToolRef]bool)
	for _, ref := range config.Definition.AllowedTools {
		if ref.Validate() != nil || refs[ref] {
			return nil, agent.ErrInvalid
		}
		refs[ref] = true
	}
	if len(refs) == 0 || len(refs) != len(config.Tools) {
		return nil, agent.ErrInvalid
	}
	tools := make(map[commercetool.ToolRef]commercetool.Definition)
	for _, def := range config.Tools {
		// AI-backed propose tools require a metered gateway (#134); the current
		// adapter cannot infer usage from AIInvocationID. Fail before execution.
		if !refs[def.Ref] || tools[def.Ref].Ref.ID != "" ||
			(def.Risk != commercetool.RiskRead && def.Risk != commercetool.RiskCompute) ||
			def.Usage.Owner != commercetool.UsageOwnerUnmetered || def.SideEffects.Mode != commercetool.SideEffectNone || def.Timeout.Duration <= 0 {
			return nil, agent.ErrInvalid
		}
		tools[def.Ref] = def
	}
	config.Definition.AllowedTools = append([]commercetool.ToolRef(nil), config.Definition.AllowedTools...)
	return &Runtime{config: config, tools: tools}, nil
}

func nilPort(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Func, reflect.Chan, reflect.Interface, reflect.Slice:
		return v.IsNil()
	}
	return false
}

func (r *Runtime) Start(ctx context.Context, request agent.Request) (agent.Record, error) {
	return r.run(ctx, request, 0, "")
}
func (r *Runtime) Resume(ctx context.Context, request agent.Request, revision uint64, feedback string) (agent.Record, error) {
	if revision == 0 || len(feedback) > 8<<10 || !utf8.ValidString(feedback) {
		return agent.Record{}, agent.ErrInvalid
	}
	return r.run(ctx, request, revision, feedback)
}

func (r *Runtime) run(ctx context.Context, request agent.Request, expected uint64, feedback string) (agent.Record, error) {
	if ctx == nil || r == nil || !request.Binding.Valid() || !agent.ValidID(request.Key) || !agent.ValidID(request.PolicyVersion) || !agent.ValidID(request.PromptVersion) || !request.Limits.Valid() || request.Limits.Steps > (1<<29) {
		return agent.Record{}, agent.ErrInvalid
	}
	scope, err := r.config.Authorizer.Authorize(ctx, request.Binding)
	if err != nil || !agent.ValidID(scope.OrganizationID) || !agent.ValidID(scope.ActorID) {
		return agent.Record{}, agent.ErrInvalid
	}
	fingerprint, err := digest(struct {
		Scope      agent.Scope
		Request    agent.Request
		Definition commercetool.AgentDefinition
	}{scope, request, r.config.Definition})
	if err != nil {
		return agent.Record{}, agent.ErrInvalid
	}
	now := time.Now().UTC()
	initial := agent.Record{State: agent.State{RunID: uuid.NewString(), Scope: scope, Request: request, Fingerprint: fingerprint, Phase: agent.Running, StartedAt: now, Deadline: now.Add(request.Limits.Runtime)}}
	initial.State.HumanReviewRequired = true
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		initial.State.TraceID = span.TraceID().String()
	}
	record, acquired, err := r.config.Store.Claim(ctx, initial, expected)
	if err != nil {
		return agent.Record{}, err
	}
	if record.State.Fingerprint != fingerprint || record.State.Scope != scope || record.State.Request != request || !agent.ValidID(record.State.RunID) || record.State.Revision == 0 {
		return agent.Record{}, agent.ErrConflict
	}
	if !acquired {
		return record, nil
	}
	if record.State.Phase != agent.Running || expected > 0 && len(record.Checkpoint) == 0 || len(record.Checkpoint) > agent.MaxStateBytes {
		return agent.Record{}, agent.ErrConflict
	}
	ctx, cancel := context.WithDeadline(ctx, record.State.Deadline)
	defer cancel()
	checkpoint := &checkpointBuffer{key: record.State.RunID, data: append([]byte(nil), record.Checkpoint...)}
	flow := &execution{runtime: r, record: record, last: &flowState{State: record.State, Next: "model"}, checkpoint: checkpoint, feedback: feedback}
	graph, err := flow.graph(ctx)
	if err != nil {
		return agent.Record{}, fmt.Errorf("%w: graph construction: %v", agent.ErrUnavailable, err)
	}
	result, graphErr := graph.Invoke(ctx, flow.last, compose.WithCheckPointID(record.State.RunID))
	if result != nil {
		flow.last = result
	}
	if _, interrupted := compose.ExtractInterruptInfo(graphErr); interrupted && checkpoint.saved {
		flow.last.State.Phase = agent.Interrupted
	} else if graphErr != nil {
		flow.last.stop(agent.StopDependency)
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			flow.last.stop(agent.StopRuntime)
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			flow.last.stop(agent.StopCancelled)
		}
		if flow.last.State.PendingInvocationID != "" {
			flow.last.stop(agent.StopModelUnknown)
		}
	}
	final := agent.Record{State: flow.last.State}
	final.State.Revision = record.State.Revision
	if final.State.Phase == agent.Interrupted {
		final.Checkpoint = checkpoint.data
	}
	// A canceled request is not permission to acknowledge a checkpoint or result
	// that the store could not commit. No background write/replay is introduced.
	return r.config.Store.Commit(ctx, final, record.State.Revision)
}

func digest(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

type flowState struct {
	State  agent.State
	Action agent.Action
	Next   string
}

func init() { schema.RegisterName[*flowState]("task_processor_product_agent_flow_v1") }
func (s *flowState) stop(reason agent.StopReason) {
	s.State.Phase = agent.Stopped
	s.State.StopReason = reason
	s.Next = compose.END
}

type execution struct {
	runtime    *Runtime
	record     agent.Record
	last       *flowState
	checkpoint *checkpointBuffer
	feedback   string
}

func (e *execution) graph(ctx context.Context) (compose.Runnable[*flowState, *flowState], error) {
	g := compose.NewGraph[*flowState, *flowState]()
	nodes := map[string]func(context.Context, *flowState){"model": e.model, "tool": e.tool, "validate": e.validate, "resume": e.resume}
	for name, action := range nodes {
		if err := g.AddLambdaNode(name, compose.InvokableLambda(func(ctx context.Context, s *flowState) (*flowState, error) {
			e.last = s
			if e.guard(ctx, s) {
				action(ctx, s)
			}
			raw, err := json.Marshal(s)
			if err != nil || len(raw) > agent.MaxStateBytes {
				s.stop(agent.StopTooLarge)
				s.State.History = nil
				s.State.Candidate = agent.Action{}.Candidate
				s.Action = agent.Action{}
			}
			return s, nil
		})); err != nil {
			return nil, err
		}
	}
	for name := range nodes {
		if err := g.AddBranch(name, compose.NewGraphBranch(func(_ context.Context, s *flowState) (string, error) { return s.Next, nil }, map[string]bool{"model": true, "tool": true, "validate": true, "resume": true, compose.END: true})); err != nil {
			return nil, err
		}
	}
	if err := g.AddEdge(compose.START, "model"); err != nil {
		return nil, err
	}
	return g.Compile(ctx, compose.WithCheckPointStore(e.checkpoint), compose.WithInterruptBeforeNodes([]string{"resume"}), compose.WithMaxRunSteps(e.record.State.Request.Limits.Steps*2+5))
}

func (e *execution) guard(ctx context.Context, s *flowState) bool {
	// Restored framework state must match the freshly claimed run envelope.
	if s.State.Fingerprint != e.record.State.Fingerprint || s.State.Request != e.record.State.Request || s.State.Scope != e.record.State.Scope || s.State.RunID != e.record.State.RunID || !s.State.Deadline.Equal(e.record.State.Deadline) {
		s.stop(agent.StopUnauthorized)
		return false
	}
	s.State.Revision = e.record.State.Revision
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.Canceled) {
			s.stop(agent.StopCancelled)
		} else {
			s.stop(agent.StopRuntime)
		}
		return false
	}
	scope, err := e.runtime.config.Authorizer.Authorize(ctx, s.State.Request.Binding)
	if err != nil || scope != s.State.Scope {
		s.stop(agent.StopUnauthorized)
		return false
	}
	if err := ctx.Err(); err != nil {
		s.stop(agent.StopRuntime)
		return false
	}
	return true
}

func (e *execution) resume(_ context.Context, s *flowState) {
	s.State.Phase = agent.Running
	s.State.UserFeedback = e.feedback
	s.Action = agent.Action{}
	s.Next = "model"
}

type checkpointBuffer struct {
	key   string
	data  []byte
	saved bool
}

func (b *checkpointBuffer) Get(_ context.Context, key string) ([]byte, bool, error) {
	if key != b.key {
		return nil, false, agent.ErrConflict
	}
	return append([]byte(nil), b.data...), len(b.data) > 0, nil
}
func (b *checkpointBuffer) Set(ctx context.Context, key string, data []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if key != b.key || len(data) > agent.MaxStateBytes {
		return agent.ErrInvalid
	}
	b.data = append([]byte(nil), data...)
	b.saved = true
	return nil
}

func callID(s *flowState) string {
	return fmt.Sprintf("%s:step:%d", s.State.RunID, s.State.Usage.Steps)
}
