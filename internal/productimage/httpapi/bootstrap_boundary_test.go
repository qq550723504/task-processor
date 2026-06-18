package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestBootstrapKeepsModelProviderAssemblyInDedicatedFile(t *testing.T) {
	bootstrapSource := readProductImageHTTPAPIBoundaryFile(t, "bootstrap.go")
	for _, marker := range []string{
		`"task-processor/internal/infra/clients/nanobanana"`,
		"func buildModelProvider(",
		"nanobanana.NewClient(",
	} {
		if strings.Contains(bootstrapSource, marker) {
			t.Fatalf("bootstrap.go should delegate ProductImage model provider assembly to model_provider_builder.go; found %s", marker)
		}
	}

	builderSource := readProductImageHTTPAPIBoundaryFile(t, "model_provider_builder.go")
	for _, marker := range []string{
		`"task-processor/internal/infra/clients/nanobanana"`,
		"func buildModelProvider(",
		"nanobanana.NewClient(",
	} {
		if !strings.Contains(builderSource, marker) {
			t.Fatalf("model_provider_builder.go missing %s", marker)
		}
	}
}

func TestBootstrapKeepsAssetPublisherAssemblyInDedicatedFile(t *testing.T) {
	bootstrapSource := readProductImageHTTPAPIBoundaryFile(t, "bootstrap.go")
	for _, marker := range []string{
		`"task-processor/internal/infra/storage"`,
		"func buildAssetPublisher(",
		"func newPublisherS3Client(",
		"func buildS3AssetPublisher(",
		"storageinfra.NewS3Client(",
	} {
		if strings.Contains(bootstrapSource, marker) {
			t.Fatalf("bootstrap.go should delegate ProductImage asset publisher assembly to asset_publisher_builder.go; found %s", marker)
		}
	}

	builderSource := readProductImageHTTPAPIBoundaryFile(t, "asset_publisher_builder.go")
	for _, marker := range []string{
		`"task-processor/internal/infra/storage"`,
		"func buildAssetPublisher(",
		"func newPublisherS3Client(",
		"func buildS3AssetPublisher(",
		"storageinfra.NewS3Client(",
	} {
		if !strings.Contains(builderSource, marker) {
			t.Fatalf("asset_publisher_builder.go missing %s", marker)
		}
	}
}

func TestBootstrapKeepsTaskRepositoryAssemblyInDedicatedFile(t *testing.T) {
	bootstrapSource := readProductImageHTTPAPIBoundaryFile(t, "bootstrap.go")
	for _, marker := range []string{
		`"task-processor/internal/infra/database"`,
		`"task-processor/internal/productimage/store"`,
		"func buildTaskRepository(",
		"func newDBTaskRepository(",
		"database.NewSharedDatabaseFromConfig(",
		"db.AutoMigrate(&productimage.Task{})",
	} {
		if strings.Contains(bootstrapSource, marker) {
			t.Fatalf("bootstrap.go should delegate ProductImage task repository assembly to task_repository_builder.go; found %s", marker)
		}
	}

	builderSource := readProductImageHTTPAPIBoundaryFile(t, "task_repository_builder.go")
	for _, marker := range []string{
		`"task-processor/internal/infra/database"`,
		`"task-processor/internal/productimage/store"`,
		"func buildTaskRepository(",
		"func newDBTaskRepository(",
		"database.NewSharedDatabaseFromConfig(",
		"db.AutoMigrate(&productimage.Task{})",
	} {
		if !strings.Contains(builderSource, marker) {
			t.Fatalf("task_repository_builder.go missing %s", marker)
		}
	}
}

func TestBootstrapKeepsImagePipelineComponentAssemblyInDedicatedFile(t *testing.T) {
	bootstrapSource := readProductImageHTTPAPIBoundaryFile(t, "bootstrap.go")
	for _, marker := range []string{
		"func buildSubjectExtractor(",
		"func buildWhiteBackgroundRenderer(",
		"func buildSceneRenderer(",
		"func resolveImagePipelineComponents(",
		"productimage.HTTPSegmentationClientConfig{",
		"productimage.HTTPWhiteBackgroundClientConfig{",
		"productimage.NewModelSubjectExtractor(",
	} {
		if strings.Contains(bootstrapSource, marker) {
			t.Fatalf("bootstrap.go should delegate ProductImage image pipeline component assembly to image_pipeline_component_builder.go; found %s", marker)
		}
	}

	builderSource := readProductImageHTTPAPIBoundaryFile(t, "image_pipeline_component_builder.go")
	for _, marker := range []string{
		"func buildSubjectExtractor(",
		"func buildWhiteBackgroundRenderer(",
		"func buildSceneRenderer(",
		"func resolveImagePipelineComponents(",
		"productimage.HTTPSegmentationClientConfig{",
		"productimage.HTTPWhiteBackgroundClientConfig{",
		"productimage.NewModelSubjectExtractor(",
	} {
		if !strings.Contains(builderSource, marker) {
			t.Fatalf("image_pipeline_component_builder.go missing %s", marker)
		}
	}
}

func readProductImageHTTPAPIBoundaryFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
