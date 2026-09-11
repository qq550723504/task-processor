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

func NewInvoker(reader catalog.VersionedSnapshotReader, agent commercetool.AgentDefinition, dependencies commercetool.InvocationDependencies) (*Invoker, error) {
	executor, err := NewExecutor(reader)
	if err != nil {
		return nil, err
	}
	definition := Definition()
	registry, err := commercetool.NewRegistry(commercetool.Tool{Definition: definition, Executor: executor})
	if err != nil {
		return nil, fmt.Errorf("register canonical inspection tool: %w", err)
	}
	bound, err := registry.Bind(agent, dependencies)
	if err != nil {
		return nil, fmt.Errorf("bind canonical inspection tool: %w", err)
	}
	return &Invoker{tools: bound, ref: definition.Ref}, nil
}

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
