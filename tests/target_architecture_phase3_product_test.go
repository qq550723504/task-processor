package tests

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPhase3ProductRootContainsNoGoPackage(t *testing.T) {
	entries, err := filepath.Glob(filepath.Join("..", "internal", "product", "*.go"))
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestPhase3ProductTargetDependencies(t *testing.T) {
	for _, name := range []string{"catalog", "sourcing", "enrichment", "asset", "image"} {
		root := filepath.Join("..", "internal", "product", name)
		if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			t.Fatal(err)
		}

		assertNoBannedImportPrefixes(t, root, []string{
			"task-processor/internal/app",
			"task-processor/internal/platform",
			"task-processor/internal/integration",
			"gorm.io/gorm",
			"go.temporal.io",
			"github.com/redis",
			"github.com/rabbitmq",
			"github.com/aws/aws-sdk-go-v2",
			"github.com/sashabaranov/go-openai",
		}, nil)
	}

	assertNoBannedImportPrefixes(t, filepath.Join("..", "internal", "product", "sourcing"), []string{
		"task-processor/internal/asset",
		"task-processor/internal/crawler",
		"task-processor/internal/model",
		"task-processor/internal/product/asset",
		"task-processor/internal/product/enrichment",
		"task-processor/internal/product/image",
		"task-processor/internal/productenrich",
	}, nil)
}

func TestPhase3LegacyProductRootsDoNotGrow(t *testing.T) {
	for root, max := range map[string]int{
		"catalog":       5,
		"asset":         26,
		"imageasset":    1,
		"productenrich": 62,
		"productimage":  88,
	} {
		if got := productionGoFileCount(t, filepath.Join("..", "internal", root)); got > max {
			t.Errorf("internal/%s production files = %d, baseline max = %d", root, got, max)
		}
	}
}

func TestPhase3PipelineDoesNotGrow(t *testing.T) {
	const baselineMax = 10
	if got := productionGoFileCount(t, filepath.Join("..", "internal", "pipeline")); got > baselineMax {
		t.Errorf("internal/pipeline production files = %d, baseline max = %d", got, baselineMax)
	}
}
