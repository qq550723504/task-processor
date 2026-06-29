package bootstrap

import (
	"os"
	"strings"
	"testing"
)

func TestLifecycleRegistrationDoesNotDependOnAppServices(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("component_adapters.go")
	if err != nil {
		t.Fatalf("read component_adapters.go: %v", err)
	}
	source := string(content)

	for _, marker := range []string{
		"svc *appServices",
		"func newLifecycleRegistrationResources(svc *appServices) lifecycleRegistrationResources",
		"func registerCoreComponents(\n\tlm lifecycle.LifecycleManager,\n\tsvc *appServices,",
		"func registerTaskFetcherComponent(\n\tlm lifecycle.LifecycleManager,\n\tsvc *appServices,",
		"func registerSchedulerComponent(\n\tlm lifecycle.LifecycleManager,\n\tsvc *appServices,",
	} {
		if strings.Contains(source, marker) {
			t.Fatalf("component_adapters.go mentions %q; lifecycle registration should use a narrow registration resource view instead of appServices", marker)
		}
	}
	if strings.Count(source, "svc.processorResources") != 1 ||
		strings.Count(source, "svc.processorService") != 1 ||
		strings.Count(source, "svc.schedulerService") != 1 {
		t.Fatalf("component_adapters.go should only read appServices fields inside newLifecycleRegistrationResources")
	}

	for _, marker := range []string{
		"type lifecycleRegistrationResources struct",
		"type registeredProcessorComponents struct",
		"func newLifecycleRegistrationResources(svc appServices) lifecycleRegistrationResources",
		"func registerComponents(\n\tlm lifecycle.LifecycleManager,\n\tresources lifecycleRegistrationResources,",
		"func registerCoreComponents(\n\tlm lifecycle.LifecycleManager,\n\tresources lifecycleRegistrationResources,",
		"func registerTaskFetcherComponent(\n\tlm lifecycle.LifecycleManager,\n\tresources lifecycleRegistrationResources,",
		"func registerSchedulerComponent(\n\tlm lifecycle.LifecycleManager,\n\tresources lifecycleRegistrationResources,",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("component_adapters.go should contain %q", marker)
		}
	}
}
