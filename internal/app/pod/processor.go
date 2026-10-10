package podapp

import (
	"context"
	"errors"
	"reflect"
	podpersistence "task-processor/internal/integration/persistence/product/pod"
	"task-processor/internal/integration/sds"
	"task-processor/internal/listing/submission"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/pod"
	"time"
)

type MutationKernel interface {
	submission.ExecutionIntentReader
	Acquire(context.Context, submission.AcquireExecutionCommand) (submission.ExecutionAcquisition, error)
	MarkUnknown(context.Context, submission.ExecutionClaim, submission.UnknownReason) (submission.ExecutionAttempt, error)
}
type Mutations interface {
	Upload(context.Context, pod.Plan, []byte, *pod.MutationPermit) (pod.ObjectReceipt, error)
	CreateMaterial(context.Context, pod.Plan, pod.ObjectReceipt, *pod.MutationPermit) (pod.MaterialReceipt, error)
	Sync(context.Context, pod.Plan, []byte, *pod.MutationPermit) error
}
type Templates interface {
	List(context.Context, int, int, string) (pod.TemplatePage, error)
	Detail(context.Context, string) (pod.Template, error)
	Manifest(context.Context, pod.AccountBinding, string, string) (pod.TemplateManifest, error)
}
type Observer interface {
	Observe(context.Context, sds.Binding, pod.DesignIntent) (pod.QualifiedFinished, error)
}
type Processor struct {
	Repository  *podpersistence.Repository
	Inputs      OriginalInputs
	Templates   Templates
	Credentials sds.CredentialSource
	Kernel      MutationKernel
	Mutations   Mutations
	Observer    Observer
}

// Activity input/output contain only original identities and bounded progress.
// No auth context, image bytes, signatures, payloads or SendPermit enter history.
type Execution struct {
	Scope       collection.Scope
	OperationID string
}
type ExecutionResult struct {
	Done       bool
	Wait       bool
	Unknown    bool
	NotStarted bool
}

// Only a definitive no-attempt read permits another pre-send check. Once any
// original attempt exists, the existing Kernel owns its non-replayable result.
func (p *Processor) preSendFailure(ctx context.Context, o pod.Operation, step string) ExecutionResult {
	if step == pod.StepOSS && o.Object == nil && o.Material == nil && o.Intent == nil {
		_, e := p.Kernel.ReadIntent(ctx, submission.ExecutionScope{OrganizationID: o.Plan.Scope.OrganizationID}, pod.StepIntent(o.Plan.OperationID, pod.StepOSS))
		if errors.Is(e, submission.ErrExecutionNotFound) {
			return ExecutionResult{NotStarted: true}
		}
	}
	return ExecutionResult{Unknown: true}
}

