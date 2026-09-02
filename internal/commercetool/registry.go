package commercetool

import (
	"encoding/json"
	"fmt"
	"sort"
)

type registeredTool struct {
	definition Definition
	executor   Executor
	schemas    compiledSchemas
}

type Registry struct {
	tools map[ToolRef]registeredTool
}

type BoundToolSet struct {
	agent AgentRef
	deps  InvocationDependencies
	tools map[ToolRef]registeredTool
}

func NewRegistry(tools ...Tool) (*Registry, error) {
	registered := make(map[ToolRef]registeredTool, len(tools))
	for _, tool := range tools {
		if err := tool.Definition.Validate(); err != nil {
			return nil, err
		}
		if isNilInterface(tool.Executor) {
			return nil, fmt.Errorf("executor is nil")
		}

		schemas, err := compileSchemas(tool.Definition)
		if err != nil {
			return nil, err
		}
		if _, exists := registered[tool.Definition.Ref]; exists {
			return nil, fmt.Errorf("duplicate tool")
		}

		registered[tool.Definition.Ref] = registeredTool{
			definition: cloneDefinition(tool.Definition),
			executor:   tool.Executor,
			schemas:    schemas,
		}
	}

	return &Registry{tools: registered}, nil
}

func (registry *Registry) Bind(agent AgentDefinition, dependencies InvocationDependencies) (*BoundToolSet, error) {
	agentRef := AgentRef{ID: agent.ID, Version: agent.Version}
	if err := agentRef.Validate(); err != nil {
		return nil, NewError(ErrorIdentityIntegrity, "agent identity is invalid", err)
	}
	for _, ref := range agent.AllowedTools {
		if err := ref.Validate(); err != nil {
			return nil, toolNotAllowed(err)
		}
	}
	if err := dependencies.Validate(); err != nil {
		return nil, err
	}

	allowed := make(map[ToolRef]registeredTool, len(agent.AllowedTools))
	for _, ref := range agent.AllowedTools {
		if _, duplicate := allowed[ref]; duplicate {
			return nil, toolNotAllowed(fmt.Errorf("duplicate allowlist ref"))
		}

		tool, exists := registry.tools[ref]
		if !exists {
			return nil, toolNotAllowed(fmt.Errorf("tool ref is not registered"))
		}
		if tool.definition.Risk == RiskWrite || tool.definition.Risk == RiskPublish {
			return nil, toolNotAllowed(fmt.Errorf("tool risk is not executable in slice A"))
		}

		tool.definition = cloneDefinition(tool.definition)
		allowed[ref] = tool
	}

	return &BoundToolSet{
		agent: agentRef,
		deps:  dependencies,
		tools: allowed,
	}, nil
}

func (tools *BoundToolSet) Definitions() []Definition {
	definitions := make([]Definition, 0, len(tools.tools))
	for _, tool := range tools.tools {
		definitions = append(definitions, cloneDefinition(tool.definition))
	}

	sort.Slice(definitions, func(i, j int) bool {
		if definitions[i].Ref.ID != definitions[j].Ref.ID {
			return definitions[i].Ref.ID < definitions[j].Ref.ID
		}
		return definitions[i].Ref.Version < definitions[j].Ref.Version
	})

	return definitions
}

func toolNotAllowed(cause error) error {
	return NewError(ErrorToolNotAllowed, "requested tool is not allowed", cause)
}

func cloneDefinition(definition Definition) Definition {
	definition.InputSchema = cloneRaw(definition.InputSchema)
	definition.OutputSchema = cloneRaw(definition.OutputSchema)
	return definition
}

func cloneRaw(raw json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), raw...)
}
