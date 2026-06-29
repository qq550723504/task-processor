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
			"func (r platformModuleRegistrar) register(ctx context.Context, module PlatformModule, resources SharedResources) error",
			"func (r platformModuleRegistrar) register(ctx context.Context, module PlatformModule) error",
		} {
			if strings.Contains(source, marker) {
				t.Fatalf("%s mentions %q; pass narrow platform runtime resources into register instead of storing or forwarding SharedResources", name, marker)
			}
		}
	}

	content, err := os.ReadFile("platform_module_registrar.go")
	if err != nil {
		t.Fatalf("read platform_module_registrar.go: %v", err)
	}
	if !strings.Contains(string(content), "func (r platformModuleRegistrar) register(ctx context.Context, module PlatformModule, resources PlatformRuntimeResources) error") {
		t.Fatalf("platform_module_registrar.go should register modules with PlatformRuntimeResources")
	}
}

func TestPlatformRegistrationDoesNotDependOnConcreteServiceManager(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"dependencies.go", "platform_processor_registry.go", "platform_module_registrar.go"} {
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		source := string(content)
		for _, marker := range []string{
			"serviceManager *ServiceManager",
			"serviceManager: serviceManager",
			"serviceManager *ServiceManager) platformModuleRegistrar",
			"RegisterPlatforms(ctx context.Context, serviceManager *ServiceManager",
			"RegisterAllProcessors(ctx context.Context, serviceManager *ServiceManager",
		} {
			if strings.Contains(source, marker) {
				t.Fatalf("%s mentions %q; platform registration should depend on narrow registration services instead of concrete ServiceManager", name, marker)
			}
		}
	}

	content, err := os.ReadFile("shared_resources.go")
	if err != nil {
		t.Fatalf("read shared_resources.go: %v", err)
	}
	if !strings.Contains(string(content), "type PlatformRegistrationServices interface") {
		t.Fatalf("shared_resources.go should define PlatformRegistrationServices for platform registration assembly")
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
		"Resources        SharedResources",
		"sharedResourcesValue",
	} {
		if strings.Contains(source, marker) {
			t.Fatalf("shared_resources.go mentions %q; expand SharedResources before building PlatformRuntimeContext", marker)
		}
	}

	if !strings.Contains(source, "type PlatformRuntimeResources struct") {
		t.Fatalf("shared_resources.go should define PlatformRuntimeResources for PlatformRuntimeContextInput")
	}
	if !strings.Contains(source, "func NewPlatformRuntimeResources(resources SharedResources) PlatformRuntimeResources") {
		t.Fatalf("shared_resources.go should expand SharedResources through NewPlatformRuntimeResources before building PlatformRuntimeContext")
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

func TestSchedulerResourcesDoesNotExposeAssemblyFields(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("shared_resources.go")
	if err != nil {
		t.Fatalf("read shared_resources.go: %v", err)
	}
	source := string(content)
	start := strings.Index(source, "type SchedulerResources struct {")
	if start < 0 {
		t.Fatal("shared_resources.go should define SchedulerResources")
	}
	end := strings.Index(source[start:], "\n}")
	if end < 0 {
		t.Fatal("SchedulerResources struct should have a closing brace")
	}
	resourcesSource := source[start : start+end]

	for _, marker := range []string{
		"Runtime        runner.SchedulerRuntimeProvider",
		"FactoryRuntime SchedulerFactoryRuntime",
		"CrawlSource    ports.CrawlSource",
	} {
		if strings.Contains(resourcesSource, marker) {
			t.Fatalf("shared_resources.go mentions %q; expose scheduler resources through methods instead of fields", marker)
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
