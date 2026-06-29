package bootstrap

import (
	"os"
	"strings"
	"testing"
)

func TestAppServiceAssemblyDoesNotPassBootstrapSharedResources(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatalf("read app.go: %v", err)
	}
	source := string(content)

	for _, marker := range []string{
		"func buildAppServices(cfg *config.Config, logger *logrus.Logger, resources *bootstrapresources.SharedResources)",
		"func buildProcessorService(logger *logrus.Logger, resources *bootstrapresources.SharedResources)",
		"func buildSchedulerService(logger *logrus.Logger, cfg *config.Config, resources *bootstrapresources.SharedResources)",
	} {
		if strings.Contains(source, marker) {
			t.Fatalf("app.go mentions %q; app service assembly should pass a narrow local resource view instead of bootstrap SharedResources", marker)
		}
	}

	for _, marker := range []string{
		"type appServiceResources struct",
		"func newAppServiceResources(resources *bootstrapresources.SharedResources) appServiceResources",
		"func buildAppServices(cfg *config.Config, logger *logrus.Logger, resources appServiceResources)",
		"func buildProcessorService(logger *logrus.Logger, resources appServiceResources)",
		"func buildSchedulerService(logger *logrus.Logger, cfg *config.Config, resources appServiceResources)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("app.go should contain %q to keep bootstrap resource expansion at the boundary", marker)
		}
	}
}
