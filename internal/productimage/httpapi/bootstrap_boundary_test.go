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

func readProductImageHTTPAPIBoundaryFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
