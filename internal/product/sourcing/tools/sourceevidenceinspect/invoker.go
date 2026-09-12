package sourceevidenceinspect

import (
	"context"
	"encoding/json"
	"task-processor/internal/commercetool"
	"task-processor/internal/product/catalog"
)

// Invoker is the tool-specific assembly point. Callers supply the admitted
// Catalog bounded reader, SRC-1 freshly authorized Read port, and current
// Registry identity/permission/audit/trace dependencies. No default route,
// shared registration, identity adapter or Agent runtime is installed here.
type Invoker struct {
	tools *commercetool.BoundToolSet
	ref   commercetool.ToolRef
}

func NewInvoker(catalogReader catalog.VersionedSnapshotReader, sources SourceReader, agent commercetool.AgentDefinition, dependencies commercetool.InvocationDependencies) (*Invoker, error) {
	executor, err := NewExecutor(catalogReader, sources)
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
		return nil, commercetool.NewError(commercetool.ErrorToolNotAllowed, "source evidence tool must be allowed exactly once", nil)
	}
	registry, err := commercetool.NewRegistry(commercetool.Tool{Definition: definition, Executor: executor})
	if err != nil {
		return nil, err
	}
	toolAgent := agent
	toolAgent.AllowedTools = []commercetool.ToolRef{definition.Ref}
	bound, err := registry.Bind(toolAgent, dependencies)
	if err != nil {
		return nil, err
	}
	return &Invoker{tools: bound, ref: definition.Ref}, nil
}

func (i *Invoker) Invoke(ctx context.Context, metadata commercetool.CallMetadata, input Input) (commercetool.Result, error) {
	if i == nil || i.tools == nil {
		return commercetool.Result{}, commercetool.NewError(commercetool.ErrorInternal, "source evidence tool is unavailable", nil)
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return commercetool.Result{}, commercetool.NewError(commercetool.ErrorInvalidInput, "source evidence input is invalid", err)
	}
	return i.tools.Invoke(ctx, commercetool.Call{Tool: i.ref, Metadata: metadata, Arguments: raw})
}
