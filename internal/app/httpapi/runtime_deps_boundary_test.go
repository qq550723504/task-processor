package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestHTTPAPITypesKeepExternalClientRuntimeDepsDedicated(t *testing.T) {
	typesSource := readHTTPAPIBoundaryFile(t, "types.go")
	for _, marker := range []string{
		`"task-processor/internal/infra/clients/openai"`,
		"type sharedRuntimeDeps struct",
		"openaiMgr",
		"aiCredentialStore",
	} {
		if strings.Contains(typesSource, marker) {
			t.Fatalf("types.go should keep concrete external-client runtime deps in runtime_shared_deps.go; found %s", marker)
		}
	}

	runtimeDepsSource := readHTTPAPIBoundaryFile(t, "runtime_shared_deps.go")
	for _, marker := range []string{
		`"task-processor/internal/infra/clients/openai"`,
		"type sharedRuntimeDeps struct",
		"openaiMgr",
		"aiCredentialStore",
	} {
		if !strings.Contains(runtimeDepsSource, marker) {
			t.Fatalf("runtime_shared_deps.go missing %s", marker)
		}
	}
}

func TestHTTPAPIAdaptersKeepOpenAIAssemblyDedicated(t *testing.T) {
	adaptersSource := readHTTPAPIBoundaryFile(t, "adapters.go")
	for _, marker := range []string{
		`"task-processor/internal/infra/clients/openai"`,
		"func newOpenAIManager(",
		"func newDBOpenAICredentialResolver(",
		"openaiclient.NewManager(",
		"openaiclient.NewGormCredentialResolver(",
	} {
		if strings.Contains(adaptersSource, marker) {
			t.Fatalf("adapters.go should keep concrete OpenAI runtime adapter assembly in adapters_openai.go; found %s", marker)
		}
	}

	openAIAdaptersSource := readHTTPAPIBoundaryFile(t, "adapters_openai.go")
	for _, marker := range []string{
		`"task-processor/internal/infra/clients/openai"`,
		"func newOpenAIManager(",
		"func newDBOpenAICredentialResolver(",
		"openaiclient.NewManager(",
		"openaiclient.NewGormCredentialResolver(",
	} {
		if !strings.Contains(openAIAdaptersSource, marker) {
			t.Fatalf("adapters_openai.go missing %s", marker)
		}
	}
}

func TestHTTPAPIAdaptersKeepTaskRepositoryAssemblyDedicated(t *testing.T) {
	adaptersSource := readHTTPAPIBoundaryFile(t, "adapters.go")
	for _, marker := range []string{
		`"task-processor/internal/amazonlisting/store"`,
		`"task-processor/internal/productenrich/store"`,
		`"task-processor/internal/productimage/store"`,
		"func newDBTaskRepository(",
		"func newDBImageTaskRepository(",
		"func newDBAmazonListingTaskRepository(",
		"store.NewTaskRepository(",
		"productimagestore.NewTaskRepository(",
		"amazonlistingstore.NewTaskRepository(",
	} {
		if strings.Contains(adaptersSource, marker) {
			t.Fatalf("adapters.go should keep task repository adapter assembly in adapters_task_repositories.go; found %s", marker)
		}
	}

	repositoryAdaptersSource := readHTTPAPIBoundaryFile(t, "adapters_task_repositories.go")
	for _, marker := range []string{
		`"task-processor/internal/amazonlisting/store"`,
		`"task-processor/internal/productenrich/store"`,
		`"task-processor/internal/productimage/store"`,
		"func newDBTaskRepository(",
		"func newDBImageTaskRepository(",
		"func newDBAmazonListingTaskRepository(",
		"store.NewTaskRepository(",
		"productimagestore.NewTaskRepository(",
		"amazonlistingstore.NewTaskRepository(",
	} {
		if !strings.Contains(repositoryAdaptersSource, marker) {
			t.Fatalf("adapters_task_repositories.go missing %s", marker)
		}
	}
}

