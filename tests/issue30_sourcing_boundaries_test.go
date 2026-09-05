package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIssue30PreparedSlicesHaveNoLegacyOwnershipDependencies(t *testing.T) {
	for _, root := range []string{"internal/product/sourcing", "internal/app/productsourcing", "internal/listing/readiness"} {
		t.Run(root, func(t *testing.T) {
			assertNoBannedImportPrefixes(t, filepath.Join("..", filepath.FromSlash(root)), []string{
				"task-processor/internal/compatibility", "task-processor/internal/listingkit",
				"task-processor/internal/tenantbridge", "task-processor/internal/sourceaccount",
			}, nil)
		})
	}
}

// Temporary prerequisite gate, not permission to keep the old route forever.
// Replace this with the old-route/import absence guard only after ownership
// cutover review and controlled application acceptance.
func TestIssue30PreparedHTTPContractIsNotWiredBeforePrerequisite(t *testing.T) {
	assertNoBannedImportPrefixes(t, filepath.Join("..", "internal", "app", "httpapi"), []string{"task-processor/internal/app/productsourcing"}, nil)
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
