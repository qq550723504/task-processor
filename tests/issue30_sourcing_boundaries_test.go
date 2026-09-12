package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIssue30PreparedSlicesHaveNoLegacyOwnershipDependencies(t *testing.T) {
	for _, root := range []string{"internal/product/sourcing", "internal/app/productsourcing", "internal/listing/readiness", "internal/integration/acquisition/a1688", "internal/integration/persistence/product/acquisition"} {
		t.Run(root, func(t *testing.T) {
			assertNoBannedImportPrefixes(t, filepath.Join("..", filepath.FromSlash(root)), []string{
				"task-processor/internal/compatibility", "task-processor/internal/listingkit",
				"task-processor/internal/tenantbridge", "task-processor/internal/sourceaccount",
			}, nil)
		})
	}
}

// REV-1 admits one isolated Product Review composition to the in-process
// productsourcing capability. SRC-2B1 separately admits the precise Public
// module and empty-database initializer (5643032970), not their subpackages.
// TestIssue398TrackedCurrentLeafAPIsStayAdmitted limits those files' exact APIs.
func TestIssue30InternalProducerIsOnlyWiredByAdmittedProductReview(t *testing.T) {
	allowed := map[string]struct{}{
		filepath.Join("..", "internal", "app", "httpapi", "product_review_application.go"):      {},
		filepath.Join("..", "internal", "app", "httpapi", "product_acquisition_application.go"): {},
	}
	assertNoBannedImportPrefixes(t, filepath.Join("..", "internal"), []string{"task-processor/internal/app/productsourcing"}, allowed)
	commands := map[string]struct{}{filepath.Join("..", "cmd", "product-acquisition-init", "main.go"): {}}
	assertNoBannedImportPrefixes(t, filepath.Join("..", "cmd"), []string{"task-processor/internal/app/productsourcing"}, commands)
}

func TestIssue30LegacyImportGuardDetectsAliasesAndSubpackages(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "bad.go"), []byte(`package fixture
import (
 bridge "task-processor/internal/tenantbridge"
 legacy "task-processor/internal/compatibility/listingkit/sourcehandoff/a1688"
 task "task-processor/internal/listingkit"
)
`), 0600))
	violations, err := findBannedImportViolations(root, []string{`"task-processor/internal/tenantbridge"`, `"task-processor/internal/compatibility/listingkit/sourcehandoff/a1688"`, `"task-processor/internal/listingkit"`}, nil, true)
	require.NoError(t, err)
	require.Len(t, violations, 3)
	require.True(t, importMatchesPrefix("task-processor/internal/compatibility/listingkit/sourcehandoff/a1688", "task-processor/internal/compatibility"))
	require.False(t, importMatchesPrefix("task-processor/internal/listingkitfake", "task-processor/internal/listingkit"))
}
