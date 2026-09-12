package readinessinspect

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/commercetool"
)

func TestRegistryRejectsAuthorityLegacyAndUnboundedInputs(t *testing.T) {
	p, a, f := &exactProducts{product: productFixture()}, &exactAssets{inventory: inventoryFixture()}, &freshPrincipal{org: "org-a"}
	i := testInvoker(t, p, a, f)
	for _, raw := range []string{
		`{"product_key":"product-1","catalog_version":"7","target_platform":"shein","task_id":"old"}`,
		`{"product_key":"product-1","catalog_version":"7","target_platform":"shein","tenant_id":"other"}`,
		`{"product_key":"product-1","catalog_version":"7"}`,
		`{"product_key":"product-1","catalog_version":7,"target_platform":"shein"}`,
		`{"product_key":"product-1","catalog_version":"7","target_platform":"shein","action":"publish"}`,
		strings.Repeat("x", commercetool.MaxInvocationArgumentsBytes+1),
	} {
		result, err := i.tools.Invoke(context.Background(), commercetool.Call{Tool: i.ref, Metadata: commercetool.CallMetadata{CallID: "call", AgentID: "test.agent", AgentVersion: "v1.0.0", AgentRunID: "run", BusinessTaskID: "correlation"}, Arguments: json.RawMessage(raw)})
		require.Equal(t, commercetool.ErrorInvalidInput, commercetool.CodeOf(err))
		require.Empty(t, result.Output)
	}
	require.Zero(t, p.calls)
	require.Zero(t, a.calls)
}

func TestConstructorRejectsNilAndUnapprovedTool(t *testing.T) {
	var product *exactProducts
	var assets *exactAssets
	var fresh *freshPrincipal
	_, err := NewExecutor(product, &exactAssets{})
	require.Error(t, err)
	_, err = NewExecutor(&exactProducts{}, assets)
	require.Error(t, err)
	_, err = NewInvoker(&exactProducts{}, &exactAssets{}, fresh, commercetool.AgentDefinition{}, commercetool.InvocationDependencies{})
	require.Error(t, err)
	_, err = NewInvoker(&exactProducts{}, &exactAssets{}, &freshPrincipal{}, commercetool.AgentDefinition{}, commercetool.InvocationDependencies{})
	require.Equal(t, commercetool.ErrorToolNotAllowed, commercetool.CodeOf(err))
}
