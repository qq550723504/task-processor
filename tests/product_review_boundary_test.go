package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProductReviewCurrentOwnerBoundaries(t *testing.T) {
	assertNoBannedImportPrefixes(t, filepath.Join("..", "internal", "product", "review"), []string{"gorm.io", "task-processor/internal/integration", "task-processor/internal/app", "task-processor/internal/platform", "task-processor/internal/listingkit", "task-processor/internal/compatibility", "task-processor/internal/tenantbridge", "task-processor/internal/product/image", "task-processor/internal/product/asset"}, nil)
	adapter := filepath.Join("..", "internal", "integration", "persistence", "product", "review")
	assertNoBannedImportPrefixes(t, adapter, []string{"task-processor/internal/product/sourcing", "task-processor/internal/product/enrichment", "task-processor/internal/product/image", "task-processor/internal/product/asset", "task-processor/internal/marketplace", "task-processor/internal/listing"}, nil)
	raw, e := os.ReadFile(filepath.Join(adapter, "repository.go"))
	require.NoError(t, e)
	for _, table := range []string{"product_snapshot_versions", "product_snapshot_heads"} {
		require.NotContains(t, string(raw), table, "review must use Catalog writer, not Catalog SQL")
	}
	root := filepath.Join("..", "internal", "app", "httpapi")
	files, e := os.ReadDir(root)
	require.NoError(t, e)
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") || strings.HasSuffix(file.Name(), "_test.go") || strings.HasPrefix(file.Name(), "product_review_") {
			continue
		}
		raw, e = os.ReadFile(filepath.Join(root, file.Name()))
		require.NoError(t, e)
		require.NotContains(t, string(raw), "NewProductReviewApplication(", "no default production registration")
	}
}
