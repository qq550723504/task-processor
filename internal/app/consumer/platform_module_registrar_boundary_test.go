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

func TestPlatformRuntimeContextDoesNotExposeRabbitMQClientField(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("shared_resources.go")
	if err != nil {
		t.Fatalf("read shared_resources.go: %v", err)
	}
	source := string(content)

	for _, marker := range []string{
		"RabbitMQClient                     *rabbitmq.Client",
		"RabbitMQClient:",
	} {
		if strings.Contains(source, marker) {
			t.Fatalf("shared_resources.go mentions %q; derive messaging client from runtime services instead of storing it on PlatformRuntimeContext", marker)
		}
	}
}

func TestPlatformRuntimeContextDoesNotExposeRuntimeServicesField(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("shared_resources.go")
	if err != nil {
		t.Fatalf("read shared_resources.go: %v", err)
	}
	source := string(content)

	for _, marker := range []string{
		"RuntimeServices                    PlatformRuntimeServices",
		"RuntimeServices  PlatformRuntimeServices",
		"RuntimeServices:",
	} {
		if strings.Contains(source, marker) {
			t.Fatalf("shared_resources.go mentions %q; expose narrow PlatformRuntimeContext methods instead of a broad runtime services field", marker)
		}
	}
}

func TestPlatformRuntimeContextDoesNotExposeSchedulerAssemblyFields(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("shared_resources.go")
	if err != nil {
		t.Fatalf("read shared_resources.go: %v", err)
	}
	source := string(content)
	start := strings.Index(source, "type PlatformRuntimeContext struct {")
	if start < 0 {
		t.Fatal("shared_resources.go should define PlatformRuntimeContext")
	}
	end := strings.Index(source[start:], "\n}")
	if end < 0 {
		t.Fatal("PlatformRuntimeContext struct should have a closing brace")
	}
	contextSource := source[start : start+end]

	for _, marker := range []string{
		"SchedulerRuntime                   runner.SchedulerRuntimeProvider",
		"SchedulerFactoryRuntime            SchedulerFactoryRuntime",
		"CrawlSource                        ports.CrawlSource",
		"SchedulerBuilder                   SchedulerDependenciesBuilder",
	} {
		if strings.Contains(contextSource, marker) {
			t.Fatalf("shared_resources.go mentions %q; expose scheduler assembly through PlatformRuntimeContext methods instead of fields", marker)
		}
	}
}

func TestPlatformRuntimeContextDoesNotExposeRuntimeResourceFields(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("shared_resources.go")
	if err != nil {
		t.Fatalf("read shared_resources.go: %v", err)
	}
	source := string(content)
	start := strings.Index(source, "type PlatformRuntimeContext struct {")
	if start < 0 {
		t.Fatal("shared_resources.go should define PlatformRuntimeContext")
	}
	end := strings.Index(source[start:], "\n}")
	if end < 0 {
		t.Fatal("PlatformRuntimeContext struct should have a closing brace")
	}
	contextSource := source[start : start+end]

	for _, marker := range []string{
		"ListingRuntimeImportTaskRepository ListingRuntimeImportTaskRepository",
		"StoreAPI                           listingadmin.StoreAPI",
		"ProcessorRuntime                   ProcessorRuntime",
		"ProductFetcher                     appfetcher.ProductFetcher",
	} {
		if strings.Contains(contextSource, marker) {
			t.Fatalf("shared_resources.go mentions %q; expose runtime resources through PlatformRuntimeContext methods instead of fields", marker)
		}
	}
}

func TestPlatformRuntimeContextDoesNotExposeContextFields(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("shared_resources.go")
	if err != nil {
		t.Fatalf("read shared_resources.go: %v", err)
	}
	source := string(content)
	start := strings.Index(source, "type PlatformRuntimeContext struct {")
	if start < 0 {
		t.Fatal("shared_resources.go should define PlatformRuntimeContext")
	}
	end := strings.Index(source[start:], "\n}")
	if end < 0 {
		t.Fatal("PlatformRuntimeContext struct should have a closing brace")
	}
	contextSource := source[start : start+end]

	for _, marker := range []string{
		"Config                             *config.Config",
		"Logger                             *logrus.Logger",
	} {
		if strings.Contains(contextSource, marker) {
			t.Fatalf("shared_resources.go mentions %q; expose PlatformRuntimeContext context through methods instead of fields", marker)
		}
	}
}

func TestConsumerSharedResourcesDoesNotExposeSchedulerAssemblyFields(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("shared_resources.go")
	if err != nil {
		t.Fatalf("read shared_resources.go: %v", err)
	}
	source := string(content)
	start := strings.Index(source, "type SharedResources struct {")
	if start < 0 {
		t.Fatal("shared_resources.go should define SharedResources")
	}
	end := strings.Index(source[start:], "\n}")
	if end < 0 {
		t.Fatal("SharedResources struct should have a closing brace")
	}
	resourcesSource := source[start : start+end]

	for _, marker := range []string{
		"SchedulerRuntime                   runner.SchedulerRuntimeProvider",
		"SchedulerFactoryRuntime            SchedulerFactoryRuntime",
		"CrawlSource                        ports.CrawlSource",
	} {
		if strings.Contains(resourcesSource, marker) {
			t.Fatalf("shared_resources.go mentions %q; group scheduler assembly dependencies before passing consumer shared resources", marker)
		}
	}
}

func TestConsumerSharedResourcesDoesNotExposeRuntimeResourceFields(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("shared_resources.go")
	if err != nil {
		t.Fatalf("read shared_resources.go: %v", err)
	}
	source := string(content)
	start := strings.Index(source, "type SharedResources struct {")
	if start < 0 {
		t.Fatal("shared_resources.go should define SharedResources")
	}
	end := strings.Index(source[start:], "\n}")
	if end < 0 {
		t.Fatal("SharedResources struct should have a closing brace")
	}
	resourcesSource := source[start : start+end]

	for _, marker := range []string{
		"ListingRuntimeImportTaskRepository ListingRuntimeImportTaskRepository",
		"StoreAPI                           listingadmin.StoreAPI",
		"ProcessorRuntime                   ProcessorRuntime",
		"ProductFetcher                     appfetcher.ProductFetcher",
		"Scheduler                          SchedulerResources",
	} {
		if strings.Contains(resourcesSource, marker) {
			t.Fatalf("shared_resources.go mentions %q; build consumer shared resources through constructor methods instead of exported fields", marker)
		}
	}
}
