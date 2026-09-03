package commercetool

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFakeAgentUsesRegistryBindInvokeContract(t *testing.T) {
	calls := 0
	tool := Tool{
		Definition: validDefinition(),
		Executor: ExecutorFunc(func(_ context.Context, _ ExecutionEnvelope, input json.RawMessage) (ExecutionResult, error) {
			calls++
			var request struct {
				TaskID string `json:"task_id"`
			}
			require.NoError(t, json.Unmarshal(input, &request))
			output, err := json.Marshal(map[string]string{"task_id": request.TaskID})
			return ExecutionResult{Output: output}, err
		}),
	}

	registry, err := NewRegistry(tool)
	require.NoError(t, err)
	bound, err := registry.Bind(AgentDefinition{
		ID:           "fake.product-agent",
		Version:      "v1.0.0",
		AllowedTools: []ToolRef{tool.Definition.Ref},
	}, validInvocationDependencies())
	require.NoError(t, err)

	result, err := bound.Invoke(context.Background(), validCall())
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.JSONEq(t, `{"task_id":"task-1"}`, string(result.Output))
}

func TestFakeAgentCannotSubstituteAnotherToolVersion(t *testing.T) {
	definitionV1 := validDefinition()
	definitionV2 := validDefinition()
	definitionV2.Ref.Version = "v1.0.1"
	executor := ExecutorFunc(func(_ context.Context, _ ExecutionEnvelope, input json.RawMessage) (ExecutionResult, error) {
		return ExecutionResult{Output: cloneRaw(input)}, nil
	})
	registry, err := NewRegistry(
		Tool{Definition: definitionV1, Executor: executor},
		Tool{Definition: definitionV2, Executor: executor},
	)
	require.NoError(t, err)
	bound, err := registry.Bind(AgentDefinition{
		ID:           "fake.product-agent",
		Version:      "v1.0.0",
		AllowedTools: []ToolRef{definitionV1.Ref},
	}, validInvocationDependencies())
	require.NoError(t, err)

	call := validCall()
	call.Tool = definitionV2.Ref
	_, err = bound.Invoke(context.Background(), call)
	require.Equal(t, ErrorToolNotAllowed, CodeOf(err))
}
