package readinessinspect

import (
	"context"
	"encoding/json"
	"errors"

	"task-processor/internal/commercetool"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

// FreshPrincipalResolver must revalidate the requested organization and current
// grants on every call. The current integration fresh adapter implements this
// port; the cached-read adapter cannot satisfy it. Permission remains read-only.
type FreshPrincipalResolver interface {
	ResolveFreshPrincipal(context.Context) (commercetool.Principal, error)
}

type freshDelegate struct{ resolver FreshPrincipalResolver }

func (d freshDelegate) ResolvePrincipal(ctx context.Context) (commercetool.Principal, error) {
	return d.resolver.ResolveFreshPrincipal(ctx)
}

type Invoker struct {
	tools *commercetool.BoundToolSet
	ref   commercetool.ToolRef
}

// NewInvoker binds only this Tool. It deliberately replaces any caller-supplied
// cached PrincipalResolver with the explicit fresh principal contract.
// Both exact readers must enforce MaxSnapshotBytes in the persistence owner
// before materializing payloads: use Catalog's NewBoundedSnapshotReader and
// Asset's NewBoundedApprovedInventoryReader in the integration composition.
func NewInvoker(products catalog.VersionedSnapshotReader, assets asset.ApprovedInventoryReader, fresh FreshPrincipalResolver,
	agent commercetool.AgentDefinition, deps commercetool.InvocationDependencies) (*Invoker, error) {
	if nilInterface(fresh) {
		return nil, errors.New("fresh principal resolver is required")
	}
	executor, err := NewExecutor(products, assets)
	if err != nil {
		return nil, err
	}
	definition := Definition()
	count := 0
	for _, ref := range agent.AllowedTools {
		if ref == definition.Ref {
			count++
		}
	}
	if count != 1 {
		return nil, commercetool.NewError(commercetool.ErrorToolNotAllowed, "input diagnostics must be allowed exactly once", nil)
	}
	registry, err := commercetool.NewRegistry(commercetool.Tool{Definition: definition, Executor: executor})
	if err != nil {
		return nil, err
	}
	agent.AllowedTools = []commercetool.ToolRef{definition.Ref}
	deps.PrincipalResolver = freshDelegate{fresh}
	bound, err := registry.Bind(agent, deps)
	if err != nil {
		return nil, err
	}
	return &Invoker{tools: bound, ref: definition.Ref}, nil
}

func (i *Invoker) Invoke(ctx context.Context, metadata commercetool.CallMetadata, input Input) (commercetool.Result, error) {
	if i == nil || i.tools == nil {
		return commercetool.Result{}, readError(ErrCorrupt)
	}
	// Registry preflight precedes the executor deadline, so bound the fresh grant
	// lookup as well as the subsequent exact reads and computation.
	ctx, cancel := context.WithTimeout(ctx, Definition().Timeout.Duration)
	defer cancel()
	raw, err := json.Marshal(input)
	if err != nil {
		return commercetool.Result{}, invalidInput()
	}
	return i.tools.Invoke(ctx, commercetool.Call{Tool: i.ref, Metadata: metadata, Arguments: raw})
}
