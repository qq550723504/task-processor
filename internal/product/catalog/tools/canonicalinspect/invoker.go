package canonicalinspect

import (
	"context"
	"encoding/json"
	"fmt"

	"task-processor/internal/commercetool"
	"task-processor/internal/product/catalog"
)

// Invoker is the callable application boundary for the exact Catalog reader.
// Registry binding is fixed at construction; principal resolution still runs
// inside every invocation through the supplied dependencies.
type Invoker struct {
	tools *commercetool.BoundToolSet
	ref   commercetool.ToolRef
}

// NewInvoker is the stable consumer assembly point. Runtime consumers supply a
// Catalog bounded reader, their approved Agent allowlist, and the existing
// Workbench-principal/Casbin/audit/trace dependencies; this function does not
// mount a default route or create an Agent runtime.
func NewInvoker(reader catalog.VersionedSnapshotReader, agent commercetool.AgentDefinition, dependencies commercetool.InvocationDependencies) (*Invoker, error) {
	executor, err := NewExecutor(reader)
	if err != nil {
		return nil, err
	}
	definition := Definition()
	allowedCount := 0
	for _, ref := range agent.AllowedTools {
		if ref == definition.Ref {
			allowedCount++
		}
	}
	if allowedCount != 1 {
		return nil, commercetool.NewError(commercetool.ErrorToolNotAllowed, "canonical inspection tool is not allowed exactly once", nil)
	}
	registry, err := commercetool.NewRegistry(commercetool.Tool{Definition: definition, Executor: executor})
	if err != nil {
		return nil, fmt.Errorf("register canonical inspection tool: %w", err)
	}
	toolAgent := agent
	toolAgent.AllowedTools = []commercetool.ToolRef{definition.Ref}
	bound, err := registry.Bind(toolAgent, dependencies)
	if err != nil {
		return nil, fmt.Errorf("bind canonical inspection tool: %w", err)
	}
	return &Invoker{tools: bound, ref: definition.Ref}, nil
}

// Invoke always traverses BoundToolSet.Invoke. Metadata is correlation and
// audit context only; product identity comes exclusively from input plus the
// request-scoped principal resolved by the supplied dependencies.
func (i *Invoker) Invoke(ctx context.Context, metadata commercetool.CallMetadata, input Input) (commercetool.Result, error) {
	if i == nil || i.tools == nil {
		return commercetool.Result{}, commercetool.NewError(commercetool.ErrorInternal, "canonical inspection is unavailable", nil)
	}
	arguments, err := json.Marshal(input)
	if err != nil {
		return commercetool.Result{}, commercetool.NewError(commercetool.ErrorInvalidInput, "canonical inspection input is invalid", err)
	}
	return i.tools.Invoke(ctx, commercetool.Call{Tool: i.ref, Metadata: metadata, Arguments: arguments})
}
