package bootstrap

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformProcessorAssemblyDoesNotDependOnAppServices(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("platform_processors.go")
	if err != nil {
		t.Fatalf("read platform_processors.go: %v", err)
	}
	source := string(content)

	for _, marker := range []string{
		"func buildTemuProcessor(svc *appServices",
		"func buildSheinProcessor(svc *appServices",
		"func buildTemuProcessorDependencies(svc *appServices)",
		"func buildSheinProcessorDependencies(svc *appServices)",
		"svc.rawJSONDataClient",
		"svc.processorRuntime",
		"svc.amazonCrawler",
		"svc.rabbitmqClient",
	} {
		if strings.Contains(source, marker) {
			t.Fatalf("platform_processors.go mentions %q; platform processor assembly should use a narrow resource view instead of appServices", marker)
		}
	}

	for _, marker := range []string{
		"type platformProcessorResources struct",
		"func buildTemuProcessor(cfg *config.Config, resources platformProcessorResources",
		"func buildSheinProcessor(cfg *config.Config, resources platformProcessorResources",
		"func buildTemuProcessorDependencies(cfg *config.Config, resources platformProcessorResources)",
		"func buildSheinProcessorDependencies(cfg *config.Config, resources platformProcessorResources)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("platform_processors.go should contain %q", marker)
		}
	}
}
