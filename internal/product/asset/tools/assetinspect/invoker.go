package assetinspect

import (
	"context"
	"encoding/json"

	"task-processor/internal/commercetool"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

// FreshPrincipalResolver resolves the current actor, explicitly selected
// organization and current grants on every call; cache hits are forbidden.
// It is intentionally distinct from the ordinary cached PrincipalResolver.
type FreshPrincipalResolver interface {
	ResolveFreshPrincipal(context.Context) (commercetool.Principal, error)
}

type freshPrincipalDelegate struct{ fresh FreshPrincipalResolver }

func (d freshPrincipalDelegate) ResolvePrincipal(ctx context.Context) (commercetool.Principal, error) {
	return d.fresh.ResolveFreshPrincipal(ctx)
}

type Invoker struct{ tools *commercetool.BoundToolSet }

// NewInvoker assembles only this tool. Consumers must inject the existing
// persistence adapters bounded by MaxCatalogSnapshotBytes/MaxInventoryBytes.
// It does not mount a route or start an Agent runtime.
func NewInvoker(snapshots catalog.VersionedSnapshotReader, assets asset.ApprovedInventoryReader, fresh FreshPrincipalResolver, agent commercetool.AgentDefinition, deps commercetool.InvocationDependencies) (*Invoker, error) {
	if nilPort(fresh) {
		return nil, commercetool.NewError(commercetool.ErrorIdentityIntegrity, "fresh principal resolver is required", nil)
	}
	executor, err := NewExecutor(snapshots, assets)
	if err != nil {
		return nil, err
	}
	definition := Definition()
	allowed := 0
	for _, ref := range agent.AllowedTools {
		if ref == definition.Ref {
			allowed++
		}
	}
	if allowed != 1 {
		return nil, commercetool.NewError(commercetool.ErrorToolNotAllowed, "asset inspection must be allowed exactly once", nil)
	}
	registry, err := commercetool.NewRegistry(commercetool.Tool{Definition: definition, Executor: executor})
	if err != nil {
		return nil, err
	}
	deps.PrincipalResolver = freshPrincipalDelegate{fresh: fresh}
	agent.AllowedTools = []commercetool.ToolRef{definition.Ref}
	bound, err := registry.Bind(agent, deps)
	if err != nil {
		return nil, err
	}
	return &Invoker{tools: bound}, nil
}

func (i *Invoker) Invoke(ctx context.Context, metadata commercetool.CallMetadata, input Input) (commercetool.Result, error) {
	if i == nil || i.tools == nil {
		return commercetool.Result{}, commercetool.NewError(commercetool.ErrorInternal, "asset inspection is unavailable", nil)
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return commercetool.Result{}, commercetool.NewError(commercetool.ErrorInvalidInput, "asset inspection input is invalid", err)
	}
	return i.tools.Invoke(ctx, commercetool.Call{Tool: Definition().Ref, Metadata: metadata, Arguments: raw})
}
