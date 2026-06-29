package consumer

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformModuleRegistrarDoesNotStoreSharedResources(t *testing.T) {
	t.Parallel()

	files := []string{"dependencies.go", "platform_module_registrar.go"}
	for _, name := range files {
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		source := string(content)

		for _, marker := range []string{
			"resources *SharedResources) platformModuleRegistrar",
			"resources      *SharedResources",
			"resources:      resources",
			"func (r platformModuleRegistrar) register(ctx context.Context, module PlatformModule) error",
		} {
			if strings.Contains(source, marker) {
				t.Fatalf("%s mentions %q; pass SharedResources into register instead of storing it on platformModuleRegistrar", name, marker)
			}
		}
	}
}

func TestPlatformRuntimeContextBuilderDoesNotAcceptSharedResourcesPointer(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("shared_resources.go")
	if err != nil {
		t.Fatalf("read shared_resources.go: %v", err)
	}
	source := string(content)

	for _, marker := range []string{
		"Resources        *SharedResources",
		"sharedResourcesValue",
	} {
		if strings.Contains(source, marker) {
			t.Fatalf("shared_resources.go mentions %q; expand SharedResources before building PlatformRuntimeContext", marker)
		}
	}
}

func TestPlatformProcessorRegistryDoesNotAcceptSharedResourcesPointer(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("platform_processor_registry.go")
	if err != nil {
		t.Fatalf("read platform_processor_registry.go: %v", err)
	}
	source := string(content)

	for _, marker := range []string{
		"resources *SharedResources",
		"resourcesValue := *resources",
		"resources == nil",
	} {
		if strings.Contains(source, marker) {
			t.Fatalf("platform_processor_registry.go mentions %q; expand SharedResources before calling registry registration methods", marker)
		}
	}
}

func TestPlatformRuntimeContextDoesNotExposeConcreteServiceManager(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("shared_resources.go")
	if err != nil {
		t.Fatalf("read shared_resources.go: %v", err)
	}
	source := string(content)

	for _, marker := range []string{
		"ServiceManager                     *ServiceManager",
		"ServiceManager   *ServiceManager",
	} {
		if strings.Contains(source, marker) {
			t.Fatalf("shared_resources.go mentions %q; expose a narrow runtime services interface instead of concrete ServiceManager", marker)
		}
	}
}
