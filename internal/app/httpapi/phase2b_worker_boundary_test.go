package httpapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPAPITypesDoesNotOwnNamedWorkerPoolMap(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("types.go")
	require.NoError(t, err)

	content := string(src)
	require.NotContains(t, content, "func (c httpFeatureComposition) namedWorkerPools(")
	require.NotContains(t, content, "func (c httpFeatureComposition) workerPools(")
}
