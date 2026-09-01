package tests

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

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