func (p *Processor) recheck(ctx context.Context, o pod.Operation) ([]byte, error) {
	if e := p.Repository.Check(ctx, o, p.Inputs.Guard); e != nil {
		return nil, e
	}
	credentials, e := p.Credentials.Current(ctx)
	if e != nil || credentials.BindingID != o.Plan.Binding.ID || credentials.Revision != o.Plan.Binding.Revision || credentials.MerchantID != o.Plan.Binding.MerchantID {
		return nil, pod.ErrUnavailable
	}
	manifest, e := p.Templates.Manifest(ctx, o.Plan.Binding, o.Plan.Template.ParentID, o.Plan.Template.VariantID)
	if e != nil || !reflect.DeepEqual(manifest, o.Plan.Template) {
		return nil, pod.ErrConflict
	}
	raw, _, e := p.Inputs.bytes(ctx, o.Plan.Scope, o.Plan.Artwork)
	if e != nil {
		return nil, e
	}
	if e = p.Repository.Check(ctx, o, p.Inputs.Guard); e != nil {
		return nil, e
	}
	return raw, nil
}
func (p *Processor) send(ctx context.Context, o pod.Operation, step string, command submission.AcquireExecutionCommand, raw []byte, guard func(context.Context) error) (any, error) {
	a, e := p.Kernel.Acquire(ctx, command)
	if e != nil {
		return nil, pod.ErrUnknown
	}
	if a.Permit == nil {
		return nil, pod.ErrUnknown
	}
	// After a committed permit, any cancellation/revocation remains non-replayable.
	if guard(ctx) != nil {
		_, _ = p.Kernel.MarkUnknown(ctx, pod.PermitClaim(o.Plan.Scope, a.Permit), submission.UnknownExecutionCancelled)
		return nil, pod.ErrForbidden
	}
	var value any
	permit := pod.TransportPermit(a.Permit)
	switch step {
	case pod.StepOSS:
		value, e = p.Mutations.Upload(ctx, o.Plan, raw, permit)
	case pod.StepMaterial:
		if o.Object == nil {
			e = pod.ErrInvalid
		} else {
			value, e = p.Mutations.CreateMaterial(ctx, o.Plan, *o.Object, permit)
		}
	case pod.StepSync:
		e = p.Mutations.Sync(ctx, o.Plan, o.Payload, permit)
	default:
		e = pod.ErrInvalid
	}
	if e == nil && step != pod.StepSync {
		e = p.Repository.SaveStep(ctx, o, step, value, completeStep(step, a.Permit, value), p.Inputs.Guard)
	}
	if e != nil {
		_, _ = p.Kernel.MarkUnknown(ctx, pod.PermitClaim(o.Plan.Scope, a.Permit), submission.UnknownResponseLost)
		return nil, pod.ErrUnknown
	}
	return value, nil
}
func (p *Processor) Process(ctx context.Context, in Execution) (ExecutionResult, error) {
	if ctx == nil || p == nil || p.Repository == nil || p.Kernel == nil || p.Mutations == nil || p.Observer == nil || p.Templates == nil || p.Credentials == nil || in.Scope.Validate() != nil || !collection.ValidID(in.OperationID) {
		return ExecutionResult{}, pod.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	o, e := p.Repository.Read(ctx, in.Scope, in.OperationID)
	if e != nil {
		return ExecutionResult{}, e
	}
	if o.Finished != nil {
		return ExecutionResult{Done: true}, nil
	}
	for _, step := range []string{pod.StepOSS, pod.StepMaterial} {
		if step == pod.StepOSS && o.Object != nil || step == pod.StepMaterial && o.Material != nil {
			continue
		}
		raw, e := p.recheck(ctx, o)
		if e != nil {
			return p.preSendFailure(ctx, o, step), nil
		}
		command, e := pod.StepCommand(o, step, "pod-worker")
		if e != nil {
			return ExecutionResult{}, e
		}
		_, e = p.send(ctx, o, step, command, raw, func(ctx context.Context) error { _, e := p.recheck(ctx, o); return e })
		if e != nil {
			return ExecutionResult{Unknown: true}, nil
		}
		o, e = p.Repository.Read(ctx, in.Scope, in.OperationID)
		if e != nil {
			return ExecutionResult{}, e
		}
	}
	if o.Intent == nil {
		o, e = p.Repository.FreezeSync(ctx, o, p.Inputs.Guard)
		if e != nil {
			return ExecutionResult{Unknown: true}, nil
		}
	}
	raw, e := p.recheck(ctx, o)
	if e != nil {
		return ExecutionResult{Unknown: true}, nil
	}
	command, e := pod.StepCommand(o, pod.StepSync, "pod-worker")
	if e != nil {
		return ExecutionResult{}, e
	}
	_, _ = p.send(ctx, o, pod.StepSync, command, raw, func(ctx context.Context) error { _, e := p.recheck(ctx, o); return e })
	return p.Observe(ctx, in)
}

// Observe is the only recovery operation exposed to a request. It performs no
// provider mutation and never starts a new attempt or releases by elapsed time.
func (p *Processor) Observe(ctx context.Context, in Execution) (ExecutionResult, error) {
	o, e := p.Repository.Read(ctx, in.Scope, in.OperationID)
	if e != nil {
		return ExecutionResult{}, e
	}
	if o.Finished != nil {
		return ExecutionResult{Done: true}, nil
	}
	if o.Intent == nil || o.Material == nil || o.Object == nil {
		return ExecutionResult{Unknown: true}, nil
	}
	allowed, e := p.Repository.ObservePermit(ctx, in.Scope, in.OperationID)
	if e != nil {
		return ExecutionResult{}, e
	}
	if !allowed {
		return ExecutionResult{Wait: true}, nil
	}
	q, e := p.Observer.Observe(ctx, sds.Binding{ID: o.Plan.Binding.ID, Revision: o.Plan.Binding.Revision, MerchantID: o.Plan.Binding.MerchantID}, *o.Intent)
	if e != nil || p.Repository.Finish(ctx, o, q, finishExecution(q), p.Inputs.Guard) != nil {
		return ExecutionResult{Wait: true}, nil
	}
	return ExecutionResult{Done: true}, nil
}