func TestHTTPAPIAdaptersKeepPromptStoreAssemblyDedicated(t *testing.T) {
	adaptersSource := readHTTPAPIBoundaryFile(t, "adapters.go")
	for _, marker := range []string{
		`"task-processor/internal/app/bootstrap/resources"`,
		`"task-processor/internal/prompt"`,
		"func newDBTenantPromptStore(",
		"prompt.NewGormTenantPromptStore(",
		"bootstrapresources.NewDBTenantPromptStore(",
	} {
		if strings.Contains(adaptersSource, marker) {
			t.Fatalf("adapters.go should keep tenant prompt store adapter assembly in adapters_prompt.go; found %s", marker)
		}
	}

	promptAdaptersSource := readHTTPAPIBoundaryFile(t, "adapters_prompt.go")
	for _, marker := range []string{
		`"task-processor/internal/app/bootstrap/resources"`,
		`"task-processor/internal/prompt"`,
		"func newDBTenantPromptStore(",
		"prompt.NewGormTenantPromptStore(",
		"bootstrapresources.NewDBTenantPromptStore(",
	} {
		if !strings.Contains(promptAdaptersSource, marker) {
			t.Fatalf("adapters_prompt.go missing %s", marker)
		}
	}
}

func TestHTTPAPIRuntimeKeepsRuntimeDepsMethodsDedicated(t *testing.T) {
	runtimeSource := readHTTPAPIBoundaryFile(t, "runtime.go")
	for _, marker := range []string{
		"func (d *runtimeDeps) managementClient(",
		"func (d *runtimeDeps) ensureListingKitSupport(",
		"func (d *runtimeDeps) addClosers(",
		"func (d *runtimeDeps) attachProductModule(",
		"func (d *runtimeDeps) attachImageModule(",
		"func (d *runtimeDeps) attachAmazonListingModule(",
		"func (d *runtimeDeps) attachListingKitModule(",
		"func (d *runtimeDeps) attachSDSLoginResult(",
	} {
		if strings.Contains(runtimeSource, marker) {
			t.Fatalf("runtime.go should keep runtimeDeps state helpers in runtime_deps_methods.go; found %s", marker)
		}
	}

	methodsSource := readHTTPAPIBoundaryFile(t, "runtime_deps_methods.go")
	for _, marker := range []string{
		"func (d *runtimeDeps) managementClient(",
		"func (d *runtimeDeps) ensureListingKitSupport(",
		"func (d *runtimeDeps) addClosers(",
		"func (d *runtimeDeps) attachProductModule(",
		"func (d *runtimeDeps) attachImageModule(",
		"func (d *runtimeDeps) attachAmazonListingModule(",
		"func (d *runtimeDeps) attachListingKitModule(",
		"func (d *runtimeDeps) attachSDSLoginResult(",
	} {
		if !strings.Contains(methodsSource, marker) {
			t.Fatalf("runtime_deps_methods.go missing %s", marker)
		}
	}
}

func TestHTTPAPIRuntimeKeepsPromptRuntimeAssemblyDedicated(t *testing.T) {
	runtimeSource := readHTTPAPIBoundaryFile(t, "runtime.go")
	for _, marker := range []string{
		`"task-processor/internal/prompt"`,
		"prompt.TenantPromptStore",
		"initPromptRegistry(",
		"initTenantPromptStore(",
		"attachTenantPromptStore(",
		"prompt.InitGlobal(",
		"prompt.SetTenantPromptStore(",
		"cfg.Prompts.Dir",
		"cfg.Prompts.HotReload",
	} {
		if strings.Contains(runtimeSource, marker) {
			t.Fatalf("runtime.go should keep prompt runtime assembly in runtime_prompt.go; found %s", marker)
		}
	}

	promptRuntimeSource := readHTTPAPIBoundaryFile(t, "runtime_prompt.go")
	for _, marker := range []string{
		"type promptRuntimeDeps struct",
		"func buildPromptRuntimeDeps(",
		"func initPromptRegistry(",
		"func initTenantPromptStore(",
		"prompt.InitGlobal(",
		"prompt.SetTenantPromptStore(",
		"cfg.Prompts.Dir",
		"cfg.Prompts.HotReload",
	} {
		if !strings.Contains(promptRuntimeSource, marker) {
			t.Fatalf("runtime_prompt.go missing %s", marker)
		}
	}
}

