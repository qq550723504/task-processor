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
	if strings.Contains(methodsSource, "func (d *runtimeDeps) managementClient(") {
		t.Fatal("runtime_deps_methods.go should not reintroduce runtimeDeps.managementClient")
	}
	for _, marker := range []string{
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
		"bootstrapresources.BuildSharedResources(",
		"bootstrapresources.SharedResourceOptions{",
		"AllowMissingManagementAuth: true",
		"SkipManagementAuth:         true",
	} {
		if strings.Contains(runtimeSource, marker) {
			t.Fatalf("runtime.go should keep shared resource assembly in runtime_shared_resources.go; found %s", marker)
		}
	}

	sharedDepsSource := readHTTPAPIBoundaryFile(t, "runtime_shared_deps.go")
	for _, marker := range []string{
		`"task-processor/internal/app/bootstrap/resources"`,
		"*bootstrapresources.SharedResources",
		"sharedResources",
	} {
		if strings.Contains(sharedDepsSource, marker) {
			t.Fatalf("runtime_shared_deps.go mentions %s; keep HTTP API shared deps on narrow StoreAPI instead of bootstrap SharedResources", marker)
		}
	}
	if !strings.Contains(compactHTTPAPISource(sharedDepsSource), "storeAPIlistingadmin.StoreAPI") {
		t.Fatalf("runtime_shared_deps.go should store the narrow listingadmin.StoreAPI dependency for login modules")
	}

	loginModulesSource := readHTTPAPIBoundaryFile(t, "runtime_login_modules.go")
	for _, marker := range []string{
		"deps.shared.sharedResources",
		"sharedResources.StoreAPI",
	} {
		if strings.Contains(loginModulesSource, marker) {
			t.Fatalf("runtime_login_modules.go mentions %s; use deps.shared.storeAPI directly", marker)
		}
	}
	if !strings.Contains(loginModulesSource, "return deps.shared.storeAPI") {
		t.Fatalf("runtime_login_modules.go should return deps.shared.storeAPI")
	}

	sharedResourcesSource := readHTTPAPIBoundaryFile(t, "runtime_shared_resources.go")
	for _, marker := range []string{
		`"task-processor/internal/app/bootstrap/resources"`,
		"func buildHTTPAPISharedResources(",
		"bootstrapresources.BuildSharedResources(",
		"bootstrapresources.SharedResourceOptions{",
	} {
		if !strings.Contains(sharedResourcesSource, marker) {
			t.Fatalf("runtime_shared_resources.go missing %s", marker)
		}
	}
	for _, marker := range []string{
		"AllowMissingManagementAuth",
		"SkipManagementAuth",
	} {
		if strings.Contains(sharedResourcesSource, marker) {
			t.Fatalf("runtime_shared_resources.go mentions retired shared-resource option %s", marker)
		}
	}
}

func compactHTTPAPISource(source string) string {
	replacer := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "")
	return replacer.Replace(source)
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

func TestHTTPAPIRuntimeKeepsConfigLoadingDedicated(t *testing.T) {
	runtimeSource := readHTTPAPIBoundaryFile(t, "runtime.go")
	for _, marker := range []string{
		`"task-processor/internal/core/config"`,
		"config.LoadConfigFromFile(",
		`fmt.Errorf("load config: %w", err)`,
	} {
		if strings.Contains(runtimeSource, marker) {
			t.Fatalf("runtime.go should keep config loading in runtime_config.go; found %s", marker)
		}
	}

	configSource := readHTTPAPIBoundaryFile(t, "runtime_config.go")
	for _, marker := range []string{
		`"task-processor/internal/core/config"`,
		"func loadHTTPAPIConfig(",
		"config.LoadConfigFromFile(",
		`fmt.Errorf("load config: %w", err)`,
	} {
		if !strings.Contains(configSource, marker) {
			t.Fatalf("runtime_config.go missing %s", marker)
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