func TestHTTPAPIRuntimeKeepsProductEnrichRuntimeAssemblyDedicated(t *testing.T) {
	runtimeSource := readHTTPAPIBoundaryFile(t, "runtime.go")
	for _, marker := range []string{
		"productenrich.NewLLMManagerAdapterFromManager(",
		"productenrich.NewLocalMockLLMManager(",
		"productenrich.ValidateMockLLMManager(",
		"productenrichenrich.NewProductUnderstanding(",
		"productenrichenrich.NewInputParser(",
	} {
		if strings.Contains(runtimeSource, marker) {
			t.Fatalf("runtime.go should keep ProductEnrich runtime assembly in runtime_productenrich.go; found %s", marker)
		}
	}

	productEnrichRuntimeSource := readHTTPAPIBoundaryFile(t, "runtime_productenrich.go")
	for _, marker := range []string{
		"func buildProductEnrichRuntimeDeps(",
		"productenrich.NewLLMManagerAdapterFromManager(",
		"productenrich.NewLocalMockLLMManager(",
		"productenrich.ValidateMockLLMManager(",
		"productenrichenrich.NewProductUnderstanding(",
		"productenrichenrich.NewInputParser(",
	} {
		if !strings.Contains(productEnrichRuntimeSource, marker) {
			t.Fatalf("runtime_productenrich.go missing %s", marker)
		}
	}
}

func TestHTTPAPIRuntimeKeepsSharedResourceAssemblyDedicated(t *testing.T) {
	runtimeSource := readHTTPAPIBoundaryFile(t, "runtime.go")
	for _, marker := range []string{
		`"task-processor/internal/app/bootstrap"`,
		"appbootstrap.BuildSharedResources(",
		"appbootstrap.SharedResourceOptions{",
		"AllowMissingManagementAuth: true",
		"SkipManagementAuth:         true",
	} {
		if strings.Contains(runtimeSource, marker) {
			t.Fatalf("runtime.go should keep shared resource assembly in runtime_shared_resources.go; found %s", marker)
		}
	}

	sharedResourcesSource := readHTTPAPIBoundaryFile(t, "runtime_shared_resources.go")
	for _, marker := range []string{
		`"task-processor/internal/app/bootstrap"`,
		"func buildHTTPAPISharedResources(",
		"appbootstrap.BuildSharedResources(",
		"appbootstrap.SharedResourceOptions{",
		"AllowMissingManagementAuth: true",
		"SkipManagementAuth:         true",
	} {
		if !strings.Contains(sharedResourcesSource, marker) {
			t.Fatalf("runtime_shared_resources.go missing %s", marker)
		}
	}
}

func TestHTTPAPIRuntimeKeepsOpenAIRuntimeAssemblyDedicated(t *testing.T) {
	runtimeSource := readHTTPAPIBoundaryFile(t, "runtime.go")
	for _, marker := range []string{
		`"task-processor/internal/infra/clients/openai"`,
		"newOpenAIManager(",
		"newDBOpenAICredentialResolver(",
		"SetConfigResolver(",
		"*openaiclient.GormCredentialResolver",
	} {
		if strings.Contains(runtimeSource, marker) {
			t.Fatalf("runtime.go should keep OpenAI runtime assembly in runtime_openai.go; found %s", marker)
		}
	}

	openAIRuntimeSource := readHTTPAPIBoundaryFile(t, "runtime_openai.go")
	for _, marker := range []string{
		`"task-processor/internal/infra/clients/openai"`,
		"type openAIRuntimeDeps struct",
		"func buildOpenAIRuntimeDeps(",
		"newOpenAIManager(",
		"newDBOpenAICredentialResolver(",
		"SetConfigResolver(",
		"*openaiclient.GormCredentialResolver",
	} {
		if !strings.Contains(openAIRuntimeSource, marker) {
			t.Fatalf("runtime_openai.go missing %s", marker)
		}
	}
}

func TestHTTPAPIRuntimeKeepsPathResolutionDedicated(t *testing.T) {
	runtimeSource := readHTTPAPIBoundaryFile(t, "runtime.go")
	for _, marker := range []string{
		`"path/filepath"`,
		"func resolveImageWorkDir(",
		"filepath.Clean(",
		`filepath.Join(".", "tmp", "productimage")`,
	} {
		if strings.Contains(runtimeSource, marker) {
			t.Fatalf("runtime.go should keep path resolution in runtime_paths.go; found %s", marker)
		}
	}

	pathSource := readHTTPAPIBoundaryFile(t, "runtime_paths.go")
	for _, marker := range []string{
		`"path/filepath"`,
		"func resolveImageWorkDir(",
		"filepath.Clean(",
		`filepath.Join(".", "tmp", "productimage")`,
	} {
		if !strings.Contains(pathSource, marker) {
			t.Fatalf("runtime_paths.go missing %s", marker)
		}
	}
}

func readHTTPAPIBoundaryFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
