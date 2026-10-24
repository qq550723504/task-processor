package tests

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var retiredHostRuntimePackages = []string{
	"internal/infra/httpx", "internal/infra/auth", "internal/app/monitoring",
	"internal/app/lifecycle", "internal/core/errors", "internal/app/ports", "internal/app/updater",
}

var retiredListingPreviewDownloadPackages = []string{
	"internal/listing/preview", "internal/listing/platform", "internal/pkg/downloader",
}

var retiredSheinPolicyContractPackages = []string{
	"internal/shein/activity", "internal/shein/productsync",
	"internal/shein/generationtopics", "internal/shein/namelimit",
}

func TestLegacyAmazonSourceAdapterClosureIsRetired(t *testing.T) {
	for _, relative := range []string{
		"internal/integration/crawler/amazon",
		"internal/integration/crawler/amazon/processor.go", "internal/integration/crawler/amazon/product_source.go",
		"internal/integration/crawler/amazon/processor_test.go", "internal/integration/crawler/amazon/product_source_test.go",
		"internal/integration/crawler/amazon/README.md",
		"internal/crawler/amazon/factory.go", "internal/crawler/amazon/scraper.go",
	} {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired Amazon source adapter artifact %s must be absent: %v", relative, err)
		}
	}
	retained := map[string]bool{}
	for _, name := range strings.Fields("AmazonProcessor NewAmazonProcessor effectiveBrowserPoolSize AmazonProcessor.Process AmazonProcessor.ProcessWithContext AmazonProcessor.ProcessBatch AmazonProcessor.ProcessBatchWithContext AmazonProcessor.Shutdown") {
		retained[name] = false
	}
	for _, relative := range trackedFiles(t, ".") {
		if !strings.HasSuffix(relative, ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", filepath.FromSlash(relative)), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			imported, err := decodeGoImportPath(spec.Path.Value)
			if err != nil {
				t.Fatalf("decode import in %s: %v", relative, err)
			}
			if importMatchesPrefix(imported, "task-processor/internal/integration/crawler/amazon") {
				t.Errorf("%s imports retired Amazon source adapter %s", relative, imported)
			}
		}
		// Only the original browser owner loses these declarations. Current
		// Source/Processor/Scraper contracts in other owners remain valid.
		if filepath.ToSlash(filepath.Dir(relative)) != "internal/crawler/amazon" || strings.HasSuffix(relative, "_test.go") {
			continue
		}
		for _, declaration := range file.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				name := declaration.Name.Name
				if declaration.Recv == nil && name == "CreateProcessor" {
					t.Errorf("%s retains retired Amazon factory CreateProcessor", relative)
				}
				if declaration.Recv != nil && len(declaration.Recv.List) > 0 {
					shape := declaration.Recv.List[0].Type
					if pointer, ok := shape.(*ast.StarExpr); ok {
						shape = pointer.X
					}
					if receiver, ok := shape.(*ast.Ident); ok {
						name = receiver.Name + "." + name
					}
				}
				if _, exists := retained[name]; exists {
					retained[name] = true
				}
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					if typ, ok := spec.(*ast.TypeSpec); ok {
						if typ.Name.Name == "Scraper" {
							t.Errorf("%s retains retired Amazon Scraper declaration", relative)
						}
						if _, exists := retained[typ.Name.Name]; exists {
							retained[typ.Name.Name] = true
						}
					}
				}
			}
		}
	}
	for name, found := range retained {
		if !found {
			t.Errorf("original Amazon browser owner must retain %s", name)
		}
	}
	for relative, tokens := range map[string][]string{
		"internal/crawler/amazon/factory_test.go": {
			"func TestEffectiveBrowserPoolSizeUsesConfiguredValue(", "func TestEffectiveBrowserPoolSizeFallsBackToAmazonDefault(", "effectiveBrowserPoolSize(",
		},
		"hack/debug/test-amazon/main.go": {
			"\"task-processor/internal/crawler/amazon\"", "amazon.NewAmazonProcessor(", "processor.Process(", "processor.Shutdown()",
		},
		"internal/app/runtime/currentapplication/runtime.go": {
			"\"task-processor/internal/integration/acquisition/amazon\"", "amazon.New(amazon.Options{",
		},
	} {
		body, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range tokens {
			if !strings.Contains(string(body), token) {
				t.Errorf("current Amazon consumer %s must retain %q", relative, token)
			}
		}
	}
}

func TestLegacyWorkerpoolSensitiveWordRecoveryClosureIsRetired(t *testing.T) {
	exprString := func(node ast.Node) string {
		var formatted bytes.Buffer
		if err := format.Node(&formatted, token.NewFileSet(), node); err != nil {
			t.Fatal(err)
		}
		return formatted.String()
	}
	for _, relative := range []string{
		"internal/platform/workerpool", "internal/pkg/recovery",
		"internal/platform/workerpool/config.go", "internal/platform/workerpool/interfaces.go",
		"internal/platform/workerpool/metrics.go", "internal/platform/workerpool/pool.go",
		"internal/platform/workerpool/types.go", "internal/platform/workerpool/worker.go",
		"internal/platform/workerpool/pool_lifecycle_test.go", "internal/platform/workerpool/worker_test.go",
		"internal/pkg/recovery/panic_handler.go",
	} {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired workerpool/recovery artifact %s must be absent: %v", relative, err)
		}
	}
	retiredFunctions := strings.Fields("resolveConfigPath NewSensitiveWordService NewSensitiveWordServiceInMemoryWithPath NewSensitiveWordServiceWithPath")
	retiredMethods := strings.Fields("loadConfig saveConfig saveConfigUnlocked ReloadConfig saveConfigAsync Close startSaveWorker logConfigLoadStats")
	retiredFields := strings.Fields("configPath persistConfig ctx saveQueue stopSave wg")
	retained := map[string]bool{}
	for _, name := range strings.Fields("NewSensitiveWordServiceInMemory SensitiveWordConfig SensitiveWordService SensitiveWordService.config SensitiveWordService.mutex SensitiveWordService.logger createDefaultConfig ProcessProductData HandleValidationErrors GetSensitiveWordsByLanguage getWordsFromConfig classifyWordsByLanguage addWordsToConfig deduplicateWords") {
		retained[name] = false
	}
	wantConfig := map[string]string{
		"StaticWords": "map[string][]string|static_words", "DynamicWords": "map[string][]string|dynamic_words",
		"LastUpdated": "time.Time|last_updated", "Version": "string|version",
	}
	gotConfig := map[string]string{}
	for _, relative := range trackedFiles(t, ".") {
		if !strings.HasSuffix(relative, ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", filepath.FromSlash(relative)), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		content := filepath.ToSlash(filepath.Dir(relative)) == "internal/shein/content" && !strings.HasSuffix(relative, "_test.go")
		for _, spec := range file.Imports {
			imported, err := decodeGoImportPath(spec.Path.Value)
			if err != nil {
				t.Fatalf("decode import in %s: %v", relative, err)
			}
			for _, retired := range []string{"task-processor/internal/platform/workerpool", "task-processor/internal/pkg/recovery"} {
				if importMatchesPrefix(imported, retired) {
					t.Errorf("%s imports retired package %s", relative, imported)
				}
			}
			if content && (imported == "os" || imported == "path/filepath") {
				t.Errorf("content must not retain file persistence import %s in %s", imported, relative)
			}
		}
		if !content {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if _, ok := node.(*ast.GoStmt); ok {
				t.Errorf("content must not retain a save goroutine in %s", relative)
			}
			return true
		})
		for _, declaration := range file.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				name := declaration.Name.Name
				receiver := ""
				if declaration.Recv != nil && len(declaration.Recv.List) > 0 {
					shape := declaration.Recv.List[0].Type
					if pointer, ok := shape.(*ast.StarExpr); ok {
						shape = pointer.X
					}
					if ident, ok := shape.(*ast.Ident); ok {
						receiver = ident.Name
					}
				}
				if (receiver == "" && slices.Contains(retiredFunctions, name)) || (receiver == "SensitiveWordService" && slices.Contains(retiredMethods, name)) {
					t.Errorf("%s retains retired content declaration %s.%s", relative, receiver, name)
				}
				if _, exists := retained[name]; exists && (receiver == "SensitiveWordService" || receiver == "") {
					retained[name] = true
				}
				if receiver == "" && name == "NewSensitiveWordServiceInMemory" {
					params := declaration.Type.Params.List
					if len(params) != 1 || len(params[0].Names) != 1 || params[0].Names[0].Name != "logs" || exprString(params[0].Type) != "func(string) *logrus.Entry" {
						t.Errorf("current in-memory factory must retain its explicit logs callback")
					}
				}
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					typ, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if _, exists := retained[typ.Name.Name]; exists {
						retained[typ.Name.Name] = true
					}
					shape, ok := typ.Type.(*ast.StructType)
					if !ok {
						continue
					}
					for _, field := range shape.Fields.List {
						for _, name := range field.Names {
							key := typ.Name.Name + "." + name.Name
							if _, exists := retained[key]; exists {
								retained[key] = true
							}
							if typ.Name.Name == "SensitiveWordService" && slices.Contains(retiredFields, name.Name) {
								t.Errorf("content retains retired lifecycle field %s", key)
							}
							if typ.Name.Name == "SensitiveWordConfig" {
								tag := ""
								if field.Tag != nil {
									decoded, err := decodeGoImportPath(field.Tag.Value)
									if err != nil {
										t.Fatal(err)
									}
									tag = reflect.StructTag(decoded).Get("json")
								}
								gotConfig[name.Name] = exprString(field.Type) + "|" + tag
							}
						}
					}
				}
			}
		}
	}
	for name, found := range retained {
		if !found {
			t.Errorf("current content owner must retain %s", name)
		}
	}
	if !reflect.DeepEqual(gotConfig, wantConfig) {
		t.Errorf("sensitive-word configuration contract changed: got %v, want %v", gotConfig, wantConfig)
	}
}

func TestLegacyRuntimeSkeletonAndIdleConfigClosureIsRetired(t *testing.T) {
	for _, relative := range []string{"internal/app/runtime/temporal_bundle.go", "internal/app/runtime/temporal_bundle_test.go"} {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired Temporal module bundle %s must be absent: %v", relative, err)
		}
	}
	for _, relative := range []string{"internal/core/config/platform_common.go", "internal/core/config/platform_common_test.go", "internal/pkg/jsonx/marshal.go", "internal/core/config/type_server.go", "internal/core/config/validator_validator.go", "internal/shared/aiidentity/envelope.go", "internal/shared/aiidentity/envelope_test.go", "internal/shared/aiidentity/tenant_scope.go", "internal/shared/aiidentity/tenant_scope_test.go", "internal/platform/logging/context.go", "internal/platform/logging/helpers.go", "internal/platform/logging/helpers_test.go", "internal/shared/resilience/breaker.go", "internal/shared/resilience/breaker_test.go", "internal/shared/resilience/ratelimit.go", "internal/shared/resilience/ratelimit_test.go", "internal/integration/openai/cached_client.go", "internal/integration/openai/resilient_client.go", "internal/pkg/httpclient/builder.go", "internal/pkg/httpclient/builder_test.go", "internal/shared/aiidentity/request_roles.go"} {
		if _, err := os.Lstat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired idle support file %s must be absent: %v", relative, err)
		}
	}
	retiredKernelTypes := []string{"TaskHandler", "WorkflowHandler", "TemporalWorkerStarter", "NamedWorkerPool", "NamedTemporalWorker", "WorkflowRegistry"}
	retiredRegistryMethods := []string{"AddWorkerPool", "WorkerPools", "AddTemporalWorker", "TemporalWorkers", "RegisterTaskHandler", "RegisterWorkflowHandler"}
	retiredRegistryFields := []string{"workerPools", "workerPoolNames", "temporalWorkers", "temporalWorkerNames", "taskHandlers", "workflowNames"}
	retiredConfig := []string{"CacheConfig", "DefaultCacheConfig", "LogConfig", "DefaultLogConfig", "MonitoringConfig", "DefaultMonitoringConfig", "CostCalculationConfig", "DefaultCostCalculationConfig", "TimeoutConfig", "DefaultTimeoutConfig", "RateLimitConfig", "DefaultRateLimitConfig", "ServerConfig"}
	retiredSupport := map[string][]string{
		"internal/platform/database":  {"openExistingReadOnly", "OpenExistingReadOnly", "OpenExistingWritable"},
		"internal/shared/ptr":         {"Int16Ptr", "Int32Ptr", "Int64Ptr", "StringPtr", "Float64Ptr", "BoolPtr"},
		"internal/integration/openai": {"CachedClient", "Cache", "CachedClientConfig", "NewCachedClient", "CachedClient.CreateChatCompletion", "CachedClient.generateCacheKey", "CachedClient.Generate", "CachedClient.AnalyzeImage", "CachedClient.GetDefaultModel", "CachedClient.SupportsAsyncImageGeneration", "CachedClient.SubmitImageGeneration", "CachedClient.SubmitImageEdit", "CachedClient.QueryImageGeneration", "CachedClient.ClearCache", "CachedClient.SetTTL", "CachedClient.Enable", "CachedClient.Disable", "CachedClient.IsEnabled", "CachedClient.GetStats", "CachedClient.Close", "ResilientClient", "ResilientClientConfig", "CircuitBreakerConfig", "CircuitState", "StateClosed", "StateOpen", "StateHalfOpen", "CircuitBreaker", "Counts", "DefaultCircuitBreakerConfig", "NewCircuitBreaker", "CircuitBreaker.Execute", "CircuitBreaker.beforeRequest", "CircuitBreaker.afterRequest", "CircuitBreaker.onSuccess", "CircuitBreaker.onFailure", "CircuitBreaker.setState", "CircuitBreaker.State", "CircuitBreaker.Counts", "CircuitBreaker.Reset", "CircuitBreaker.OnStateChange", "NewResilientClient", "ResilientClient.CreateChatCompletion", "ResilientClient.Generate", "ResilientClient.AnalyzeImage", "ResilientClient.GetDefaultModel", "ResilientClient.SupportsAsyncImageGeneration", "ResilientClient.SubmitImageGeneration", "ResilientClient.SubmitImageEdit", "ResilientClient.QueryImageGeneration", "ResilientClient.GetCircuitBreakerState", "ResilientClient.GetCircuitBreakerCounts", "ResilientClient.ResetCircuitBreaker", "ResilientClient.GetStats", "ResilientClient.Close", "stateString"},
		"internal/platform/logging":   {"contextKey", "loggerContextKey", "traceIDKey", "requestIDKey", "WithLogger", "FromContext", "WithTraceID", "GetTraceID", "WithRequestID", "GetRequestID", "WithFields", "WithField", "FieldComponent", "FieldPlatform", "FieldProductID", "FieldTraceID", "FieldRequestID", "FieldDurationMs", "FieldRetryCount", "FieldErrorCode", "FieldErrorType", "FieldOperation", "FieldStatus", "FieldUserID", "FieldSessionID", "LoggerHelper", "NewLoggerHelper", "LoggerHelper.LogOperation", "LoggerHelper.LogOperationWithResult", "LoggerHelper.LogProgress", "LoggerHelper.LogRetry", "LoggerHelper.LogAPICall", "LoggerHelper.LogCacheHit", "LoggerHelper.LogDatabaseQuery", "LoggerHelper.LogStateChange", "LoggerHelper.LogMetric", "WithComponent", "WithPlatform", "ShouldLog", "LoggerHelper.LogTaskStart", "LoggerHelper.LogTaskComplete", "LoggerHelper.LogTaskFailed", "WithTaskContext", "WithStoreContext", "FieldTaskID", "FieldTenantID", "FieldStoreID", "SetGlobalLogLevel"},
		"internal/shared/resilience":  {"ErrCircuitOpen", "ErrTooManyRequests", "CircuitState", "StateClosed", "StateHalfOpen", "StateOpen", "CircuitBreakerCounts", "CircuitBreakerConfig", "CircuitBreaker", "NewCircuitBreaker", "CircuitBreaker.Execute", "CircuitBreaker.State", "CircuitBreaker.Counts", "convertCounts", "convertState", "RateLimiter", "NewRateLimiter", "RateLimiter.Wait", "RateLimiter.Allow"},
		"internal/shared/aiidentity":  {"CurrentEnvelopeVersion", "ErrMissingIdentity", "ErrIdentityIntegrity", "ExecutionEnvelope", "PersistedExecutionEnvelope", "PersistedEnvelopeState", "PersistedEnvelopeAbsent", "PersistedEnvelopePartial", "PersistedEnvelopePresent", "envelopeContextKey", "ExecutionEnvelope.Validate", "CaptureExecutionEnvelope", "WithExecutionEnvelope", "ExecutionEnvelopeFromContext", "RestoreExecutionEnvelope", "EnsureExecutionEnvelopeContext", "PersistedExecutionEnvelopeFrom", "PersistedExecutionEnvelope.State", "PersistedExecutionEnvelope.ExecutionEnvelope", "PersistedExecutionEnvelope.executionEnvelope", "normalizePersistedEnvelope", "normalizeEnvelope", "validSource", "TenantIDFromContext", "NormalizePersistedTenantID", "TenantMatchesContext", "TenantCanCreateTask", "requestRolesContextKey", "RequestUserIDFromContext", "WithRequestRoles", "RequestRolesFromContext", "normalizeRequestRoles"},
		"internal/core/config":        {"CostCalculationConfig.CalculateTotalCost", "LoadConfigWithFallback", "LoadAmazonCrawlerAPIConfigWithFallback", "validateAmazonCrawlerAPIConfig", "Validator", "NewValidator", "Validator.Validate", "Validator.ValidateAndLog", "Validator.ValidateWithError", "Config.Validate", "Config.ValidateAndLog", "Config.ValidateWithError", "ValidateConfigAndLog"},
		"internal/pkg/goroutine":      {"TaskInfo", "Manager", "NewManager", "Manager.Start", "Manager.Wait", "Manager.WaitWithTimeout", "Manager.GetRunningCount", "Manager.GetStatus"},
		"internal/pkg/timeout":        {"AITimeout", "AILongTimeout", "HTTPTimeout", "HTTPShortTimeout", "HTTPLongTimeout", "TaskTimeout", "TaskShortTimeout", "TaskLongTimeout", "TaskExtraTimeout", "ShutdownTimeout", "HealthTimeout", "WithAITimeout", "WithAILongTimeout", "WithHTTPTimeout", "WithHTTPShortTimeout", "WithHTTPLongTimeout", "WithTaskTimeout", "WithTaskShortTimeout", "WithTaskLongTimeout", "WithTaskExtraTimeout", "WithDownloadTimeout", "WithShutdownTimeout", "WithHealthTimeout", "WithCustomTimeout"},
		"internal/pkg/jsonx":          {"MarshalWithoutHTMLEscape", "MarshalIndentWithoutHTMLEscape", "MarshalPretty", "UnmarshalStrict", "ToJSONString", "ToJSONStringPretty", "IsValidJSON", "CompactJSON", "SaveToFile", "UnmarshalBytes"},
		"internal/pkg/appenv":         {"SetupLogger", "LoggingConfig", "ApplyLoggingConfig", "InstanceInfo", "GetInstanceID", "GetPodName", "GetNamespace", "GetInstanceInfo", "ParsePlatformList", "ContainsPlatform"},
		"internal/pkg/httpclient":     {"DefaultConfig", "NewWithTransport", "ClientConfig", "Build", "buildTLSConfig", "NewSimple"},
		"internal/pkg/imagex":         {"Format", "FormatJPEG", "FormatPNG", "FormatAuto", "ToBase64", "ToBase64JPEG", "ToBase64PNG", "FromBase64", "FromBytes", "ToBytes", "FromReader", "Encode", "Size", "FormatFromBytes", "HasTransparency", "OptimalFormat"},
	}
	// Owner-qualified declarations avoid banning current same-name workflows,
	// platform logging or real workerpool/Temporal dependencies.
	retained := map[string]map[string]bool{
		"internal/shared/ptr":         {"IntPtr": false, "Float32Ptr": false},
		"internal/platform/database":  {"OpenExistingReadOnlyContext": false, "OpenExistingWritableContext": false, "Open": false, "OpenShared": false, "Close": false, "CloseShared": false, "openDatabase": false, "openDatabaseContext": false, "databaseDSN": false},
		"internal/shared/resilience":  {"Retry": false, "RetryConfig": false, "RetryAttempt": false},
		"internal/integration/openai": {"Client": false, "ClientConfig": false, "RequestPool": false, "PoolConfig": false, "BaseClient": false, "RateLimiter": false, "Manager": false, "ManagerConfig": false, "Logger": false, "AdaptLogrus": false},
		"internal/shared/aiidentity":  {"Identity": false, "Identity.AgentRunID": false, "Identity.TenantID": false, "Identity.UserID": false, "Identity.BusinessTaskID": false, "Identity.TraceID": false, "WithIdentity": false, "FromContext": false},
		"internal/kernel/module":      {"Module": false, "Registry": false, "NewRegistry": false, "Registry.AddRoutes": false, "Registry.Routes": false, "Registry.routes": false},
		"internal/core/config":        {"Config": false, "LoggingConfig": false, "HTTPClientConfig": false, "DefaultHTTPClientConfig": false, "RetryConfig": false, "DefaultRetryConfig": false, "LoadConfig": false, "LoadConfigFromFile": false, "LoadConfigFromFileWithoutValidation": false, "LoadFromBytes": false, "NewDefaultConfig": false, "BuildConfig": false, "ValidateOpenAIConfig": false, "ValidateBrowserConfig": false, "ValidateAmazonConfig": false, "ValidatePlatformsConfig": false, "ValidateListingKitConfig": false, "ValidateWorkbenchConfig": false, "ValidateConfig": false, "ValidateConfigWithError": false, "ValidationError": false, "formatValidationErrors": false, "classifyValidationError": false, "moduleFromField": false, "validationModuleRank": false},
		"internal/platform/logging":   {"LogConfig": false, "DefaultLogConfig": false, "LogManager": false, "NewLogManager": false, "InitGlobalLogger": false, "GetGlobalLogManager": false, "GetGlobalLogger": false, "LevelFileConfig": false, "LogManager.SetLevel": false, "LogManager.GetLevel": false},
		"internal/pkg/goroutine":      {"Task": false, "Result": false, "ProcessFunc": false, "Processor": false, "NewProcessor": false, "Processor.ProcessParallel": false, "Processor.worker": false, "Processor.executeTask": false},
		"internal/pkg/timeout":        {"AIShortTimeout": false, "DownloadTimeout": false, "DownloadLongTimeout": false, "WithAIShortTimeout": false},
		"internal/pkg/jsonx":          {"UnmarshalString": false, "CleanLLMResponse": false},
		"internal/pkg/appenv":         {"ComponentLoggers": false, "SetupLoggerWithLevel": false, "VersionInfo": false, "VersionInfo.Version": false, "VersionInfo.BuildTime": false, "PrintVersionInfo": false},
		"internal/pkg/httpclient":     {"Config": false, "Client": false, "New": false, "Client.Do": false, "NewSimpleWithTimeout": false},
		"internal/pkg/imagex":         {"ImageInfo": false, "ImageInfo.Width": false, "ImageInfo.Height": false, "ImageInfo.Format": false, "ImageInfo.Size": false, "FromBytesWithFormat": false, "Inspect": false},
	}
	decoders := map[string]bool{"image/jpeg": false, "image/png": false, "golang.org/x/image/webp": false}
	for _, relative := range trackedFiles(t, ".") {
		if !strings.HasSuffix(relative, ".go") {
			continue
		}
		owner := filepath.ToSlash(filepath.Dir(relative))
		if owner != "internal/kernel/module" && owner != "internal/app/httpapi" && owner != "internal/core/config" && owner != "internal/platform/logging" && owner != "internal/pkg/goroutine" && owner != "internal/pkg/timeout" && owner != "internal/pkg/jsonx" && owner != "internal/pkg/appenv" && owner != "internal/pkg/httpclient" && owner != "internal/pkg/imagex" && owner != "internal/shared/aiidentity" && owner != "internal/shared/resilience" && owner != "internal/integration/openai" && owner != "internal/shared/ptr" && owner != "internal/platform/database" {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", filepath.FromSlash(relative)), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		if relative == "internal/pkg/imagex/imageutil.go" {
			for _, spec := range file.Imports {
				imported, err := decodeGoImportPath(spec.Path.Value)
				if err != nil {
					t.Fatalf("decode image decoder import in %s: %v", relative, err)
				}
				if _, required := decoders[imported]; required {
					if spec.Name == nil || spec.Name.Name != "_" {
						t.Errorf("%s must explicitly retain production decoder registration for %s", relative, imported)
					} else {
						decoders[imported] = true
					}
				}
			}
		}
		if owner == "internal/kernel/module" {
			for _, spec := range file.Imports {
				imported, err := decodeGoImportPath(spec.Path.Value)
				if err != nil {
					t.Fatalf("decode import in %s: %v", relative, err)
				}
				if importMatchesPrefix(imported, "task-processor/internal/platform/workerpool") {
					t.Errorf("%s retains the retired kernel workerpool import %s", relative, imported)
				}
			}
		}
		for _, declaration := range file.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				name := declaration.Name.Name
				receiver := ""
				if declaration.Recv != nil && len(declaration.Recv.List) > 0 {
					shape := declaration.Recv.List[0].Type
					if pointer, ok := shape.(*ast.StarExpr); ok {
						shape = pointer.X
					}
					if typ, ok := shape.(*ast.Ident); ok {
						receiver = typ.Name
					}
				}
				if owner == "internal/kernel/module" && ((receiver == "Registry" && slices.Contains(retiredRegistryMethods, name)) || (receiver == "WorkflowRegistry" && name == "Register") || (receiver == "" && name == "isNilInterface")) {
					t.Errorf("%s retains retired kernel method/helper %s.%s", relative, receiver, name)
				}
				if owner == "internal/app/httpapi" && receiver == "runtimeBundle" && name == "pools" {
					t.Errorf("%s retains retired HTTP runtimeBundle.pools", relative)
				}
				if receiver == "" && owner == "internal/core/config" && slices.Contains(retiredConfig, name) {
					t.Errorf("%s retains retired core config declaration %s", relative, name)
				}
				if receiver != "" {
					name = receiver + "." + name
				}
				if slices.Contains(retiredSupport[owner], name) {
					t.Errorf("%s retains retired support declaration %s", relative, name)
				}
				if _, exists := retained[owner][name]; exists {
					retained[owner][name] = true
				}
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					if values, ok := spec.(*ast.ValueSpec); ok {
						for _, value := range values.Names {
							name := value.Name
							if slices.Contains(retiredSupport[owner], name) {
								t.Errorf("%s retains retired support value %s", relative, name)
							}
							if _, exists := retained[owner][name]; exists {
								retained[owner][name] = true
							}
						}
					}
					typ, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					name := typ.Name.Name
					if (owner == "internal/kernel/module" && slices.Contains(retiredKernelTypes, name)) || (owner == "internal/core/config" && slices.Contains(retiredConfig, name)) {
						t.Errorf("%s retains retired owner declaration %s", relative, name)
					}
					if slices.Contains(retiredSupport[owner], name) {
						t.Errorf("%s retains retired support type %s", relative, name)
					}
					if _, exists := retained[owner][name]; exists {
						retained[owner][name] = true
					}
					if shape, ok := typ.Type.(*ast.StructType); ok {
						for _, field := range shape.Fields.List {
							for _, fieldName := range field.Names {
								if owner == "internal/pkg/imagex" && name == "ImageInfo" {
									want := map[string]string{"Width": "int", "Height": "int", "Format": "string", "Size": "int"}[fieldName.Name]
									if want != "" {
										fieldType, ok := field.Type.(*ast.Ident)
										if !ok || fieldType.Name != want {
											t.Errorf("%s must retain ImageInfo.%s type %s", relative, fieldName.Name, want)
										}
									}
								}
								if (owner == "internal/kernel/module" && name == "Registry" && slices.Contains(retiredRegistryFields, fieldName.Name)) || (owner == "internal/app/httpapi" && name == "runtimeBundle" && fieldName.Name == "workerPools") {
									t.Errorf("%s retains retired field %s.%s", relative, name, fieldName.Name)
								}
								key := name + "." + fieldName.Name
								if _, exists := retained[owner][key]; exists {
									retained[owner][key] = true
								}
							}
						}
					}
				}
			}
		}
	}
	for owner, declarations := range retained {
		for name, found := range declarations {
			if !found {
				t.Errorf("current owner %s must retain %s", owner, name)
			}
		}
	}
	for imported, found := range decoders {
		if !found {
			t.Errorf("imageutil.go must retain production blank import %s", imported)
		}
	}
}

func assertLegacyLoggerFacadeRetired(t *testing.T) {
	t.Helper()
	for _, relative := range []string{"internal/core/logger", "internal/publishing/shein/logging.go"} {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired logger facade/helper %s must be absent: %v", relative, err)
		}
	}
	businessRoots := []string{"internal/amazon/api", "internal/crawler", "internal/localagent", "internal/prompt", "internal/publishing/shein", "internal/shein/client", "internal/shein/content", "internal/sheinlogin", "internal/temu/api/client"}
	for _, relative := range trackedFiles(t, ".") {
		if !strings.HasSuffix(relative, ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", filepath.FromSlash(relative)), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		business := false
		for _, root := range businessRoots {
			business = business || importMatchesPrefix(relative, root)
		}
		for _, spec := range file.Imports {
			imported, err := decodeGoImportPath(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if importMatchesPrefix(imported, "task-processor/internal/core/logger") ||
				(business && (importMatchesPrefix(imported, "task-processor/internal/platform/logging") || imported == "task-processor/internal/pkg/appenv")) {
				t.Errorf("%s imports retired/global logging owner %s; inject a consumer-local dependency", relative, imported)
			}
		}
		if !business {
			continue
		}
		if importMatchesPrefix(relative, "internal/prompt") {
			for _, declaration := range file.Decls {
				if function, ok := declaration.(*ast.FuncDecl); ok && function.Recv == nil && (function.Name.Name == "NewPromptLoader" || function.Name.Name == "NewFileWatcher") {
					t.Errorf("%s recreates the retired default diagnostic constructor %s", relative, function.Name.Name)
				}
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if selector, ok := node.(*ast.SelectorExpr); ok && (selector.Sel.Name == "GetGlobalLogger" || selector.Sel.Name == "GetGlobalLogManager") {
				t.Errorf("%s retains global logger lookup %s", relative, selector.Sel.Name)
			}
			if name, ok := node.(*ast.Ident); ok && name.Name == "sheinLoginServiceLogger" {
				t.Errorf("%s retains SHEIN package-initialization logger capture", relative)
			}
			return true
		})
	}
}

func TestLegacyLoggerFacadeClosureIsRetired(t *testing.T) {
	assertLegacyLoggerFacadeRetired(t)
	for _, command := range []string{"current-application", "shein-login-worker", "image-agent-temporal-worker"} {
		cmd := exec.Command("go", "list", "-deps", "./cmd/"+command)
		cmd.Dir = ".."
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go list %s: %v\n%s", command, err, output)
		}
		for _, dependency := range strings.Fields(string(output)) {
			if importMatchesPrefix(dependency, "task-processor/internal/core/logger") {
				t.Errorf("%s retains retired logger dependency %s", command, dependency)
			}
		}
	}
}

func assertLegacyListingPreviewDownloadSourcesRetired(t *testing.T) {
	t.Helper()
	for _, relative := range append(slices.Clone(retiredListingPreviewDownloadPackages),
		"internal/listing/export", "internal/listing/revision", "internal/listing/settings", "internal/listing/workflow") {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired listing preview/download owner %s must be absent: %v", relative, err)
		}
	}
	// Parse every tracked target, including tests and the independent debug module.
	// Import literals are decoded; aliases and escaped/raw strings cannot bypass it.
	for _, relative := range trackedFiles(t, ".") {
		if !strings.HasSuffix(relative, ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", filepath.FromSlash(relative)), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			imported, err := decodeGoImportPath(spec.Path.Value)
			if err != nil {
				t.Fatalf("decode import in %s: %v", relative, err)
			}
			for _, retired := range retiredListingPreviewDownloadPackages {
				if importMatchesPrefix(imported, "task-processor/"+retired) {
					t.Errorf("%s imports retired listing preview/download owner %s", relative, imported)
				}
			}
		}
	}
}

func TestLegacyListingPreviewDownloadClosureIsRetired(t *testing.T) {
	assertLegacyListingPreviewDownloadSourcesRetired(t)
	rule, ok := loadDepguardRules(t, filepath.Join("..", ".golangci.yml"))["listing_preview_platform_neutral"]
	if !ok || !reflect.DeepEqual(rule.Files, []string{"**/*.go"}) {
		t.Fatal("listing_preview_platform_neutral must block retired imports across production Go files")
	}
	var wantDenied []string
	for _, retired := range retiredListingPreviewDownloadPackages {
		wantDenied = append(wantDenied, "task-processor/"+retired+"$", "task-processor/"+retired+"/")
	}
	denied := depguardDenyPackageSet(rule)
	if len(denied) != len(wantDenied) {
		t.Errorf("retired preview/download depguard must only deny %v, got %v", wantDenied, denied)
	}
	for _, imported := range wantDenied {
		if _, exists := denied[imported]; !exists {
			t.Errorf("retired preview/download depguard must deny %s", imported)
		}
	}
	for _, command := range []string{"current-application", "shein-login-worker", "image-agent-temporal-worker"} {
		t.Run(command, func(t *testing.T) {
			cmd := exec.Command("go", "list", "-deps", "./cmd/"+command)
			cmd.Dir = ".."
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go list current command: %v\n%s", err, output)
			}
			for _, dependency := range strings.Fields(string(output)) {
				for _, retired := range retiredListingPreviewDownloadPackages {
					if importMatchesPrefix(dependency, "task-processor/"+retired) {
						t.Errorf("%s still compiles retired listing preview/download dependency %s", command, dependency)
					}
				}
			}
		})
	}
}

func TestLegacySDSRuntimeClosureIsRetired(t *testing.T) {
	retiredPackages := []string{
		"internal/sds/client", "internal/sds/template", "internal/sds/design",
		"internal/sds/workflow", "internal/sds/adapter", "internal/sds/usecase",
		"internal/sds/httpapi", "internal/sds/httpbootstrap", "internal/sdslogin",
	}
	isRetired := func(path string) bool {
		if importMatchesPrefix(path, "internal/sds/adapter/product_source") {
			return false
		}
		for _, retired := range retiredPackages {
			if importMatchesPrefix(path, retired) {
				return true
			}
		}
		return false
	}
	for _, relative := range []string{
		"internal/sds/adapter/service.go",
		"internal/sds/adapter/service_test.go",
		"internal/sds/adapter/types.go",
		"internal/sds/client/auth.go",
		"internal/sds/client/auth_bootstrap.go",
		"internal/sds/client/auth_bootstrap_local.go",
		"internal/sds/client/auth_bootstrap_local_test.go",
		"internal/sds/client/auth_bootstrap_test.go",
		"internal/sds/client/auth_state.go",
		"internal/sds/client/auth_test.go",
		"internal/sds/client/client.go",
		"internal/sds/client/config.go",
		"internal/sds/client/config_test.go",
		"internal/sds/client/errors.go",
		"internal/sds/client/request.go",
		"internal/sds/client/session_store.go",
		"internal/sds/client/session_store_test.go",
		"internal/sds/client/transient_upload_error.go",
		"internal/sds/client/transient_upload_error_test.go",
		"internal/sds/design/material_helpers.go",
		"internal/sds/design/models.go",
		"internal/sds/design/payload.go",
		"internal/sds/design/render_validation.go",
		"internal/sds/design/render_validation_test.go",
		"internal/sds/design/rendered_images.go",
		"internal/sds/design/sensitive_words.go",
		"internal/sds/design/service.go",
		"internal/sds/design/service_test.go",
		"internal/sds/design/types.go",
		"internal/sds/httpapi/config.go",
		"internal/sds/httpapi/config_test.go",
		"internal/sds/httpapi/handler.go",
		"internal/sds/httpapi/handler_test.go",
		"internal/sds/httpapi/http_module.go",
		"internal/sds/httpapi/http_module_test.go",
		"internal/sds/httpbootstrap/support.go",
		"internal/sds/httpbootstrap/support_test.go",
		"internal/sds/template/models.go",
		"internal/sds/template/service.go",
		"internal/sds/template/service_test.go",
		"internal/sds/template/types.go",
		"internal/sds/usecase/service.go",
		"internal/sds/usecase/service_test.go",
		"internal/sds/usecase/types.go",
		"internal/sds/workflow/approved_asset.go",
		"internal/sds/workflow/service.go",
		"internal/sds/workflow/service_test.go",
		"internal/sds/workflow/types.go",
		"internal/sdslogin/api.go",
		"internal/sdslogin/automation.go",
		"internal/sdslogin/automation_test.go",
		"internal/sdslogin/bootstrap/build.go",
		"internal/sdslogin/bootstrap/build_test.go",
		"internal/sdslogin/build.go",
		"internal/sdslogin/build_test.go",
		"internal/sdslogin/http_module.go",
		"internal/sdslogin/http_module_test.go",
		"internal/sdslogin/redis_store.go",
		"internal/sdslogin/service.go",
		"internal/sdslogin/service_test.go",
		"internal/sdslogin/types.go",
		"hack/debug/test-sds/main.go",
		"hack/debug/test-sds/README.md",
		"scripts/refresh-sds-auth-via-portforward.ps1",
		"tools/cloakbrowser-poc/package.json",
		"tools/cloakbrowser-poc/package-lock.json",
		"tools/cloakbrowser-poc/sds-login.mjs",
		"tools/cloakbrowser-poc/sds-login-redis.mjs",
		"internal/core/config/sds_auth_redis_test.go",
		"web/listingkit-ui/src/app/api/sds/categories/route.ts",
		"web/listingkit-ui/src/app/api/sds/products/[productId]/route.ts",
		"web/listingkit-ui/src/app/api/sds/products/route.ts",
		"web/listingkit-ui/src/app/api/sds/shared.ts",
		"web/listingkit-ui/src/app/api/sds/shipment-areas/route.ts",
		"web/listingkit-ui/src/lib/api/sds-products-schema.ts",
		"web/listingkit-ui/src/lib/api/sds-products-schema.test.ts",
		"web/listingkit-ui/src/lib/api/api-error.ts",
		"web/listingkit-ui/src/lib/api/response-schema.ts",
		"web/listingkit-ui/src/lib/types/sds.ts",
		"web/listingkit-ui/src/lib/utils/sds-recent-variants.ts",
		"web/listingkit-ui/src/lib/utils/sds-recent-variants.test.ts",
	} {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired SDS artifact %s must be absent: %v", relative, err)
		}
	}
	retiredDeclarations := strings.Fields("SDSPlatformConfig SDSAuthBootstrapConfig LoginServiceConfig SDSLoginServiceConfig EffectiveSDSAuthRedis BuildLoginServiceConfig BuildSDSAuthBootstrapConfig setSDSDefaults")
	for _, relative := range trackedFiles(t, ".") {
		if !strings.HasSuffix(relative, ".go") {
			continue
		}
		if isRetired(relative) {
			t.Errorf("retired SDS package source remains: %s", relative)
		}
		configOwner := strings.HasPrefix(relative, "internal/core/config/") && !strings.HasSuffix(relative, "_test.go")
		mode := parser.ImportsOnly
		if configOwner {
			mode = 0
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", filepath.FromSlash(relative)), nil, mode)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			imported, err := decodeGoImportPath(spec.Path.Value)
			if err != nil {
				t.Fatalf("decode import in %s: %v", relative, err)
			}
			if isRetired(strings.TrimPrefix(imported, "task-processor/")) {
				t.Errorf("%s imports retired SDS package %s", relative, imported)
			}
		}
		if !configOwner {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.FuncDecl:
				if slices.Contains(retiredDeclarations, node.Name.Name) {
					t.Errorf("%s recreates retired SDS declaration %s", relative, node.Name.Name)
				}
			case *ast.TypeSpec:
				if slices.Contains(retiredDeclarations, node.Name.Name) {
					t.Errorf("%s recreates retired SDS type %s", relative, node.Name.Name)
				}
				if node.Name.Name == "PlatformsConfig" {
					if body, ok := node.Type.(*ast.StructType); ok {
						for _, field := range body.Fields.List {
							for _, name := range field.Names {
								if name.Name == "SDS" {
									t.Errorf("%s retains PlatformsConfig.SDS", relative)
								}
							}
						}
					}
				}
			case *ast.BasicLit:
				if node.Kind == token.STRING {
					value, err := decodeGoImportPath(node.Value)
					if err != nil {
						t.Fatal(err)
					}
					if strings.HasPrefix(value, "platforms.sds.") || strings.HasPrefix(value, "TASK_PROCESSOR_SDS_") {
						t.Errorf("%s retains retired SDS config key %s", relative, value)
					}
				}
			}
			return true
		})
	}
	for _, name := range []string{"config-dev.yaml", "config-prod.yaml", "config-test.yaml"} {
		content, err := os.ReadFile(filepath.Join("..", "config", name))
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]any
		if err := yaml.Unmarshal(content, &document); err != nil {
			t.Fatal(err)
		}
		if value, exists := document["platforms"]; exists {
			platforms, ok := value.(map[string]any)
			if !ok {
				t.Fatalf("%s platforms is not a mapping", name)
			}
			if _, exists := platforms["sds"]; exists {
				t.Errorf("%s retains retired platforms.sds", name)
			}
		}
	}
	for path, retired := range map[string][]string{
		"deployments/kubernetes/listingkit-workbench/base/configmap.yaml":      {"TASK_PROCESSOR_SDS_"},
		"deployments/kubernetes/listingkit-workbench/base/secret.example.yaml": {"TASK_PROCESSOR_SDS_"},
		"hack/debug/README.md":                                  {"go run ./test-sds"},
		"web/listingkit-ui/src/lib/server/upstream-identity.ts": {"AuthenticatedUpstreamRequest"},
		"internal/authz/httpapi/route_authorization.go":         {`"/api/v1/sds-login/health"`, `"sds"`, `"sds-login"`},
		"scripts/start-listingkit-local-ui.ps1":                 {"SDS_API_BASE", "SDS_LOGIN_API_BASE"},
	} {
		content, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range retired {
			if strings.Contains(string(content), value) {
				t.Errorf("%s retains retired SDS policy/config reference %s", path, value)
			}
		}
	}
	for _, command := range []string{"current-application", "shein-login-worker", "image-agent-temporal-worker"} {
		t.Run(command, func(t *testing.T) {
			cmd := exec.Command("go", "list", "-deps", "./cmd/"+command)
			cmd.Dir = ".."
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go list current command: %v\n%s", err, output)
			}
			for _, dependency := range strings.Fields(string(output)) {
				if isRetired(strings.TrimPrefix(dependency, "task-processor/")) {
					t.Errorf("%s still compiles retired SDS dependency %s", command, dependency)
				}
			}
		})
	}
}

func TestLegacyRedisProxyAndSharedClosureIsRetired(t *testing.T) {
	retiredPackages := []string{
		"internal/platform/redis", "internal/shared/hashx", "internal/shared/mathx", "internal/shared/strx",
	}
	retiredArtifacts := []string{
		"internal/platform/database/connection_proxy.go", "internal/platform/database/connection_proxy_test.go",
		"internal/platform/redis/client.go", "internal/platform/redis/client_test.go", "internal/platform/redis/config.go",
		"internal/platform/redis/lock.go", "internal/platform/redis/lock_test.go",
		"internal/platform/redis/memory_lock.go", "internal/platform/redis/memory_lock_test.go", "internal/platform/redis/README.md",
		"internal/app/configadapter/redis.go", "internal/app/configadapter/redis_test.go",
		"internal/shared/hashx/hashutil.go", "internal/shared/mathx/math.go", "internal/shared/mathx/math_test.go",
		"internal/shared/strx/cleaner.go", "internal/shared/strx/join.go", "internal/shared/strx/join_test.go",
		"internal/shared/strx/parse.go", "internal/shared/strx/string.go", "internal/shared/strx/string_test.go", "internal/shared/strx/strings_test.go",
	}
	for _, relative := range append(append([]string{}, retiredPackages...), retiredArtifacts...) {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired Redis/proxy/shared artifact %s must be absent: %v", relative, err)
		}
	}
	for _, relative := range trackedFiles(t, ".") {
		if !strings.HasSuffix(relative, ".go") {
			continue
		}
		var retiredDeclarations []string
		if !strings.HasSuffix(relative, "_test.go") {
			switch {
			case strings.HasPrefix(relative, "internal/platform/database/"):
				retiredDeclarations = []string{"ConnectionProxy", "ConnectionProxyConfig", "ProxyLogger", "NewConnectionProxy"}
			case strings.HasPrefix(relative, "internal/app/configadapter/"):
				retiredDeclarations = []string{"Redis"}
			}
		}
		mode := parser.ImportsOnly
		if len(retiredDeclarations) > 0 {
			mode = 0
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", filepath.FromSlash(relative)), nil, mode)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			imported, err := decodeGoImportPath(spec.Path.Value)
			if err != nil {
				t.Fatalf("decode import in %s: %v", relative, err)
			}
			for _, retired := range retiredPackages {
				if importMatchesPrefix(imported, "task-processor/"+retired) {
					t.Errorf("%s imports retired Redis/shared package %s", relative, imported)
				}
			}
		}
		if len(retiredDeclarations) == 0 {
			continue
		}
		for _, declaration := range file.Decls {
			var names []string
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				if declaration.Recv == nil {
					names = append(names, declaration.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						names = append(names, spec.Name.Name)
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							names = append(names, name.Name)
						}
					}
				}
			}
			for _, name := range names {
				if slices.Contains(retiredDeclarations, name) {
					t.Errorf("%s recreates retired owner declaration %s", relative, name)
				}
			}
		}
	}
	for _, command := range []string{"current-application", "shein-login-worker", "image-agent-temporal-worker"} {
		t.Run(command, func(t *testing.T) {
			cmd := exec.Command("go", "list", "-deps", "./cmd/"+command)
			cmd.Dir = ".."
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go list current command: %v\n%s", err, output)
			}
			for _, dependency := range strings.Fields(string(output)) {
				for _, retired := range retiredPackages {
					if importMatchesPrefix(dependency, "task-processor/"+retired) {
						t.Errorf("%s still compiles retired Redis/shared dependency %s", command, dependency)
					}
				}
			}
		})
	}
}

func TestLegacyHostConfigFeatureFlagTracingClosureIsRetired(t *testing.T) {
	retiredPackages := []string{"internal/platform/config", "internal/platform/featureflag", "internal/platform/observability"}
	for _, relative := range append(append([]string{}, retiredPackages...),
		"internal/core/config/manager.go", "internal/core/config/defaults_applier.go", "internal/core/config/merge.go",
		"internal/platform/config/codec.go", "internal/platform/config/loader.go", "internal/platform/config/path.go",
		"internal/platform/config/source.go", "internal/platform/config/source_test.go", "internal/platform/config/README.md",
		"internal/platform/featureflag/runtime.go", "internal/platform/featureflag/runtime_test.go",
		"internal/platform/observability/config.go", "internal/platform/observability/tracing.go", "internal/platform/observability/tracing_test.go",
	) {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired host config/flag/tracing artifact %s must be absent: %v", relative, err)
		}
	}
	retiredDeclarations := strings.Fields("ConfigManager managerImpl NewConfigManager DefaultsApplier NewDefaultsApplier MergeConfigs MergeMaps FeatureFlagsConfig ObservabilityConfig TracingConfig buildFeatureFlagsConfig")
	for _, relative := range trackedFiles(t, ".") {
		if !strings.HasSuffix(relative, ".go") {
			continue
		}
		configOwner := strings.HasPrefix(relative, "internal/core/config/") && !strings.HasSuffix(relative, "_test.go")
		mode := parser.ImportsOnly
		if configOwner {
			mode = 0
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", filepath.FromSlash(relative)), nil, mode)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			imported, err := decodeGoImportPath(spec.Path.Value)
			if err != nil {
				t.Fatalf("decode import in %s: %v", relative, err)
			}
			for _, retired := range retiredPackages {
				if importMatchesPrefix(imported, "task-processor/"+retired) {
					t.Errorf("%s imports retired host config/flag/tracing package %s", relative, imported)
				}
			}
		}
		if !configOwner {
			continue
		}
		for _, declaration := range file.Decls {
			var names []string
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				if declaration.Recv == nil {
					names = append(names, declaration.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						names = append(names, spec.Name.Name)
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							names = append(names, name.Name)
						}
					}
				}
			}
			for _, name := range names {
				if slices.Contains(retiredDeclarations, name) {
					t.Errorf("%s recreates retired core/config declaration %s", relative, name)
				}
			}
		}
	}
	for _, name := range []string{"config-dev.yaml", "config-test.yaml", "config-prod.yaml"} {
		content, err := os.ReadFile(filepath.Join("..", "config", name))
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]any
		if err := yaml.Unmarshal(content, &document); err != nil {
			t.Fatal(err)
		}
		for _, root := range []string{"featureFlags", "observability"} {
			if _, exists := document[root]; exists {
				t.Errorf("%s retains retired host config root %s", name, root)
			}
		}
	}
	for _, command := range []string{"current-application", "shein-login-worker", "image-agent-temporal-worker"} {
		t.Run(command, func(t *testing.T) {
			cmd := exec.Command("go", "list", "-deps", "./cmd/"+command)
			cmd.Dir = ".."
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go list current command: %v\n%s", err, output)
			}
			for _, dependency := range strings.Fields(string(output)) {
				for _, retired := range retiredPackages {
					if importMatchesPrefix(dependency, "task-processor/"+retired) {
						t.Errorf("%s still compiles retired host config/flag/tracing dependency %s", command, dependency)
					}
				}
			}
		})
	}
}

func TestLegacyWatermarkClosureIsRetired(t *testing.T) {
	const retired = "task-processor/internal/pkg/watermark"
	if _, err := os.Stat(filepath.Join("..", "internal", "pkg", "watermark")); !os.IsNotExist(err) {
		t.Errorf("retired watermark package and examples must be absent: %v", err)
	}
	for _, relative := range trackedFiles(t, ".") {
		if !strings.HasSuffix(relative, ".go") || strings.HasSuffix(relative, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", filepath.FromSlash(relative)), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			imported, err := decodeGoImportPath(spec.Path.Value)
			if err != nil {
				t.Fatalf("decode import in %s: %v", relative, err)
			}
			if importMatchesPrefix(imported, retired) {
				t.Errorf("%s imports retired watermark package %s", relative, imported)
			}
		}
	}
	content, err := os.ReadFile(filepath.Join("..", "config", "config-dev.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	if _, exists := document["watermark"]; exists {
		t.Error("retired watermark sample root must be absent")
	}
	for _, command := range []string{"current-application", "shein-login-worker", "image-agent-temporal-worker"} {
		t.Run(command, func(t *testing.T) {
			cmd := exec.Command("go", "list", "-deps", "./cmd/"+command)
			cmd.Dir = ".."
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go list current command: %v\n%s", err, output)
			}
			for _, dependency := range strings.Fields(string(output)) {
				if importMatchesPrefix(dependency, retired) {
					t.Errorf("%s still compiles retired watermark dependency %s", command, dependency)
				}
			}
		})
	}
}

func TestLegacyQueueProxyAndConfigTailClosureIsRetired(t *testing.T) {
	retiredPackages := []string{
		"internal/platform/queue/rabbitmq", "internal/pkg/cache", "internal/pkg/perf",
		"internal/pkg/types", "internal/pkg/fileio",
	}
	retiredArtifacts := []string{
		"internal/platform/queue/README.md", "scripts/validate-rabbitmq-flow.ps1", "tests/test_config.yaml",
		"scripts/sync-shein-store-proxies.ps1", "deployments/kubernetes/shein-proxy-sync",
		"deployments/kubernetes/shein-listing/base/secret.example.yaml",
		"deployments/kubernetes/shein-listing/overlays/prod-store-shards-example/secret.example.yaml",
		"deployments/kubernetes/shein-listing/overlays/prod/secret.example.yaml",
		"deployments/kubernetes/shein-listing/overlays/prod/external-secret.yaml",
		"deployments/kubernetes/shein-listing/overlays/prod/kustomization.yaml",
		"internal/core/config/type_updater.go",
	}
	for _, relative := range append(retiredPackages, retiredArtifacts...) {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired queue/proxy/config artifact %s must be absent: %v", relative, err)
		}
	}
	for _, root := range []string{"internal", "cmd"} {
		index, err := loadGoFileIndex(filepath.Join("..", root), "")
		if err != nil {
			t.Fatal(err)
		}
		for path, facts := range index.files {
			for literal := range facts.imports {
				imported, err := decodeGoImportPath(literal)
				if err != nil {
					t.Fatalf("decode import in %s: %v", path, err)
				}
				for _, relative := range retiredPackages {
					if importMatchesPrefix(imported, "task-processor/"+relative) {
						t.Errorf("%s imports retired queue/proxy/config package %s", path, imported)
					}
				}
			}
		}
	}
}

func TestOldHostRuntimeClosureStaysRetired(t *testing.T) {
	for _, relative := range retiredHostRuntimePackages {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired host runtime root %s must be absent: %v", relative, err)
		}
	}
	for _, root := range []string{"internal", "cmd"} {
		index, err := loadGoFileIndex(filepath.Join("..", root), "")
		if err != nil {
			t.Fatal(err)
		}
		for path, facts := range index.files {
			for literal := range facts.imports {
				imported, err := decodeGoImportPath(literal)
				if err != nil {
					t.Fatalf("decode import in %s: %v", path, err)
				}
				for _, relative := range retiredHostRuntimePackages {
					if importMatchesPrefix(imported, "task-processor/"+relative) {
						t.Errorf("%s imports retired host runtime %s", path, imported)
					}
				}
			}
		}
	}
}

func TestNumericContextAndValueContractsAreRetired(t *testing.T) {
	assertLegacyLoggerFacadeRetired(t)
	if _, err := os.Lstat(filepath.Join("..", "internal", "platform", "logging", "helpers.go")); !os.IsNotExist(err) {
		t.Errorf("retired logging helper file must be absent: %v", err)
	}
	for _, relative := range []string{"internal/listingruntime/rules.go", "internal/listingruntime/operation_strategy.go"} {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired rule owner %s must be absent: %v", relative, err)
		}
	}
	for relative, retired := range map[string][]string{
		"internal/product/sourcing/source_identity.go": {"func (id SourceIdentity) Key()", "id.SourcePlatform = id.Platform", "id.Platform = id.SourcePlatform", "id.SourceID = id.ProductID", "id.ProductID = id.SourceID"},
	} {
		content, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range retired {
			if strings.Contains(string(content), declaration) {
				t.Errorf("%s retains %q", relative, declaration)
			}
		}
	}
	// The former 5/8-field rules leave with their owners. Their complete
	// absence is enforced by TestListingKitTaskBackendSourcesAreRetired.
}

func TestNumericAcquisitionAndPlatformTailClosureIsRetired(t *testing.T) {
	if _, err := os.Lstat(filepath.Join("..", "internal", "listingruntime")); !os.IsNotExist(err) {
		t.Errorf("retired numeric mapping owner internal/listingruntime must be absent: %v", err)
	}
	for _, relative := range []string{
		"internal/platformbase", "internal/crawler/fetcher", "internal/marketplace/sourceproduct", "internal/domain/task",
		"internal/crawler/amazon/api_service.go", "internal/crawler/amazon/crawler_service.go", "internal/crawler/amazon/worker_processor.go",
		"internal/crawler/amazon/concurrency_control.go", "internal/crawler/amazon/region_guard.go", "internal/crawler/amazon/service_metrics.go",
		"internal/crawler/shared/base_service.go", "internal/crawler/shared/errors.go", "internal/crawler/shared/job_handler.go",
		"internal/crawler/shared/result.go", "internal/crawler/shared/task.go", "internal/crawler/shared/util.go",
		"internal/infra/httpx/crawler_service.go", "internal/infra/httpx/crawler_base_handler.go", "internal/infra/httpx/crawler_amazon_handler.go",
		"internal/shein/client/base_client.go", "internal/shein/api/types.go", "internal/shein/client/api_client.go", "internal/shein/client/cookie_manager.go", "internal/shein/client/runtime_types.go",
		"internal/shein/sherr/cookie_errors.go", "internal/pricing/cost_calculator.go",
		"deployments/docker/Dockerfile.amazon-crawler-api", "deployments/docker/docker-compose.amazon-crawler-api.yml",
		"scripts/push-amazon-crawler-api-dockerhub.ps1", "config/config-amazon-crawler-api.yaml",
		"deployments/kubernetes/amazon-crawler-api", "deployments/kubernetes/amazon-crawler-external-lb",
		"deployments/kubernetes/monitoring/amazon-crawler-api", "deployments/kubernetes/monitoring/alertmanager-wecom/alertmanagerconfig.yaml",
	} {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired numeric acquisition artifact %s must be absent: %v", relative, err)
		}
	}
	for relative, retired := range map[string][]string{
		"internal/shein/authorizedbrand/types.go":     {"ConfigFromStore", "listingruntime"},
		"internal/product/sourcing/source_request.go": {"TenantID"},
		"internal/core/config/type_amazon.go":         {"DataFreshnessDays", "ProductDedupe", "RegionGuard", "ConcurrencyControl", "RemoteAPI"},
		"internal/core/config/type_platform.go":       {"FetchMode"},
	} {
		content, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range retired {
			if strings.Contains(string(content), token) {
				t.Errorf("%s retains retired declaration %q", relative, token)
			}
		}
	}
	for _, relative := range trackedFiles(t, ".") {
		if !strings.HasSuffix(relative, ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", filepath.FromSlash(relative)), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			imported, err := decodeGoImportPath(spec.Path.Value)
			if err != nil {
				t.Fatalf("decode import in %s: %v", relative, err)
			}
			if importMatchesPrefix(imported, "task-processor/internal/listingruntime") {
				t.Errorf("%s imports retired numeric mapping owner %s", relative, imported)
			}
		}
	}
}

func TestLegacyKubernetesReleaseExecutionClosureIsRetired(t *testing.T) {
	for _, relative := range []string{
		"deployments/kubernetes/listingkit-workbench/base/product-listing-api-deployment.yaml",
		"deployments/kubernetes/listingkit-workbench/base/product-listing-api-service.yaml",
		"deployments/kubernetes/listingkit-workbench/jobs/listingkit-schema-migrate-job.yaml",
		"deployments/kubernetes/listingkit-workbench/jobs/product-listing-api-schema-migrate-job.yaml",
		"deployments/kubernetes/listingkit-workbench/jobs/pod-image-index-backfill-job.yaml",
		"scripts/listingkit-schema-migrate-job.sh",
		"scripts/tests/listingkit-schema-migrate-job-test.sh",
		"scripts/listingkit-image-agent-v2-drain-check.sh",
		"scripts/verify-listingkit-api-release-attestation.sh",
		"tests/listingkit_v2_drain_script_test.go",
		"policy/listingkit-release-authority/fixtures/negative/release-identity-owner-drift.yaml",
		"scripts/validate-listingkit-invitation-secret.sh",
		"tests/listingkit_invitation_secret_preflight_test.go",
		"deployments/kubernetes/listingkit-workbench/base/member-invitation-secret.example.yaml",
		"deployments/kubernetes/listingkit-workbench/base/tencent-sms-secret.example.yaml",
		"deployments/kubernetes/listingkit-workbench/release-authority/listingkit-api-release-role.yaml",
		"deployments/kubernetes/listingkit-workbench/release-authority/listingkit-api-release-rolebinding.yaml",
		"deployments/kubernetes/listingkit-workbench/release-authority/listingkit-ui-release-role.yaml",
		"deployments/kubernetes/listingkit-workbench/release-authority/listingkit-ui-release-rolebinding.yaml",
		"deployments/kubernetes/listingkit-workbench/release-authority/kubernetes-authentication-config.example.yaml",
		"scripts/listingkit-configure-github-oidc-kubeconfig.sh",
		"scripts/listingkit-refresh-github-oidc-kubeconfig.sh",
		"scripts/tests/listingkit-github-oidc-refresh-test.sh",
		"policy/listingkit-release-authority/fixtures/negative/excess-rbac-verbs.yaml",
		"policy/listingkit-release-authority/fixtures/negative/excess-rbac-resources.yaml",
		"policy/listingkit-release-authority/fixtures/negative/excess-rbac-resource-names.yaml",
		"policy/listingkit-release-authority/fixtures/negative/shared-environment.yaml",
		"policy/listingkit-release-authority/fixtures/negative/shared-subject.yaml",
		"policy/listingkit-release-authority/fixtures/negative/missing-id-token.yaml",
		"policy/listingkit-release-authority/fixtures/negative/wrong-audience.yaml",
		"policy/listingkit-release-authority/fixtures/negative/workflow-job-owner-drift.yaml",
		"policy/listingkit-release-authority/fixtures/negative/workflow-step-owner-drift.yaml",
		"policy/listingkit-release-authority/fixtures/negative/with-target-bypass.yaml",
		"policy/listingkit-release-authority/fixtures/negative/workflow-ref-feature-branch.yaml",
		"policy/listingkit-release-authority/fixtures/negative/workflow-ref-tag.yaml",
		"policy/listingkit-release-authority/fixtures/negative/workflow-ref-path-prefix-lookalike.yaml",
		"policy/listingkit-release-authority/fixtures/negative/workflow-ref-missing.yaml",
		"policy/listingkit-release-authority/fixtures/negative/trusted-workflow-ref-policy-drift.yaml",
	} {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired Kubernetes release artifact %s must be absent: %v", relative, err)
		}
	}
	// Scan the complete governed source directory: an unregistered or nested
	// manifest must not evade the explicit Conftest input bundle.
	authorityRoot := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "release-authority")
	retainedYAML := map[string]bool{
		"kustomization.yaml":                   true,
		"release-policy.yaml":                  true,
		"listingkit-release-gate-runners.yaml": true,
	}
	if err := filepath.WalkDir(authorityRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if entry.IsDir() || (ext != ".yaml" && ext != ".yml") {
			return nil
		}
		relative, err := filepath.Rel(authorityRoot, path)
		if err != nil {
			return err
		}
		if !retainedYAML[relative] {
			t.Errorf("unexpected release-authority YAML source %s; only the three retained inputs are allowed", relative)
		}
		return nil
	}); err != nil {
		t.Fatalf("scan governed release-authority inventory: %v", err)
	}

	for relative, retired := range map[string][]string{
		"deployments/kubernetes/listingkit-workbench/release-authority/kustomization.yaml": {"listingkit-api-release-role.yaml", "listingkit-api-release-rolebinding.yaml", "listingkit-ui-release-role.yaml", "listingkit-ui-release-rolebinding.yaml", "kubernetes-authentication-config.example.yaml", "listingkit-configure-github-oidc-kubeconfig.sh", "listingkit-refresh-github-oidc-kubeconfig.sh"},
		"scripts/verify-listingkit-release-authority-policy.sh":                            {"listingkit-api-release-role.yaml", "listingkit-api-release-rolebinding.yaml", "listingkit-ui-release-role.yaml", "listingkit-ui-release-rolebinding.yaml", "kubernetes-authentication-config.example.yaml", "listingkit-configure-github-oidc-kubeconfig.sh", "listingkit-refresh-github-oidc-kubeconfig.sh"},
		"deployments/kubernetes/listingkit-workbench/base/kustomization.yaml":              {"product-listing-api-deployment.yaml", "product-listing-api-service.yaml"},
		"deployments/kubernetes/listingkit-workbench/base/configmap.yaml":                  {"http://product-listing-api:8085"},
		"deployments/kubernetes/listingkit-workbench/base/ingress.yaml":                    {"product-listing-api", "/api/v1/listing-kits/integrations/zitadel/sms"},
		"deployments/kubernetes/listingkit-workbench/overlays/prod/patch-ingress.yaml":     {"product-listing-api", "/api/v1/listing-kits/integrations/zitadel/sms"},
		"deployments/kubernetes/listingkit-workbench/base/secret.example.yaml":             {"listingkit-member-invitation-secret", "release-scoped identity preflight"},
	} {
		content, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range retired {
			if strings.Contains(string(content), value) {
				t.Errorf("%s retains retired wiring %q", relative, value)
			}
		}
	}
	content, err := os.ReadFile(filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "release-authority", "release-policy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var bundle map[string]any
	if err := yaml.Unmarshal(content, &bundle); err != nil {
		t.Fatal(err)
	}
	authority, ok := bundle["releaseAuthority"].(map[string]any)
	if !ok {
		t.Fatal("release bundle must contain its machine policy object")
	}
	for _, key := range []string{"producerRouting", "releaseAttestation", "releaseIdentity", "oidc", "rbac", "protectedResources"} {
		if _, exists := authority[key]; exists {
			t.Errorf("retired producer policy %s remains", key)
		}
	}
}

func TestRetiredListingOperationsClosure(t *testing.T) {
	for _, relative := range []string{
		"tools/listingkit-pricing-cache-inspector/main.go",
		"scripts/listingkit-regression-dataset-check.ps1",
		"scripts/listingkit-regression-dataset-check.Tests.ps1",
		"docs/product/validation/listingkit-production-regression-dataset.example.json",
		"deployments/kubernetes/shein-listing/base/configmap.yaml",
		"deployments/kubernetes/shein-listing/overlays/staging/kustomization.yaml",
		"deployments/kubernetes/shein-listing/overlays/prod-single-store-181/kustomization.yaml",
		"deployments/kubernetes/shein-listing/overlays/prod-single-store-976/kustomization.yaml",
		"deployments/kubernetes/shein-listing/overlays/prod-store-shards-gray/kustomization.yaml",
		"deployments/kubernetes/shein-listing/overlays/prod-store-shards-example/kustomization.yaml",
		"deployments/kubernetes/temu-listing/base/configmap.yaml",
		"deployments/kubernetes/temu-listing/base/kustomization.yaml",
		"deployments/kubernetes/temu-listing/base/namespace.yaml",
		"deployments/kubernetes/temu-listing/base/secret.example.yaml",
		"deployments/kubernetes/temu-listing/overlays/prod/external-secret.yaml",
		"deployments/kubernetes/temu-listing/overlays/prod/kustomization.yaml",
		"deployments/kubernetes/temu-listing/overlays/staging/kustomization.yaml",
		"deployments/kubernetes/monitoring/shein-listing/README.md",
		"deployments/kubernetes/monitoring/shein-listing/kustomization.yaml",
		"deployments/kubernetes/monitoring/shein-listing/podmonitor.yaml",
		"deployments/kubernetes/monitoring/shein-listing/podmonitor-store-883.yaml",
		"deployments/kubernetes/monitoring/shein-listing/grafana-dashboard-configmap.yaml",
		"deployments/kubernetes/monitoring/shein-listing/grafana-dashboard-business-configmap.yaml",
		"deployments/kubernetes/monitoring/shein-listing/prometheusrule.yaml",
		"scripts/lib/shein-rollout-batches.ps1",
		"scripts/rollout-shein-shard-statefulset.Tests.ps1",
		"scripts/fix-missing-shein-tasks.sql",
	} {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired listing operations artifact %s must be absent: %v", relative, err)
		}
	}
	readYAML := func(relative string, target any) {
		t.Helper()
		content, err := os.ReadFile(filepath.Join("..", "deployments", "kubernetes", filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		if err := yaml.Unmarshal(content, target); err != nil {
			t.Fatal(err)
		}
	}
	for relative, resources := range map[string][]string{
		"shein-listing/base/kustomization.yaml":            {"namespace.yaml"},
		"monitoring/prometheus-storage/kustomization.yaml": {"prometheusrule.yaml", "alertmanagerconfig.yaml"},
	} {
		var assembly struct {
			Namespace string   `yaml:"namespace"`
			Resources []string `yaml:"resources"`
		}
		readYAML(relative, &assembly)
		if !reflect.DeepEqual(assembly.Resources, resources) {
			t.Errorf("%s resources = %v, want %v", relative, assembly.Resources, resources)
		}
		if strings.Contains(relative, "prometheus-storage") && assembly.Namespace != "monitoring" {
			t.Error("shared storage assembly must use the monitoring namespace")
		}
	}
	for _, relative := range []string{
		"shein-listing/base/namespace.yaml",
	} {
		var resource struct {
			Kind     string                `yaml:"kind"`
			Metadata struct{ Name string } `yaml:"metadata"`
		}
		readYAML(relative, &resource)
		if resource.Kind == "Namespace" && resource.Metadata.Name != "task-processor" {
			t.Error("shared task-processor namespace must remain")
		}
	}
	for relative, tokens := range map[string][]string{
		"deployments/kubernetes/listingkit-workbench/base/shein-login-worker-deployment.yaml": {"shein-login-worker-current", "app: shein-login-worker", "/healthz"},
	} {
		content, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range tokens {
			if !strings.Contains(string(content), token) {
				t.Errorf("%s must retain %q", relative, token)
			}
		}
	}
	var rule struct {
		APIVersion string                           `yaml:"apiVersion"`
		Kind       string                           `yaml:"kind"`
		Metadata   struct{ Name, Namespace string } `yaml:"metadata"`
		Spec       struct {
			Groups []struct {
				Name  string
				Rules []struct {
					Alert, Expr, For string
					Labels           map[string]string
				}
			}
		}
	}
	readYAML("monitoring/prometheus-storage/prometheusrule.yaml", &rule)
	if rule.APIVersion != "monitoring.coreos.com/v1" || rule.Kind != "PrometheusRule" || rule.Metadata.Name != "prometheus-storage" || rule.Metadata.Namespace != "monitoring" {
		t.Fatal("shared storage rule must belong to the monitoring owner")
	}
	if len(rule.Spec.Groups) != 1 || rule.Spec.Groups[0].Name != "prometheus-storage" || len(rule.Spec.Groups[0].Rules) != 1 {
		t.Fatal("shared storage rule must contain only the retained PVC protection")
	}
	alert := rule.Spec.Groups[0].Rules[0]
	const expression = `(\n  kubelet_volume_stats_available_bytes{namespace="monitoring"}\n  /\n  kubelet_volume_stats_capacity_bytes{namespace="monitoring"}\n) < 0.20\nand on (namespace, persistentvolumeclaim)\nkube_persistentvolumeclaim_info{namespace="monitoring",persistentvolumeclaim=~"prometheus-.*"}`
	if strings.TrimSpace(alert.Expr) != strings.ReplaceAll(expression, `\n`, "\n") || alert.For != "15m" || alert.Alert != "PrometheusStorageAtRisk" || !reflect.DeepEqual(alert.Labels, map[string]string{"namespace": "monitoring", "severity": "warning", "service": "monitoring"}) {
		t.Error("retained storage expression, timing and scope must remain intact")
	}
	type alertmanagerConfig struct {
		APIVersion string                           `yaml:"apiVersion"`
		Kind       string                           `yaml:"kind"`
		Metadata   struct{ Name, Namespace string } `yaml:"metadata"`
		Spec       struct {
			Route     map[string]any `yaml:"route"`
			Receivers []any          `yaml:"receivers"`
		}
	}
	var shared alertmanagerConfig
	readYAML("monitoring/prometheus-storage/alertmanagerconfig.yaml", &shared)
	if shared.APIVersion != "monitoring.coreos.com/v1alpha1" || shared.Kind != "AlertmanagerConfig" || shared.Metadata.Name != "prometheus-storage-wecom" || shared.Metadata.Namespace != alert.Labels["namespace"] {
		t.Error("storage route must share the rule namespace under OnNamespace enforcement")
	}
	wantRoute := map[string]any{
		"groupBy":   []any{"alertname", "service", "severity"},
		"groupWait": "30s", "groupInterval": "5m", "repeatInterval": "2h", "receiver": "wecom-webhook",
		"matchers": []any{
			map[string]any{"name": "service", "value": alert.Labels["service"], "matchType": "="},
			map[string]any{"name": "alertname", "value": alert.Alert, "matchType": "="},
		},
	}
	wantReceivers := []any{map[string]any{"name": "wecom-webhook", "webhookConfigs": []any{
		map[string]any{"url": "http://alertmanager-wecom.monitoring.svc.cluster.local:8080/webhook", "sendResolved": true, "maxAlerts": 10, "timeout": "10s"},
	}}}
	if !reflect.DeepEqual(shared.Spec.Route, wantRoute) || !reflect.DeepEqual(shared.Spec.Receivers, wantReceivers) {
		t.Error("storage route must match only its alert and retain grouping/receiver settings")
	}
	var stack map[string]any
	readYAML("monitoring/kube-prometheus-stack-values.yaml", &stack)
	alertmanager := stack["alertmanager"].(map[string]any)["alertmanagerSpec"].(map[string]any)
	for _, key := range []string{"alertmanagerConfigSelector", "alertmanagerConfigNamespaceSelector", "alertmanagerConfigMatcherStrategy"} {
		if _, changed := alertmanager[key]; changed {
			t.Errorf("%s must keep the fixed chart's existing selection/namespace default", key)
		}
	}
}

func TestRetiredTaskAndListingRuntimeConfigClosure(t *testing.T) {
	for _, relative := range []string{
		"internal/core/config/type_listing_control_plane.go", "internal/core/config/type_processor.go",
		"internal/core/config/type_worker.go", "internal/core/config/validator_worker.go",
		"internal/core/config/loader_rabbitmq.go", "internal/core/config/validator_rabbitmq.go",
		"internal/core/config/platform_config_registry.go", "internal/app/configadapter/workerpool.go",
		"internal/app/configadapter/queue.go", "internal/core/config/processor_config_test.go",
		"internal/core/config/platform_legacy_sync_test.go", "internal/app/configadapter/workerpool_test.go",
		"internal/app/configadapter/queue_test.go",
	} {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired config artifact %s must be absent: %v", relative, err)
		}
	}
	retiredRoots := []string{"processor", "worker", "listingControlPlane", "rabbitmq", "updater"}
	retiredPlatform := []string{"enabled", "schedulerEnabled", "autoPricing", "listingPricing", "productSync", "inventorySync", "activityRegistration", "sync", "monitor", "configPaths", "fetchMode"}
	for _, relative := range []string{
		"config/config-dev.yaml", "config/config-prod.yaml", "config/config-test.yaml",
	} {
		t.Run(relative, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(relative)))
			if err != nil {
				t.Fatal(err)
			}
			var document map[string]any
			if err := yaml.Unmarshal(content, &document); err != nil {
				t.Fatal(err)
			}
			configs := []map[string]any{document}
			if document["kind"] == "ConfigMap" {
				configs = nil
				data, ok := document["data"].(map[string]any)
				if !ok || len(data) == 0 {
					t.Fatal("missing ConfigMap data")
				}
				for key, value := range data {
					if !strings.HasSuffix(key, ".yaml") {
						continue
					}
					text, ok := value.(string)
					if !ok {
						t.Fatalf("embedded config %s must be text", key)
					}
					var embedded map[string]any
					if err := yaml.Unmarshal([]byte(text), &embedded); err != nil {
						t.Fatal(err)
					}
					configs = append(configs, embedded)
				}
				if len(configs) == 0 {
					t.Fatal("missing embedded config YAML")
				}
			}
			for _, cfg := range configs {
				if amazon, ok := cfg["amazon"].(map[string]any); ok {
					for _, key := range []string{"dataFreshnessDays", "productDedupe", "regionGuard", "concurrencyControl", "remoteAPI"} {
						if _, exists := amazon[key]; exists {
							t.Errorf("retired Amazon config %s remains", key)
						}
					}
				}
				for _, key := range retiredRoots {
					if _, exists := cfg[key]; exists {
						t.Errorf("retired root %s remains", key)
					}
				}
				if debug, ok := cfg["debug"].(map[string]any); ok {
					if _, exists := debug["save_publish_json"]; exists {
						t.Error("retired debug input remains")
					}
				}
				if listing, ok := cfg["listingkit"].(map[string]any); ok {
					for _, key := range []string{"sheinSubmitDebugDumpDir", "generationUsageLedgerEnabled", "generationUsageLedgerTenantIDs"} {
						if _, exists := listing[key]; exists {
							t.Errorf("retired listingkit.%s remains", key)
						}
					}
				}
				platforms, hasPlatforms := cfg["platforms"].(map[string]any)
				if strings.Contains(relative, "amazon-crawler-api") && !hasPlatforms {
					t.Fatal("current Alibaba1688 platform configuration must remain")
				}
				if hasPlatforms {
					for _, name := range []string{"temu", "shein"} {
						if platform, ok := platforms[name].(map[string]any); ok {
							for _, key := range retiredPlatform {
								if _, exists := platform[key]; exists {
									t.Errorf("retired platforms.%s.%s remains", name, key)
								}
							}
						}
					}
					if strings.Contains(relative, "amazon-crawler-api") {
						alibaba, ok := platforms["alibaba1688"].(map[string]any)
						if !ok || alibaba["enabled"] != false || alibaba["timeout"] != 120 || alibaba["poolSize"] != 2 {
							t.Error("current Alibaba1688 crawler config must remain")
						}
					}
				}
			}
		})
	}
	for _, relative := range []string{
		"listingkit-workbench/base/secret.example.yaml",
	} {
		content, err := os.ReadFile(filepath.Join("..", "deployments", "kubernetes", filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), "TASK_PROCESSOR_RABBITMQ_URL") {
			t.Errorf("%s declares retired queue input", relative)
		}
	}
}

var retiredNumericListingSupportFiles = []string{
	"internal/listing/submission/action_record.go",
	"internal/listing/submission/action_record_test.go",
	"internal/listing/submission/attempt_finalize.go",
	"internal/listing/submission/attempt_finalize_test.go",
	"internal/listing/submission/attempt_record.go",
	"internal/listing/submission/attempt_record_test.go",
	"internal/listing/submission/confirm_remote_state.go",
	"internal/listing/submission/confirm_remote_state_test.go",
	"internal/listing/submission/direct_submit_service.go",
	"internal/listing/submission/direct_submit_service_test.go",
	"internal/listing/submission/enqueue_retry.go",
	"internal/listing/submission/enqueue_retry_test.go",
	"internal/listing/submission/event_draft.go",
	"internal/listing/submission/event_draft_test.go",
	"internal/listing/submission/event_history.go",
	"internal/listing/submission/event_history_test.go",
	"internal/listing/submission/event_outcome.go",
	"internal/listing/submission/event_outcome_test.go",
	"internal/listing/submission/failure_persistence_service.go",
	"internal/listing/submission/failure_persistence_service_test.go",
	"internal/listing/submission/failure_state.go",
	"internal/listing/submission/failure_state_test.go",
	"internal/listing/submission/inflight_state.go",
	"internal/listing/submission/inflight_state_test.go",
	"internal/listing/submission/lease_acquire_service.go",
	"internal/listing/submission/lease_acquire_service_test.go",
	"internal/listing/submission/payload_stage_service.go",
	"internal/listing/submission/payload_stage_service_test.go",
	"internal/listing/submission/phase_detail.go",
	"internal/listing/submission/phase_detail_test.go",
	"internal/listing/submission/phase_event.go",
	"internal/listing/submission/phase_event_test.go",
	"internal/listing/submission/projection.go",
	"internal/listing/submission/projection_test.go",
	"internal/listing/submission/readiness_gate.go",
	"internal/listing/submission/readiness_gate_test.go",
	"internal/listing/submission/readiness_projection.go",
	"internal/listing/submission/readiness_projection_test.go",
	"internal/listing/submission/record_draft.go",
	"internal/listing/submission/record_draft_test.go",
	"internal/listing/submission/recovered_submit_persistence.go",
	"internal/listing/submission/recovered_submit_persistence_test.go",
	"internal/listing/submission/recovery_batch_service.go",
	"internal/listing/submission/recovery_batch_service_test.go",
	"internal/listing/submission/recovery_lease.go",
	"internal/listing/submission/recovery_lease_test.go",
	"internal/listing/submission/recovery_now_service.go",
	"internal/listing/submission/recovery_now_service_test.go",
	"internal/listing/submission/recovery_route_service.go",
	"internal/listing/submission/recovery_route_service_test.go",
	"internal/listing/submission/refresh_guard.go",
	"internal/listing/submission/refresh_guard_test.go",
	"internal/listing/submission/refresh_request.go",
	"internal/listing/submission/refresh_request_test.go",
	"internal/listing/submission/refresh_selection.go",
	"internal/listing/submission/refresh_selection_test.go",
	"internal/listing/submission/refresh_service.go",
	"internal/listing/submission/refresh_service_test.go",
	"internal/listing/submission/remote_completion_state.go",
	"internal/listing/submission/remote_completion_state_test.go",
	"internal/listing/submission/remote_record_id.go",
	"internal/listing/submission/remote_record_id_test.go",
	"internal/listing/submission/remote_refresh_service.go",
	"internal/listing/submission/remote_refresh_service_test.go",
	"internal/listing/submission/remote_refresh_state.go",
	"internal/listing/submission/remote_refresh_state_test.go",
	"internal/listing/submission/remote_submit_service.go",
	"internal/listing/submission/remote_submit_service_test.go",
	"internal/listing/submission/remote_sync.go",
	"internal/listing/submission/remote_sync_test.go",
	"internal/listing/submission/requeue_service.go",
	"internal/listing/submission/requeue_service_test.go",
	"internal/listing/submission/response_outcome.go",
	"internal/listing/submission/response_outcome_test.go",
	"internal/listing/submission/result_persistence_service.go",
	"internal/listing/submission/result_persistence_service_test.go",
	"internal/listing/submission/result_state.go",
	"internal/listing/submission/result_state_test.go",
	"internal/listing/submission/retryable_backfill_service.go",
	"internal/listing/submission/retryable_backfill_service_test.go",
	"internal/listing/submission/retryable_block.go",
	"internal/listing/submission/retryable_block_test.go",
	"internal/listing/submission/source_facts.go",
	"internal/listing/submission/source_facts_test.go",
	"internal/listing/submission/start_failure_service.go",
	"internal/listing/submission/start_failure_service_test.go",
	"internal/listing/submission/submit_action.go",
	"internal/listing/submission/submit_action_test.go",
	"internal/listing/submission/submit_attempt.go",
	"internal/listing/submission/submit_attempt_plan.go",
	"internal/listing/submission/submit_attempt_plan_test.go",
	"internal/listing/submission/submit_attempt_test.go",
	"internal/listing/submission/submit_error.go",
	"internal/listing/submission/submit_error_test.go",
	"internal/listing/submission/submit_lock.go",
	"internal/listing/submission/submit_lock_test.go",
	"internal/listing/submission/submit_request_id.go",
	"internal/listing/submission/submit_request_id_test.go",
	"internal/listing/submission/submit_target.go",
	"internal/listing/submission/submit_target_test.go",
	"internal/listing/submission/success_persistence_service.go",
	"internal/listing/submission/success_persistence_service_test.go",
	"internal/marketplace/shein/publishing/phase_detail_policy.go",
	"internal/marketplace/shein/publishing/phase_detail_policy_test.go",
	"internal/marketplace/shein/publishing/projection_policy.go",
	"internal/marketplace/shein/publishing/projection_policy_test.go",
	"internal/marketplace/shein/publishing/remote_record_policy.go",
	"internal/marketplace/shein/publishing/remote_record_policy_test.go",
	"internal/marketplace/shein/publishing/sensitive_retry_policy.go",
	"internal/marketplace/shein/publishing/sensitive_retry_policy_test.go",
	"internal/publishing/shein/api_builders.go",
	"internal/publishing/shein/attribute_cache_freshness.go",
	"internal/publishing/shein/attribute_resolver.go",
	"internal/publishing/shein/attribute_resolver_test.go",
	"internal/publishing/shein/attribute_value_matcher.go",
	"internal/publishing/shein/attribute_value_matcher_test.go",
	"internal/publishing/shein/cache_identity.go",
	"internal/publishing/shein/category_ai_contract.go",
	"internal/publishing/shein/category_legacy_selector_adapter.go",
	"internal/publishing/shein/category_path_helpers.go",
	"internal/publishing/shein/category_query.go",
	"internal/publishing/shein/category_reselection_signals.go",
	"internal/publishing/shein/category_resolver.go",
	"internal/publishing/shein/category_resolver_ai.go",
	"internal/publishing/shein/category_resolver_test.go",
	"internal/publishing/shein/category_review_signals.go",
	"internal/publishing/shein/category_review_signals_test.go",
	"internal/publishing/shein/category_semantic_verifier.go",
	"internal/publishing/shein/category_semantic_verifier_test.go",
	"internal/publishing/shein/category_suggest_fallback.go",
	"internal/publishing/shein/category_tree_fallback.go",
	"internal/publishing/shein/derived_refresh.go",
	"internal/publishing/shein/derived_refresh_test.go",
	"internal/publishing/shein/display_attribute_batch_inference.go",
	"internal/publishing/shein/display_attribute_candidate_set.go",
	"internal/publishing/shein/display_attribute_candidate_set_test.go",
	"internal/publishing/shein/display_attribute_dependency.go",
	"internal/publishing/shein/display_attribute_evidence.go",
	"internal/publishing/shein/display_attribute_evidence_test.go",
	"internal/publishing/shein/display_attribute_field_selector.go",
	"internal/publishing/shein/display_attribute_inputs.go",
	"internal/publishing/shein/display_attribute_missing_inference.go",
	"internal/publishing/shein/display_attribute_prompt.go",
	"internal/publishing/shein/display_attribute_required_repair.go",
	"internal/publishing/shein/display_attribute_required_repair_test.go",
	"internal/publishing/shein/display_attribute_resolution_flow.go",
	"internal/publishing/shein/display_attribute_resolution_flow_test.go",
	"internal/publishing/shein/display_attribute_static_matcher.go",
	"internal/publishing/shein/display_attribute_static_matcher_test.go",
	"internal/publishing/shein/display_attribute_template.go",
	"internal/publishing/shein/display_attribute_template_test.go",
	"internal/publishing/shein/final_draft_images.go",
	"internal/publishing/shein/final_draft_images_test.go",
	"internal/publishing/shein/final_draft_submit.go",
	"internal/publishing/shein/final_draft_submit_test.go",
	"internal/publishing/shein/generation_topics.go",
	"internal/publishing/shein/generation_topics_test.go",
	"internal/publishing/shein/pricing_cache_identity.go",
	"internal/publishing/shein/pricing_cache_identity_test.go",
	"internal/publishing/shein/pricing_cache_reconcile.go",
	"internal/publishing/shein/pricing_cache_reconcile_test.go",
	"internal/publishing/shein/resolution_cache_store_test.go",
	"internal/publishing/shein/resolver_cache.go",
	"internal/publishing/shein/resolver_cache_codec.go",
	"internal/publishing/shein/resolver_cache_key.go",
	"internal/publishing/shein/resolver_cache_test.go",
	"internal/publishing/shein/review_content.go",
	"internal/publishing/shein/review_content_test.go",
	"internal/publishing/shein/runtime_api_factory.go",
	"internal/publishing/shein/runtime_attribute_resolver.go",
	"internal/publishing/shein/runtime_base_client.go",
	"internal/publishing/shein/runtime_category_resolver.go",
	"internal/publishing/shein/runtime_image_api.go",
	"internal/publishing/shein/runtime_product_api.go",
	"internal/publishing/shein/runtime_resolver_fail_closed_test.go",
	"internal/publishing/shein/runtime_sale_attribute_resolver.go",
	"internal/publishing/shein/runtime_translate_api.go",
	"internal/publishing/shein/sale_attribute_ai.go",
	"internal/publishing/shein/sale_attribute_cache_reconcile.go",
	"internal/publishing/shein/sale_attribute_cache_reconcile_test.go",
	"internal/publishing/shein/sale_attribute_candidates.go",
	"internal/publishing/shein/sale_attribute_mapping_llm.go",
	"internal/publishing/shein/sale_attribute_prompt.go",
	"internal/publishing/shein/sale_attribute_resolution_result.go",
	"internal/publishing/shein/sale_attribute_resolver.go",
	"internal/publishing/shein/sale_attribute_resolver_test.go",
	"internal/publishing/shein/sale_attribute_selection_rules.go",
	"internal/publishing/shein/sale_attribute_template_rules.go",
	"internal/publishing/shein/sale_attribute_value_helpers.go",
	"internal/publishing/shein/sale_attribute_value_llm.go",
	"internal/publishing/shein/sale_attribute_value_resolution.go",
	"internal/publishing/shein/sale_attribute_value_resolution_test.go",
	"internal/publishing/shein/sale_attribute_value_segments.go",
	"internal/publishing/shein/sds_images.go",
	"internal/publishing/shein/sds_images_test.go",
	"internal/publishing/shein/size_attribute_cache_reconcile.go",
	"internal/publishing/shein/size_attribute_cache_reconcile_test.go",
	"internal/publishing/shein/size_attribute_header_resolver.go",
	"internal/publishing/shein/size_attribute_header_resolver_test.go",
	"internal/publishing/shein/source_dimension_heuristics.go",
	"internal/publishing/shein/source_dimension_selection.go",
	"internal/publishing/shein/source_dimension_selection_test.go",
	"internal/publishing/shein/studio_ai_images.go",
	"internal/publishing/shein/studio_ai_images_test.go",
	"internal/publishing/shein/submission.go",
	"internal/publishing/shein/submission_persistence_input.go",
	"internal/publishing/shein/submission_persistence_input_test.go",
	"internal/publishing/shein/submission_projection.go",
	"internal/publishing/shein/submission_remote.go",
	"internal/publishing/shein/submission_remote_test.go",
	"internal/publishing/shein/submission_state.go",
	"internal/publishing/shein/submission_state_test.go",
	"internal/publishing/shein/submission_test.go",
	"internal/publishing/shein/submit_image_upload.go",
	"internal/publishing/shein/submit_image_upload_cache.go",
	"internal/publishing/shein/submit_image_upload_cache_test.go",
	"internal/publishing/shein/submit_image_upload_test.go",
	"internal/publishing/shein/submit_localization.go",
	"internal/publishing/shein/submit_payload_prepare_test.go",
	"internal/publishing/shein/submit_prep.go",
	"internal/publishing/shein/submit_readiness_action_policy.go",
	"internal/publishing/shein/submit_remote_action.go",
	"internal/publishing/shein/submit_remote_action_test.go",
	"internal/publishing/shein/submit_sensitive_retry.go",
	"internal/publishing/shein/submit_sensitive_retry_test.go",
	"internal/publishing/shein/submit_validation.go",
	"internal/publishing/shein/template_matcher_test.go",
	"internal/publishing/shein/text_normalization.go",
	"internal/publishing/shein/text_normalization_test.go",
	"internal/publishing/shein/variant_image_sets.go",
	"internal/publishing/shein/variant_image_sets_test.go",
	"internal/publishing/sheinmanaged/api_builders.go",
	"internal/publishing/sheinmanaged/api_factory.go",
	"internal/publishing/sheinmanaged/attribute_api_factory.go",
	"internal/publishing/sheinmanaged/attribute_resolver.go",
	"internal/publishing/sheinmanaged/bridge.go",
	"internal/publishing/sheinmanaged/bridge_test.go",
	"internal/publishing/sheinmanaged/category_api_factory.go",
	"internal/publishing/sheinmanaged/category_resolver.go",
	"internal/publishing/sheinmanaged/category_selector_adapter.go",
	"internal/publishing/sheinmanaged/sale_attribute_resolver.go",
	"internal/shein/aicache/cache.go",
	"internal/shein/category/manager.go",
	"internal/shein/category/manager_test.go",
	"internal/shein/content/optimizer.go",
	"internal/shein/content/string_sanitizer.go",
	"internal/shein/content/string_sanitizer_test.go",
	"internal/shein/content/text_cleaner.go",
	"internal/shein/content/text_cleaner_test.go",
	"internal/shein/submitprep/language.go",
	"internal/shein/submitprep/language_test.go",
	"internal/shein/submitprep/localized_content.go",
	"internal/shein/submitprep/localized_content_test.go",
	"internal/shein/submitprep/localized_list.go",
	"internal/shein/submitprep/localized_list_test.go",
	"prompts/shein/category_selector.yaml",
	"prompts/shein/content_optimizer.yaml",
	"prompts/shein/display_attribute.yaml",
	"prompts/shein/sale_attribute.yaml",

	"internal/app/worker/job_handler.go",
	"internal/core/metrics/shein_metrics.go",
	"internal/core/metrics/task_metrics.go",
	"internal/listingadmin/category.go",
	"internal/listingadmin/category_model.go",
	"internal/listingadmin/category_model_test.go",
	"internal/listingadmin/category_query.go",
	"internal/listingadmin/category_query_test.go",
	"internal/listingadmin/category_repository.go",
	"internal/listingadmin/dto_value_helpers.go",
	"internal/listingadmin/filter_rule.go",
	"internal/listingadmin/filter_rule_model.go",
	"internal/listingadmin/filter_rule_model_test.go",
	"internal/listingadmin/filter_rule_query.go",
	"internal/listingadmin/filter_rule_query_test.go",
	"internal/listingadmin/filter_rule_repository.go",
	"internal/listingadmin/gorm_filter_rule_api.go",
	"internal/listingadmin/gorm_operation_strategy_api.go",
	"internal/listingadmin/gorm_product_data_api.go",
	"internal/listingadmin/gorm_product_data_api_test.go",
	"internal/listingadmin/gorm_product_import_mapping_api.go",
	"internal/listingadmin/gorm_product_import_mapping_api_test.go",
	"internal/listingadmin/gorm_raw_json_data_api.go",
	"internal/listingadmin/import_task_owner.go",
	"internal/listingadmin/import_task_owner_test.go",
	"internal/listingadmin/numeric_helpers.go",
	"internal/listingadmin/operation_strategy.go",
	"internal/listingadmin/operation_strategy_model.go",
	"internal/listingadmin/operation_strategy_model_test.go",
	"internal/listingadmin/operation_strategy_query.go",
	"internal/listingadmin/operation_strategy_query_test.go",
	"internal/listingadmin/operation_strategy_repository.go",
	"internal/listingadmin/owner_scoped_batch_test.go",
	"internal/listingadmin/owner_test_helpers_test.go",
	"internal/listingadmin/pricing_rule.go",
	"internal/listingadmin/pricing_rule_model.go",
	"internal/listingadmin/pricing_rule_model_test.go",
	"internal/listingadmin/pricing_rule_query.go",
	"internal/listingadmin/pricing_rule_query_test.go",
	"internal/listingadmin/pricing_rule_repository.go",
	"internal/listingadmin/product_data.go",
	"internal/listingadmin/product_data_model.go",
	"internal/listingadmin/product_data_model_test.go",
	"internal/listingadmin/product_data_query.go",
	"internal/listingadmin/product_data_query_test.go",
	"internal/listingadmin/product_data_repository.go",
	"internal/listingadmin/product_import_mapping.go",
	"internal/listingadmin/product_import_mapping_model.go",
	"internal/listingadmin/product_import_mapping_model_test.go",
	"internal/listingadmin/product_import_mapping_query.go",
	"internal/listingadmin/product_import_mapping_query_test.go",
	"internal/listingadmin/product_import_mapping_repository.go",
	"internal/listingadmin/profit_rule.go",
	"internal/listingadmin/profit_rule_model.go",
	"internal/listingadmin/profit_rule_model_test.go",
	"internal/listingadmin/profit_rule_query.go",
	"internal/listingadmin/profit_rule_query_test.go",
	"internal/listingadmin/profit_rule_repository.go",
	"internal/listingadmin/raw_json_data.go",
	"internal/listingadmin/raw_json_data_query_test.go",
	"internal/listingadmin/raw_json_data_repository.go",
	"internal/listingadmin/rule_status.go",
	"internal/marketplace/shein/workspace/category_search.go",
	"internal/marketplace/shein/workspace/category_search_test.go",
	"internal/marketplace/shein/workspace/display_title_policy.go",
	"internal/marketplace/shein/workspace/display_title_policy_test.go",
	"internal/marketplace/shein/workspace/editor_context_builder.go",
	"internal/marketplace/shein/workspace/editor_context_builder_test.go",
	"internal/marketplace/shein/workspace/editor_context_model.go",
	"internal/marketplace/shein/workspace/editor_dirty_hints.go",
	"internal/marketplace/shein/workspace/editor_effects.go",
	"internal/marketplace/shein/workspace/editor_effects_test.go",
	"internal/marketplace/shein/workspace/editor_model.go",
	"internal/marketplace/shein/workspace/editor_progress.go",
	"internal/marketplace/shein/workspace/editor_progress_test.go",
	"internal/marketplace/shein/workspace/editor_recommendations.go",
	"internal/marketplace/shein/workspace/editor_recommendations_test.go",
	"internal/marketplace/shein/workspace/editor_revision_from_context.go",
	"internal/marketplace/shein/workspace/editor_revision_from_context_test.go",
	"internal/marketplace/shein/workspace/editor_revision_minimal.go",
	"internal/marketplace/shein/workspace/editor_revision_minimal_test.go",
	"internal/marketplace/shein/workspace/editor_revision_model.go",
	"internal/marketplace/shein/workspace/editor_revision_skeleton.go",
	"internal/marketplace/shein/workspace/history_detail.go",
	"internal/marketplace/shein/workspace/history_detail_test.go",
	"internal/marketplace/shein/workspace/inspection.go",
	"internal/marketplace/shein/workspace/inspection_test.go",
	"internal/marketplace/shein/workspace/overview.go",
	"internal/marketplace/shein/workspace/overview_test.go",
	"internal/marketplace/shein/workspace/preview_card.go",
	"internal/marketplace/shein/workspace/preview_card_test.go",
	"internal/marketplace/shein/workspace/preview_projection.go",
	"internal/marketplace/shein/workspace/preview_projection_test.go",
	"internal/marketplace/shein/workspace/preview_support.go",
	"internal/marketplace/shein/workspace/preview_support_test.go",
	"internal/marketplace/shein/workspace/readiness_guidance.go",
	"internal/marketplace/shein/workspace/readiness_guidance_test.go",
	"internal/marketplace/shein/workspace/repair.go",
	"internal/marketplace/shein/workspace/repair_center_from_readiness.go",
	"internal/marketplace/shein/workspace/repair_center_from_readiness_test.go",
	"internal/marketplace/shein/workspace/repair_revision.go",
	"internal/marketplace/shein/workspace/repair_revision_test.go",
	"internal/marketplace/shein/workspace/repair_test.go",
	"internal/marketplace/shein/workspace/review_state.go",
	"internal/marketplace/shein/workspace/review_state_test.go",
	"internal/marketplace/shein/workspace/revision_apply_changes.go",
	"internal/marketplace/shein/workspace/revision_apply_patch.go",
	"internal/marketplace/shein/workspace/revision_apply_patch_test.go",
	"internal/marketplace/shein/workspace/revision_diff.go",
	"internal/marketplace/shein/workspace/revision_diff_model.go",
	"internal/marketplace/shein/workspace/revision_diff_test.go",
	"internal/marketplace/shein/workspace/revision_field_validation.go",
	"internal/marketplace/shein/workspace/revision_field_validation_test.go",
	"internal/marketplace/shein/workspace/revision_history_compare.go",
	"internal/marketplace/shein/workspace/revision_history_compare_test.go",
	"internal/marketplace/shein/workspace/revision_history_detail_builder.go",
	"internal/marketplace/shein/workspace/revision_history_restore_detail.go",
	"internal/marketplace/shein/workspace/revision_history_restore_detail_test.go",
	"internal/marketplace/shein/workspace/revision_restore_draft.go",
	"internal/marketplace/shein/workspace/revision_restore_draft_test.go",
	"internal/marketplace/shein/workspace/revision_restore_presentation.go",
	"internal/marketplace/shein/workspace/revision_restore_preview.go",
	"internal/marketplace/shein/workspace/revision_restore_preview_builder.go",
	"internal/marketplace/shein/workspace/revision_restore_preview_test.go",
	"internal/marketplace/shein/workspace/revision_restore_request.go",
	"internal/marketplace/shein/workspace/revision_success.go",
	"internal/marketplace/shein/workspace/revision_success_model.go",
	"internal/marketplace/shein/workspace/revision_success_test.go",
	"internal/marketplace/shein/workspace/revision_validation.go",
	"internal/marketplace/shein/workspace/revision_validation_test.go",
	"internal/marketplace/shein/workspace/source_product.go",
	"internal/marketplace/shein/workspace/source_product_test.go",
	"internal/marketplace/shein/workspace/store_resolution.go",
	"internal/marketplace/shein/workspace/store_resolution_test.go",
	"internal/marketplace/shein/workspace/submit_freshness.go",
	"internal/marketplace/shein/workspace/submit_freshness_readiness.go",
	"internal/marketplace/shein/workspace/submit_freshness_readiness_test.go",
	"internal/marketplace/shein/workspace/submit_freshness_test.go",
	"internal/marketplace/shein/workspace/task_queue.go",
	"internal/marketplace/shein/workspace/task_queue_test.go",
	"internal/marketplace/shein/workspace/task_taxonomy.go",
	"internal/marketplace/shein/workspace/task_taxonomy_test.go",
	"internal/model/task.go",
	"internal/model/task_retry.go",
	"internal/model/task_retry_test.go",
	"internal/model/task_status.go",
	"internal/model/task_status_test.go",
	"internal/model/task_test.go",
	"internal/shein/sherr/handled_errors.go",
	"internal/taskrpcapi/build.go",
	"internal/taskrpcapi/build_test.go",
	"internal/taskrpcapi/handler.go",
	"internal/taskrpcapi/http_module.go",
	"internal/taskrpcapi/http_module_test.go",
	"internal/taskrpcapi/types.go",
	"internal/workspace/shein/compatibility_guard_test.go",
	"internal/workspace/shein/doc.go",
	"internal/workspace/shein/editor_context_builder.go",
	"internal/workspace/shein/editor_context_builder_test.go",
	"internal/workspace/shein/editor_context_model.go",
	"internal/workspace/shein/editor_dirty_hints.go",
	"internal/workspace/shein/editor_effects.go",
	"internal/workspace/shein/editor_model.go",
	"internal/workspace/shein/editor_progress.go",
	"internal/workspace/shein/editor_recommendations.go",
	"internal/workspace/shein/editor_revision_from_context.go",
	"internal/workspace/shein/editor_revision_minimal.go",
	"internal/workspace/shein/editor_revision_minimal_test.go",
	"internal/workspace/shein/editor_revision_model.go",
	"internal/workspace/shein/editor_revision_skeleton.go",
	"internal/workspace/shein/history_detail.go",
	"internal/workspace/shein/inspection.go",
	"internal/workspace/shein/inspection_payloads.go",
	"internal/workspace/shein/inspection_test.go",
	"internal/workspace/shein/overview.go",
	"internal/workspace/shein/overview_test.go",
	"internal/workspace/shein/readiness.go",
	"internal/workspace/shein/readiness_guidance.go",
	"internal/workspace/shein/readiness_guidance_test.go",
	"internal/workspace/shein/readiness_test.go",
	"internal/workspace/shein/repair.go",
	"internal/workspace/shein/repair_test.go",
	"internal/workspace/shein/revision_apply_changes.go",
	"internal/workspace/shein/revision_diff.go",
	"internal/workspace/shein/revision_diff_model.go",
	"internal/workspace/shein/revision_field_validation.go",
	"internal/workspace/shein/revision_field_validation_test.go",
	"internal/workspace/shein/revision_history_compare.go",
	"internal/workspace/shein/revision_history_detail_builder.go",
	"internal/workspace/shein/revision_history_restore_detail.go",
	"internal/workspace/shein/revision_restore_draft.go",
	"internal/workspace/shein/revision_restore_presentation.go",
	"internal/workspace/shein/revision_restore_preview.go",
	"internal/workspace/shein/revision_restore_preview_builder.go",
	"internal/workspace/shein/revision_restore_request.go",
	"internal/workspace/shein/revision_success.go",
	"internal/workspace/shein/revision_success_model.go",
	"internal/workspace/shein/revision_success_test.go",
	"internal/workspace/shein/revision_validation.go",
	"internal/workspace/shein/state.go",
}

var retiredNumericListingSupportDeclarations = map[string][]string{
	"internal/publishing/shein": strings.Fields(`
Package.RequestDraft Package.PreviewProduct Package.FinalDraft Package.MarshalJSON Package.UnmarshalJSON NormalizePackageSemanticFields conflictingPersistedAliases equalPersistedAliases
AttributeAPI BuildRequest.SheinStoreID BuildRequest.Text CategoryAPI CleanSubmitProductSensitiveWords CloneImageUploadCache
CloneResolutionCacheInfo ConfigureSubmitPrepRepositories DistinctSKCMainImageCount EnforceVariantImageCoverage EnsureDraftPayload GormResolutionCacheStore
GormResolutionCacheStore.DeleteResolutionCache GormResolutionCacheStore.DeleteResolutionCacheByShortKey GormResolutionCacheStore.GetManualResolutionCacheByProductIdentity GormResolutionCacheStore.GetManualResolutionCacheBySourceIdentity GormResolutionCacheStore.GetResolutionCache GormResolutionCacheStore.SaveResolutionCache
ImageInfoPendingUploadCount ImageInfoURLCount Inspection InspectionAction InspectionAttributePayload InspectionCategoryPayload
InspectionSKCPatchPayload InspectionSKUPatchPayload InspectionSaleAttributePayload InspectionSection IsSDSImageURL IsUploadedImageURL
MultimodalTextGenerator NewGormResolutionCacheStore NormalizeSaleDimension Package.Inspection Package.Submission Package.SubmissionEvents
Package.SubmissionState PrepareProductForNewSubmit PrepareProductForSubmit ProductAttributesReadyForSubmit ProductImageURLCount ProductPendingImageUploadCount
RepairSubmitSaleAttributes ResolutionCacheHitSourceMemoryCache ResolutionCacheHitSourcePersistentHistoryCache ResolutionCacheHitSourcePersistentManualCache ResolutionCacheHitSourcePublishRemembered ResolutionCacheKindAttribute
ResolutionCacheKindCategory ResolutionCacheKindPricing ResolutionCacheKindSaleAttribute ResolutionCacheKindSaleAttributeCustomDenied ResolutionCacheKindSizeAttribute ResolutionCacheShortKeyDeleter
ResolutionCacheSourceIdentityGetter ResolutionCacheStore ResolveSingleManualSaleAttributeValue ResolveSingleSaleAttributeValue RetrySensitiveWordCleanup SKCMainImageURL
SetSensitiveWordRepository SetVariantImageCoverageMetadata SheinResolutionCacheEntry ValidateProductPublishPayload VariantImageCoverageInput VariantImageCoverageMessageKey
VariantImageCoverageStatusKey VariantImageGroupCount VariantImageGroupKey applyProductSizeAttributes buildCustomAttributeValueNameMultis buildResolvedSaleAttribute
buildSKCAssignments buildSKUAssignments buildSourceVariantDimensions buildValueAssignments buildValueAssignmentsPreservingSourceValues buildValueAssignmentsWithDeniedStore
buildValueAssignmentsWithPreparationPolicy cloneAttributeResolution cloneAttributeResolutionWithCacheNote cloneCategoryResolution cloneCategoryResolutionWithCacheNote cloneCategorySuggestion
clonePendingAttributeCandidates cloneResolutionCacheInfo cloneSaleAttributeCandidateInfos cloneSaleAttributeResolution cloneSaleAttributeResolutionWithCacheNote cloneSaleAttributeTemplateOptions
cloneSourceVariantDimensions collectAttributeNames collectVariantAttributeKeys comparableAttributeValueForms comparableOptionScore compareComparableForm
compositionAttributeNotes containsNumericToken customSaleAttributePermissionDeniedCache customSaleAttributePermissionDeniedCacheKey customSaleAttributePermissionDeniedKey isCustomSaleAttributePermissionDenied
isCustomSaleAttributePermissionDeniedError isLikelySaleAttributeCodePrefix isTemplateImportant isTemplateRequired matchSaleAttributeValueByComparableScore matchSaleAttributeValueDeterministic
matchSaleAttributeValueExact matchSaleAttributeValueNormalized maxSubmitWeightGrams minSubmitWeightGrams newTemplateIndex normalizeEnglishMaterial
normalizeNumericDisplayAttributeValue normalizeSaleAttributeValue normalizeSizeText numericAttributeNotes rememberCustomSaleAttributePermissionDenied resetCustomSaleAttributePermissionDeniedCache
resolveCustomSaleAttributeValues resolveSKCName resolveSingleSaleAttributeValue saleAttributeLeadingScalePattern saleAttributeNoisePattern saleAttributeParenContentPattern
saleAttributeSourceDimensions stripComparableSizeAnnotations submitSaleAttributesNeedRepair synthesizeEnglishTitle templateIndex templateIndex.FindAttribute
templateIndex.Match trimSaleAttributeCodePrefix uniqueDimensionValues uniqueNonEmpty
`),
	"internal/shein/submitprep": strings.Fields(`
CleanSensitiveWords CleanSensitiveWordsWithContext RetrySensitiveWordCleanup RetrySensitiveWordCleanupWithContext applyAuthorizedBrandFromRuntime autoDiscoveredSensitiveWordRemark
detectSensitiveWordLanguage mergeSensitiveWordRemark mergeSensitiveWordTags persistValidationSensitiveWords sensitiveWordRecordKey splitCSV
`),

	"internal/app/worker": strings.Fields(`
NewTaskJobHandler TaskCompletionNotifier TaskJobHandler
`),
	"internal/listingadmin": strings.Fields(`
AutoMigrateCategoryRepository AutoMigrateFilterRuleRepository AutoMigrateOperationStrategyRepository
AutoMigratePricingRuleRepository AutoMigrateProductDataRepository AutoMigrateProductImportMappingRepository
AutoMigrateProfitRuleRepository Category CategoryQuery CategoryRepository CommonResult ErrCategoryHasChildren
ErrCategoryNotFound ErrFilterRuleNotFound ErrOperationStrategyNotFound ErrPricingRuleNotFound
ErrProductDataNotFound ErrProductImportMappingNotFound ErrProductImportMappingOwnerRequired
ErrProfitRuleNotFound ErrRawJSONDataNotFound FilterRule FilterRuleAPI FilterRulePage FilterRuleQuery
FilterRuleRepository FilterRuleReqDTO FilterRuleRespDTO FilterRuleToRespDTO GormCategoryRepository
GormFilterRuleRepository GormOperationStrategyRepository GormPricingRuleRepository GormProductDataRepository
GormProductImportMappingRepository GormProfitRuleRepository GormRawJSONDataRepository ImageDownloader ImageInfo
IsRuleStatusEnabled NewGormCategoryRepository NewGormFilterRuleAPI NewGormFilterRuleRepository
NewGormOperationStrategyAPI NewGormOperationStrategyRepository NewGormPricingRuleRepository
NewGormProductDataAPI NewGormProductDataRepository NewGormProductImportMappingAPI
NewGormProductImportMappingRepository NewGormProfitRuleRepository NewGormRawJSONDataRepository
NewGormRawJsonDataAPI NewProductDataBatchSaveReqDTO NewProductDataItemDTO OperationStrategy
OperationStrategyAPI OperationStrategyDTO OperationStrategyPage OperationStrategyQuery
OperationStrategyRepository OperationStrategyToDTO PricingRule PricingRulePage PricingRuleQuery
PricingRuleRepository ProductAttributesItemDTO ProductData ProductDataAPI ProductDataBatchSaveReqDTO
ProductDataBatchUpdateAttributesReqDTO ProductDataDTO ProductDataFromAttributesBatchReq
ProductDataFromBatchSaveReq ProductDataItemDTO ProductDataListByStorePageReqDTO ProductDataPage
ProductDataQuery ProductDataRepository ProductDataRespDTO ProductDataToDTO ProductImportMapping
ProductImportMappingAPI ProductImportMappingCheckReqDTO ProductImportMappingCreateReqDTO
ProductImportMappingFromCreateReq ProductImportMappingGetByPlatformProductIdAndStoreReqDTO
ProductImportMappingGetBySkuReqDTO ProductImportMappingGetByTaskAndSkuReqDTO ProductImportMappingGetReqDTO
ProductImportMappingPage ProductImportMappingQuery ProductImportMappingRepository ProductImportMappingRespDTO
ProductImportMappingToRespDTO ProductVariantConfirmationReqDTO ProfitRule ProfitRulePage ProfitRuleQuery
ProfitRuleRepository RawJSONData RawJSONDataRepository RawJSONDataToRespDTO RawJsonDataAPI
RawJsonDataCreateReqDTO RawJsonDataReqDTO RawJsonDataRespDTO ResolveStoreOwnerUserID RuleStatusDisabled
RuleStatusEnabled ShelfStatusDeleted ShelfStatusOffShelf ShelfStatusOnShelf ShelfStatusPending
ShelfStatusRejected ShelfStatusReviewing activityProfitRatePtr applyCategoryAuditFields applyCategoryDefaults
applyCategoryQuery applyFilterRuleAuditFields applyFilterRuleDefaults applyFilterRuleQuery
applyOperationStrategyAuditFields applyOperationStrategyQuery applyPricingRuleAuditFields
applyPricingRuleDefaults applyPricingRuleQuery applyProductDataAuditFields applyProductDataQuery
applyProductImportMappingAuditFields applyProductImportMappingDefaults applyProductImportMappingQuery
applyProfitRuleAuditFields applyProfitRuleDefaults applyProfitRuleQuery boolToInt16 defaultActivityPartakeType
ensureOperationStrategyActivityColumns ensureTextColumn findCategoryRows findFilterRuleRows
findOperationStrategyRows findPricingRuleRows findProductDataRows findProductImportMappingRows
findProfitRuleRows flexibleStringToFloat64 flexibleTimeValue floatPtrIfPositive floatValue gormFilterRuleAPI
gormOperationStrategyAPI gormProductDataAPI gormProductImportMappingAPI gormRawJsonDataAPI int64Ptr
int64PtrIfPositive int64Value intPtrIfPositive intValue listingCategory listingCategoryFromCategory
listingFilterRule listingFilterRuleFromFilterRule listingOperationStrategy
listingOperationStrategyFromOperationStrategy listingPricingRule listingPricingRuleFromPricingRule
listingProductData listingProductDataFromProductData listingProductImportMapping
listingProductImportMappingFromProductImportMapping listingProfitRule listingProfitRuleFromProfitRule
listingRawJSONData nullableFloat64 operationStrategyFlexibleString operationStrategyFloat64Value
operationStrategyIntValue parseOptionalMultiplier ptrFloat64 ptrInt ptrString rawJSONBytes rawJSONOrString
resolveStoreOwnerUserID resolveStoreOwnerUserIDForUpdate
`),
	"internal/listingruntime": strings.Fields(`
FilterRule ProductImportMappingUpsert ProfitRule StorePauseStatusDetail StoreService
`),
	"internal/marketplace/shein/workspace": strings.Fields(`
ActionItem ActionQueueAttributes ActionQueueClassification ActionQueueDescriptors ActionQueueFinalReview
ActionQueueManualReview ActionQueueMedia ActionQueuePayloadRebuild ActionQueuePricing ActionQueueSourceReview
ActionQueueStoreAuth ActionQueueSubmitReady ActionQueueVariant ApplyAttributeResolutionPatch
ApplyCategoryResolutionPatch ApplySKCRevisionPatches ApplySKURevisionPatches ApplySaleAttributeResolutionPatch
AttachSubmissionEventStoreResolution AttributeResolutionPatch BuildAppliedChangesPreview BuildAttributeEffects
BuildAttributePayload BuildAttributeRecommendationMeta BuildAttributeResolutionPatch BuildAttributeSuggestions
BuildCategoryEffects BuildCategoryPayload BuildCategoryRecommendationMeta BuildCategoryResolutionPatch
BuildCurrentHistoryComparePreview BuildEditorContext BuildEditorDirtyHints BuildEditorProgress
BuildEditorRevisionSkeleton BuildEditorSKCPatches BuildFinalReviewImages BuildFinalReviewSKU
BuildFinalReviewSKUs BuildFreshnessAttributeCheck BuildFreshnessAuthFailureCheck BuildFreshnessAuthSuccessCheck
BuildFreshnessCategoryCheck BuildFreshnessSaleAttributeCheck BuildHistoryComparePreview BuildHistoryDetail
BuildHistoryNavigation BuildHistoryRestoreContext BuildHistoryRestoreDetailData BuildHistoryRestoreMessages
BuildHistoryRestoreNextActions BuildHistoryRestoreOverview BuildHistoryRestorePresentationData
BuildHistoryRestoreSafety BuildImageUploadPreflight BuildInspection BuildMinimalRevisionSkeleton
BuildPreviewCardStatus BuildPreviewCardSummary BuildPreviewReviewSummary BuildReadinessGuidanceSpec
BuildReadinessPatchPayload BuildReadinessReason BuildRepairApplyQueue BuildRepairCenter
BuildRepairCenterFromReadiness BuildRepairCenterStatus BuildRepairCenterSummary BuildRepairPlan
BuildRepairReason BuildRepairRevisionInput BuildRepairRevisionSeed BuildRepairSession BuildRepairStateInput
BuildRepairValidationPreview BuildResolutionCacheSummary BuildRestoreDraftFromSkeleton
BuildRestorePreviewPayload BuildRestoreRequestSeed BuildRevisionDiffBetweenRevisions BuildRevisionDiffPreview
BuildRevisionDiffPreviewFromInput BuildRevisionInputFromEditorContext BuildSaleAttributeEffects
BuildSaleAttributePayload BuildSaleAttributeResolutionPatch BuildSaleCandidateSuggestions
BuildSaleRecommendationMeta BuildSourceProductSummary BuildStatusOverview BuildStoreResolutionSummary
BuildSubmissionStoreResolution BuildSubmitChecklist BuildSubmitStateInput BuildSuccessFollowUpChecklist
BuildSuccessFollowUpOverview BuildSuccessMessages BuildSuccessNextActions BuildSuccessPayload
BuildSuccessPresentation BuildSuccessRecommendedView BuildSuccessStatusSummary
BuildSuccessSuggestedFollowUpRevision BuildSuccessSummaryCard BuildTaskActionQueue BuildTaskWorkQueue
BuildValidationPayload BuildWorkspaceOverview CategoryResolutionPatch CategorySearchCandidate
ChecklistGroupItem ChecklistItemCount CloneAttributeResolutionPatch CloneCategoryResolutionPatch
CloneEditorRevisionSkeleton CloneReadinessItems CloneReadinessReason CloneRepairPatchPayload
CloneRepairValidationPreview CloneRevisionDiffPreview CloneRevisionInput CloneSKCRevisionPatches
CloneSaleAttributeResolutionPatch CookieUnavailableReviewNotes DisplayTitleInput EditorAttributeContext
EditorAttributeSuggestion EditorBasicsContext EditorCategoryContext EditorContext EditorDirtyHintSection
EditorDirtyHints EditorEffect EditorProgress EditorProgressSection EditorRecommendationMeta
EditorRevisionSkeleton EditorSaleAttributeContext EditorSaleCandidateSuggestion EvaluateAttributeFreshness
EvaluateCategoryFreshness EvaluateSaleAttributeFreshness EvaluateSaleAttributeFreshnessWithCustomValidation
FacetDescriptor FieldError FilterOutCookieUnavailableReviewNotes FinalReviewImage FinalReviewSKU FindKeys
FindLabels FreshnessAttributeKey FreshnessAuthKey FreshnessCategoryKey FreshnessSaleAttributeKey
HasCookieUnavailableReviewNotes HistoryComparePreview HistoryCompareRecord HistoryDetail HistoryNavigation
HistoryRestoreCompareInput HistoryRestoreContext HistoryRestoreDetailData HistoryRestoreMessages
HistoryRestoreOverview HistoryRestorePresentationData HistoryRestoreRecommendedView HistoryRestoreRecordInput
HistoryRestoreSafety HistoryRestoreStateInput HistoryRestoreTimeline ImageUploadCacheHit ImageUploadClassifier
ImageUploadPreflight InspectionReviewReasons IsCookieUnavailableText IsEmptyRevisionInput
IsSaleAttributeResolved JoinReadinessLabels NewFieldError PreviewCardNeedsReview PruneAttributeResolutionPatch
PruneCategoryResolutionPatch PruneRevisionInput PruneSaleAttributeResolutionPatch ReadinessGuidanceSpec
ReadinessHintSpec ReadinessReason ReadinessReasonSpec RebuildRestorePreviewPayload RepairApplyQueue
RepairApplyQueueItem RepairArtifacts RepairCenter RepairCenterAction RepairCenterActionCount
RepairCenterDirectApplyCount RepairCenterFromReadinessOptions RepairCenterPrimaryPlanStatus RepairCenterSection
RepairCenterSeedAction RepairCenterSessionStatus RepairCenterStats RepairCompletionSnapshot RepairHint
RepairHintAccessors RepairPatchPayload RepairPlan RepairPlanStep RepairResumeState RepairRevisionSeed
RepairRunbookStep RepairSession RepairSessionActionInfo RepairStateInput RepairValidationPreview
ResolutionCacheSummary ResolveDisplayTitle ResolveHistoryCompareTarget RestorePreviewCoreData
RestorePreviewPayload RestoreRequestSeed RevisionDiffPreview RevisionFieldChange RevisionInput SKCRevisionPatch
SKURevisionPatch SaleAttributeResolutionPatch SceneApplySuccess SceneRestoreSuccess SearchCategoryCandidates
SessionInput SourceProductSummary StatusOverview StoreResolutionSummary StripCookieUnavailableReviewNotes
SubmitChecklist SubmitChecklistGroupForKey SubmitStateInput SuccessCoreData SuccessFollowUpChecklist
SuccessFollowUpOverview SuccessInteractionPresentation SuccessMessages SuccessModeApply SuccessModeRestore
SuccessPayload SuccessRecommendedView SuccessStatusSummary SuccessSummaryCard ToActionItems
ValidateRevisionInput ValidationPayload WorkQueueDescriptors WorkQueueDraft WorkQueueGeneration
WorkQueueGenerationFailed WorkQueuePublished WorkQueueRepair WorkQueueReview WorkQueueSubmitFailed
WorkQueueSubmitReady WorkflowStatusDescriptors WorkflowStatusDraftSaved WorkflowStatusPendingConfirmation
WorkflowStatusPublishFailed WorkflowStatusPublished WorkflowStatusReadyToSubmit WorkspaceOverview
WorkspaceRepairState WorkspaceSessionEntry WorkspaceSubmitState actionQueueDescriptors actionQueueForKey
addPreviewImageInfo addPreviewProductImages appendAppliedSaleAttributeReviewChanges appendAttributesChange
appendCompareAttributesChange appendCompareImageChange appendCompareIntChange appendCompareIntPtrChange
appendCompareIntSliceChange appendCompareResolvedAttributesChange appendCompareSalePatchChange
appendCompareStringChange appendImageChange appendImageInfoURLs appendIntChange appendIntPtrChange
appendIntSliceChange appendResolvedAttributesChange appendSalePatchChange appendStringChange
applyPrimaryActionFromItems attributeConfidence attributeFreshnessIssueState attributeFreshnessTemplateContext
attributeReason attributeSuggestionConfidence attributeSuggestionReason buildApplyHighlights
buildApplyPrimaryAction buildApplySubtitle buildAttributeActions buildAttributeFreshnessOutcome
buildAttributeFreshnessTemplateContext buildAttributeInspectionSection buildAttributePayload
buildAttributePayloadMap buildAttributeProgressSection buildBasicProgressSection buildCategoryActions
buildCategoryInspectionSection buildCategoryPayload buildCategoryPayloadMap buildCategoryProgressSection
buildEditorSKUPatches buildFreshnessSaleTemplateOptions buildHistoryRestoreRecommendedView
buildImageUploadPreflightSummary buildNextActions buildNextActionsFromReadiness buildPendingAttributeCandidates
buildPendingAttributes buildRepairAffectedSections buildRepairCompletionRule buildRepairCompletionSnapshot
buildRepairPlanCopy buildRepairPlanStatus buildRepairResumeState buildRepairSessionCopy
buildRepairSessionStatus buildRepairSkippedSteps buildRepairStepHighlights buildRepairStepReason
buildResolvedAttributeTemplateCandidates buildResolvedAttributeTemplateDriftDetails buildSKCPatchSuggestions
buildSKUPatchSuggestions buildSaleAttributeActions buildSaleAttributeFreshnessResolutionOutcome
buildSaleAttributeFreshnessTemplateContext buildSaleAttributeInspectionSection buildSaleAttributePayload
buildSaleAttributePayloadMap buildSaleProgressSection buildStatusHighlights buildSuccessFollowUpHeadline
categoryConfidence categoryMatchScore categoryReason categorySearchMatch checklistItemAsReadinessItem
cloneAttributePatch cloneBoolPointer cloneCategoryPatch cloneFacetDescriptors cloneImageSet cloneIntPointer
cloneMap cloneNonNegativeIntPointer clonePendingAttributeCandidates clonePositiveIntPointer
cloneRepairArtifacts cloneRepairReason cloneRepairSKCPatches cloneRepairSKUPatches cloneRequestSKCs
cloneRequestSKUs cloneResolvedSaleAttributeMap cloneRevisionDiffPreview cloneSKCPatches cloneSKUPatches
cloneSalePatch cloneSourceDimensionsForInspection cloneStringPointer cloneSuccessFollowUpChecklist
cloneSuccessFollowUpOverview cloneSuccessMessages cloneSuccessRecommendedView cloneSuccessStatusSummary
cloneSuccessSummaryCard cloneTemplateOptionsForInspection collectAttributeDirtyFields collectBasicDirtyFields
collectCategoryDirtyFields collectInvalidSaleAttributes collectProductImageURLs collectRepairCandidates
collectSaleDirtyFields compareRepairActions conditionalOtherAttribute containsNormalized
dedupeCustomAttributeRelations defaultInspectionReviewReason defaultSuccessEditorReason
defaultSuccessInspectionReason defaultSuccessLabel defaultSuccessSubmitReason defaultSuccessTitle
defaultSuccessWarningTitle dependencyIsActive diffResolvedAttributes enrichAttributeResolutionCacheInfo
enrichCategoryResolutionCacheInfo enrichPricingResolutionCacheInfo enrichSaleAttributeResolutionCacheInfo
enrichSizeAttributeResolutionCacheInfo ensureImageDraft equalIntSlices equalPlatformAttributes
equalResolvedAttributes evaluateAttributeFreshnessIssueState evaluateResolvedAttributeFreshness
evaluateSaleAttributeFreshnessInvalidState evaluateSaleAttributeFreshnessResolution
filterFreshnessDisplayAttributes filterFreshnessSaleScopeAttributes filterNonEmptyStrings findPackageSKC
findPackageSKU findReadinessItem findRequestSKC findRequestSKU findSKCPackageBySupplierCode firstAction
firstNonEmptyString firstNonEmptyValue firstSummaryLine flattenAttributeTemplatesByID
formatFreshnessAttributeName formatResolvedAttributeDiffItem formatResolvedSaleAttributeDiffItem
freshnessCustomAttributeValueIDs hasHistoryRestoreWarning hasImageContent hasResolvedSaleAttributeDraft
indexNormalized intPointer isEmptyAttributePatch isEmptyCategoryPatch isEmptySalePatch isFinalReviewSwatchRole
isTemplateRequired maxCategorySearchResults maxInt mergeFinalReviewImage messageDescription messageTitle
messageWarnings normalizeFinalReviewAttributeName normalizeRepairSection normalizeReviewReasons normalizeText
orderFreshnessSaleScopeAttributes parseCascadeValueIDs parseMoney previewImageDetailRole progressSection
pruneSKCRevisionPatches pruneSKURevisionPatches readinessSummary recommendedViewValue repairActionCandidate
repairActionID repairExecutionMode repairFreshnessSaleAttributes repairPriorityRank repairSectionLabel
repairSectionRank repairStatusRank resolveFinalReviewImageRole resolvedAttributeStillLegal
saleAttributeCategoryReviewReason saleAttributeFreshnessInvalidState saleAttributeFreshnessTemplateContext
saleAttributeRecommendCategoryReview saleCandidateConfidence saleCandidateReason saleConfidence saleReason
stringPointerOrNil stringsContainsNormalized stringsTrim summaryHighlights summaryStatus summarySubheadline
toSubmitStateInput tryRepairFreshnessSaleAttribute workQueueDescriptors workflowStatusDescriptors
workspaceHeadlineFromReadiness workspacePrimaryView
`),
	"internal/model": strings.Fields(`
ApplyRetryFailure ParseTaskStatus ResolveTaskMaxRetries RetryDecision Task TaskStatus TaskStatusCancelled
TaskStatusCrawlFailed TaskStatusCrawled TaskStatusDraft TaskStatusPaused TaskStatusPending
TaskStatusPendingRetry TaskStatusProcessing TaskStatusPublished TaskStatusQueued TaskStatusRepublishing
TaskStatusResumed TaskStatusResuming TaskStatusTerminated ValidateTaskStatusTransition
ValidateTaskStatusTransitionCode defaultTaskMaxRetries retryPriorityPenalty taskStatusNames
taskStatusTransitions
`),
	"internal/shein/sherr": strings.Fields(`
AsTaskHandledError IsTaskHandledError NewTaskHandledError TaskHandledError
`),
}

func TestNumericListingSupportClosureSourcesAreRetired(t *testing.T) {
	for _, relative := range retiredNumericListingSupportFiles {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired numeric listing support source remains: %s (stat: %v)", relative, err)
		}
	}
	for _, relative := range []string{"internal/taskrpcapi", "internal/workspace/shein", "internal/core/metrics", "internal/publishing/sheinmanaged", "internal/shein/category", "internal/shein/aicache"} {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired numeric listing support package remains: %s (stat: %v)", relative, err)
		}
	}
	sources := trackedProductionTextSources(t, []string{"."}, func(path string) bool {
		return strings.HasSuffix(path, ".go")
	})
	violations, err := phase3BannedImportDeclarationViolations(sources, []string{
		"task-processor/internal/taskrpcapi",
		"task-processor/internal/workspace/shein",
		"task-processor/internal/core/metrics",
		"task-processor/internal/publishing/sheinmanaged",
		"task-processor/internal/shein/category",
		"task-processor/internal/shein/aicache",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range violations {
		t.Errorf("retired numeric listing support import remains: %s", violation)
	}
	banned := make(map[string]map[string]bool)
	for relative, names := range retiredNumericListingSupportDeclarations {
		banned[relative] = make(map[string]bool)
		for _, name := range names {
			banned[relative][name] = true
		}
	}
	for _, source := range sources {
		file, err := parser.ParseFile(token.NewFileSet(), source.path, source.text, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			imported, err := decodeGoImportPath(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			// Only the old root hook is retired; imageagent remains current.
			if imported == "task-processor/internal/app/worker" {
				t.Errorf("%s imports retired root Task worker hook", source.path)
			}
		}
		if strings.HasSuffix(source.path, "_test.go") || strings.HasPrefix(source.path, "tests/") {
			continue
		}
		for _, route := range []string{
			"/api/v1/management/tasks/health",
			"/api/v1/management/tasks/:task_id/status",
			"/api/v1/management/tasks/queue-stats",
		} {
			if strings.Contains(source.text, route) {
				t.Errorf("%s recreates retired Task RPC route %s", source.path, route)
			}
		}
		names := banned[filepath.ToSlash(filepath.Dir(source.path))]
		checkName := func(name string) {
			if names[name] {
				t.Errorf("%s recreates retired numeric listing support declaration %s", source.path, name)
			}
		}
		for _, declaration := range file.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				if declaration.Recv == nil {
					checkName(declaration.Name.Name)
				} else if len(declaration.Recv.List) == 1 {
					receiver := declaration.Recv.List[0].Type
					if pointer, ok := receiver.(*ast.StarExpr); ok {
						receiver = pointer.X
					}
					if owner, ok := receiver.(*ast.Ident); ok {
						checkName(owner.Name + "." + declaration.Name.Name)
					}
				}
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						checkName(spec.Name.Name)
						if shape, ok := spec.Type.(*ast.StructType); ok {
							for _, field := range shape.Fields.List {
								for _, name := range field.Names {
									checkName(spec.Name.Name + "." + name.Name)
								}
							}
						}
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							checkName(name.Name)
						}
					}
				}
			}
		}
	}
}

var retiredTEMUListingPackages = []string{
	"internal/temu/ai", "internal/temu/context", "internal/temu/filter",
	"internal/temu/handlerbase", "internal/temu/pricing", "internal/temu/product",
	"internal/temu/property", "internal/temu/rules", "internal/temu/sku", "internal/temu/spec",
	"internal/pipeline", "internal/pipeline/handlers", "internal/pkg/skugen",
	"internal/marketplace/temu/publishing",
}

var retiredAmazonListingPackages = []string{
	"internal/amazon/attribute", "internal/amazon/image", "internal/amazon/listing",
	"internal/amazon/llm", "internal/amazon/model", "internal/amazon/pipeline", "internal/amazon/schema",
	"internal/amazonlisting", "internal/marketplace/amazon", "internal/state",
	"internal/httpbootstrap", "internal/shared/timex",
}

func TestAmazonListingClosureSourcesAreRetired(t *testing.T) {
	paths := append([]string{}, retiredAmazonListingPackages...)
	paths = append(paths, "hack/debug/test-amazon-listing", "deployments/kubernetes/amazon-listing")
	for _, relative := range paths {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired Amazon listing source remains: %s (stat: %v)", relative, err)
		}
	}
	var banned []string
	for _, relative := range retiredAmazonListingPackages {
		banned = append(banned, "task-processor/"+relative)
	}
	sources := trackedProductionTextSources(t, []string{"."}, func(path string) bool {
		return strings.HasSuffix(path, ".go")
	})
	violations, err := phase3BannedImportDeclarationViolations(sources, banned)
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range violations {
		t.Errorf("retired Amazon listing import remains: %s", violation)
	}
}

func TestTemuListingClosureSourcesAreRetired(t *testing.T) {
	paths := append([]string{}, retiredTEMUListingPackages...)
	paths = append(paths,
		"internal/listingadmin/gorm_profit_rule_api.go", "internal/listingadmin/gorm_pricing_rule_api.go",
		"prompts/temu/attribute_mapping.yaml", "prompts/temu/content_rewriter.yaml", "prompts/temu/sku_variant_mapping.yaml",
		"data/prohibited_items_temu.json", "data/sensitive_words_temu.json",
	)
	for _, relative := range paths {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired TEMU listing source remains: %s (stat: %v)", relative, err)
		}
	}
	for relative, retired := range map[string][]string{
		"internal/listingadmin": {"ProfitRuleAPI", "ProfitRuleReqDTO", "ProfitRuleRespDTO", "PricingRuleAPI", "PricingRuleReqDTO", "PricingRuleRespDTO", "NewGormProfitRuleAPI", "gormProfitRuleAPI", "ProfitRuleToRespDTO", "NewGormPricingRuleAPI", "gormPricingRuleAPI", "PricingRuleToRespDTO", "DailyListingCountGetReqDTO", "DailyListingCountSetReqDTO", "DailyListingCountRespDTO", "TryConsumeDailyQuotaReqDTO", "TryConsumeDailyQuotaRespDTO", "RollbackDailyQuotaReqDTO", "DailyListingCountAPI"},
		"internal/core/config":  {"SensitiveWordsConfig", "DefaultSensitiveWordsConfig", "GetWordsForLanguage", "AddStaticWords", "AddDynamicWords"},
	} {
		root := filepath.Join("..", filepath.FromSlash(relative))
		index, err := loadGoFileIndex(root, "")
		if err != nil {
			t.Fatal(err)
		}
		for path := range index.files {
			if filepath.Dir(path) != root || strings.HasSuffix(path, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, declaration := range file.Decls {
				var names []*ast.Ident
				switch declaration := declaration.(type) {
				case *ast.FuncDecl:
					names = append(names, declaration.Name)
				case *ast.GenDecl:
					for _, spec := range declaration.Specs {
						switch spec := spec.(type) {
						case *ast.TypeSpec:
							names = append(names, spec.Name)
						case *ast.ValueSpec:
							names = append(names, spec.Names...)
						}
					}
				}
				for _, name := range names {
					for _, symbol := range retired {
						if name.Name == symbol {
							t.Errorf("%s recreates retired TEMU listing declaration %s", path, symbol)
						}
					}
				}
			}
		}
	}
}

func TestListingKitTaskBackendSourcesAreRetired(t *testing.T) {
	if _, err := os.Lstat(filepath.Join("..", "internal", "temu", "api")); !os.IsNotExist(err) {
		t.Errorf("retired TEMU API root remains: %v", err)
	}
	if _, err := os.Lstat(filepath.Join("..", "internal", "listingruntime")); !os.IsNotExist(err) {
		t.Errorf("retired numeric mapping owner internal/listingruntime must be absent: %v", err)
	}
	for _, relative := range retiredSheinPolicyContractPackages {
		if _, err := os.Lstat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired SHEIN policy/contract root %s must be absent: %v", relative, err)
		}
	}
	for _, relative := range []string{
		"internal/shein/models.go", "internal/shein/pricing/price_manager.go",
		"internal/shein/pricing/calculator.go", "internal/shein/pricing/calculator_test.go",
		"internal/shein/pricing/cost_profit_calculator.go", "internal/shein/pricing/cost_profit_calculator_test.go",
	} {
		if _, err := os.Lstat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired SHEIN policy/contract artifact %s must be absent: %v", relative, err)
		}
	}
	const retiredPolicyRoot = "internal/marketplace/productpolicy"
	if _, err := os.Lstat(filepath.Join("..", filepath.FromSlash(retiredPolicyRoot))); !os.IsNotExist(err) {
		t.Errorf("retired product policy root must be absent: %v", err)
	}
	retiredPolicyDeclarations := map[string]bool{}
	for _, name := range strings.Fields("FilterRule RuleChecker NewRuleChecker CheckPriceRange CheckInventory CheckRating CheckReviewCount CheckDeliveryTime CheckFulfillmentType CheckAllRules GetProductPrice GetFreight GetInventory extractStockFromAvailability IsFBAFulfillment IsAMZSeller") {
		retiredPolicyDeclarations[name] = true
	}
	for _, relative := range trackedFiles(t, ".") {
		if !strings.HasSuffix(relative, ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", filepath.FromSlash(relative)), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			imported, err := decodeGoImportPath(spec.Path.Value)
			if err != nil {
				t.Fatalf("decode import in %s: %v", relative, err)
			}
			if importMatchesPrefix(imported, "task-processor/"+retiredPolicyRoot) {
				t.Errorf("%s imports retired product policy %s", relative, imported)
			}
			for _, retired := range retiredSheinPolicyContractPackages {
				if importMatchesPrefix(imported, "task-processor/"+retired) {
					t.Errorf("%s imports retired SHEIN policy/contract %s", relative, imported)
				}
			}
			// Only the root package retires; API/client/content subpackages remain.
			if imported == "task-processor/internal/shein" {
				t.Errorf("%s imports retired root SHEIN contract %s", relative, imported)
			}
		}
		owner := filepath.ToSlash(filepath.Dir(relative))
		for _, declaration := range file.Decls {
			var names []*ast.Ident
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				names = append(names, declaration.Name)
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						names = append(names, spec.Name)
					case *ast.ValueSpec:
						names = append(names, spec.Names...)
					}
				}
			}
			for _, name := range names {
				if owner == "internal/shein" && name.Name == "AmazonMonitorData" {
					t.Errorf("%s recreates retired root SHEIN declaration %s", relative, name.Name)
				}
				if owner == "internal/shein/pricing" && slices.Contains([]string{
					"PriceManager", "NewPriceManager", "CostPriceRequest", "AutoPricingRule",
					"AutoPricingCalculator", "NewAutoPricingCalculator", "CostProfitCalculator", "NewCostProfitCalculator",
				}, name.Name) {
					t.Errorf("%s recreates retired SHEIN pricing declaration %s", relative, name.Name)
				}
			}
		}
		// Scope declarations to the retired root/owner. Same-named platform
		// APIs, rule methods and negative fixture strings elsewhere remain valid.
		if !importMatchesPrefix(filepath.ToSlash(relative), retiredPolicyRoot) && file.Name.Name != "productpolicy" {
			continue
		}
		for _, declaration := range file.Decls {
			var names []*ast.Ident
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				names = append(names, declaration.Name)
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						names = append(names, spec.Name)
					case *ast.ValueSpec:
						names = append(names, spec.Names...)
					}
				}
			}
			for _, name := range names {
				if retiredPolicyDeclarations[name.Name] {
					t.Errorf("%s recreates retired product policy declaration %s", relative, name.Name)
				}
			}
		}
	}
	for _, relative := range []string{"internal/temu/image", "internal/temu/handlerbase/types.go", "internal/temu/api/image", "prompts/temu/vision_detector.yaml", "prompts/shein/attribute_selector.yaml", "prompts/shein/attribute_value_fallback.yaml", "prompts/shein/translation.yaml"} {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired image artifact remains: %s (stat: %v)", relative, err)
		}
	}
	rootFiles, err := filepath.Glob(filepath.Join("..", "internal", "listingkit", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rootFiles) != 0 {
		t.Errorf("mixed ListingKit root must be absent, found %d Go files", len(rootFiles))
	}
	rootProcessorFiles, err := filepath.Glob(filepath.Join("..", "internal", "temu", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rootProcessorFiles) != 0 {
		t.Errorf("retired TEMU task processor root remains: %v", rootProcessorFiles)
	}
	for _, relative := range []string{
		"internal/shein/addresscopy/helpers.go",
		"internal/shein/addresscopy/helpers_test.go",
		"internal/shein/addresscopy/models.go",
		"internal/shein/addresscopy/service.go",
		"internal/shein/attribute.go",
		"internal/shein/error_exports.go",
		"internal/shein/product.go",
		"internal/shein/sensitive_words_filter.go",
		"internal/shein/task_reason.go",
		"internal/shein/types.go",
		"internal/shein/category/ai_selector.go",
		"internal/shein/category/category_repo.go",
		"internal/shein/category/tree.go",
		"internal/shein/content/words_processor.go",
		"internal/shein/context/attribute.go",
		"internal/shein/context/context.go",
		"internal/shein/context/context_test.go",
		"internal/shein/context/sale_attribute_selection_state.go",
		"internal/shein/languageconfig/languages.go",
		"internal/shein/languageconfig/languages_test.go",
		"internal/shein/managedclient/api_client_test.go",
		"internal/shein/managedclient/bridge.go",
		"internal/shein/managedclient/manager.go",
		"internal/shein/managedclient/runtime_adapters.go",
		"internal/shein/mapping/builder.go",
		"internal/shein/mapping/builder_owner_test.go",
		"internal/shein/mapping/handler.go",
		"internal/shein/mapping/service.go",
		"internal/shein/mapping/strategies.go",
		"internal/shein/mapping/strategies_test.go",
		"internal/shein/mapping/types.go",
		"internal/shein/product/attribute/apparel_size_resolver.go",
		"internal/shein/product/attribute/custom_processor.go",
		"internal/shein/product/attribute/custom_processor_test.go",
		"internal/shein/product/attribute/fill.go",
		"internal/shein/product/attribute/importance.go",
		"internal/shein/product/attribute/importance_test.go",
		"internal/shein/product/attribute/input.go",
		"internal/shein/product/attribute/mapper.go",
		"internal/shein/product/attribute/mapper_runtime_input.go",
		"internal/shein/product/attribute/mapper_test.go",
		"internal/shein/product/attribute/matcher.go",
		"internal/shein/product/attribute/matcher_test.go",
		"internal/shein/product/attribute/platform_value_domain.go",
		"internal/shein/product/attribute/platform_value_fallback.go",
		"internal/shein/product/attribute/platform_value_fallback_llm.go",
		"internal/shein/product/attribute/platform_value_fallback_llm_test.go",
		"internal/shein/product/attribute/platform_value_resolver.go",
		"internal/shein/product/attribute/prompt.go",
		"internal/shein/product/attribute/selector.go",
		"internal/shein/product/attribute/size_chart_resolver.go",
		"internal/shein/product/attribute/size_rules.go",
		"internal/shein/product/attribute/size_system.go",
		"internal/shein/product/attribute/structured_size_resolver.go",
		"internal/shein/product/attribute/task_context_sale_attribute_selection_test.go",
		"internal/shein/product/attribute/template.go",
		"internal/shein/product/attribute/types.go",
		"internal/shein/product/attribute/utils.go",
		"internal/shein/product/attribute/validate_repair.go",
		"internal/shein/product/attribute/validate_repair_test.go",
		"internal/shein/product/attribute/validator.go",
		"internal/shein/product/attribute/sale/batch_processor.go",
		"internal/shein/product/attribute/sale/batch_processor_test.go",
		"internal/shein/product/attribute/sale/cache_consistency.go",
		"internal/shein/product/attribute/sale/cache_consistency_test.go",
		"internal/shein/product/attribute/sale/comparison.go",
		"internal/shein/product/attribute/sale/context.go",
		"internal/shein/product/attribute/sale/context_test.go",
		"internal/shein/product/attribute/sale/debug.go",
		"internal/shein/product/attribute/sale/file_saver.go",
		"internal/shein/product/attribute/sale/filter.go",
		"internal/shein/product/attribute/sale/gpt.go",
		"internal/shein/product/attribute/sale/gpt_test.go",
		"internal/shein/product/attribute/sale/handler.go",
		"internal/shein/product/attribute/sale/json_parser.go",
		"internal/shein/product/attribute/sale/json_parser_test.go",
		"internal/shein/product/attribute/sale/metadata.go",
		"internal/shein/product/attribute/sale/metadata_test.go",
		"internal/shein/product/attribute/sale/preparation.go",
		"internal/shein/product/attribute/sale/product_data.go",
		"internal/shein/product/attribute/sale/prompt_generator.go",
		"internal/shein/product/attribute/sale/request.go",
		"internal/shein/product/attribute/sale/request_test.go",
		"internal/shein/product/attribute/sale/sale_attribute_input.go",
		"internal/shein/product/attribute/sale/sale_attribute_output.go",
		"internal/shein/product/attribute/sale/single_processor.go",
		"internal/shein/product/attribute/sale/single_processor_test.go",
		"internal/shein/product/attribute/sale/smart_filter.go",
		"internal/shein/product/attribute/sale/smart_filter_input.go",
		"internal/shein/product/attribute/sale/smart_filter_test.go",
		"internal/shein/product/attribute/sale/utils.go",
		"internal/shein/product/attribute/sale/utils_test.go",
		"internal/shein/product/attribute/sale/validation.go",
		"internal/shein/product/attribute/sale/validation_test.go",
		"internal/shein/product/attribute/sale/value_filter.go",
		"internal/shein/product/attribute/sale/value_filter_test.go",
		"internal/shein/product/attribute_repo.go",
		"internal/shein/product/init_data.go",
		"internal/shein/product/init_data_test.go",
		"internal/shein/product/inventory_repo.go",
		"internal/shein/product/product_manager.go",
		"internal/shein/product/product_repo.go",
		"internal/shein/product/result_merger.go",
		"internal/shein/product/shelf_quota.go",
		"internal/shein/product/shelf_quota_test.go",
		"internal/shein/product/spu_limit.go",
		"internal/shein/product/spu_record.go",
		"internal/shein/product/spu_record_test.go",
		"internal/shein/product/build/attribute_builder.go",
		"internal/shein/product/build/attribute_builder_test.go",
		"internal/shein/product/build/attribute_classifier.go",
		"internal/shein/product/build/build_attribute.go",
		"internal/shein/product/build/build_input.go",
		"internal/shein/product/build/skc_list.go",
		"internal/shein/product/build/spu.go",
		"internal/shein/product/image/processor.go",
		"internal/shein/product/image/validation.go",
		"internal/shein/product/size/chart.go",
		"internal/shein/product/size/chart_test.go",
		"internal/shein/product/size/normalizer.go",
		"internal/shein/product/size/normalizer_test.go",
		"internal/shein/product/skc/attribute_strategy.go",
		"internal/shein/product/skc/builder.go",
		"internal/shein/product/skc/image.go",
		"internal/shein/product/skc/selection_strategy.go",
		"internal/shein/product/skc/selection_strategy_test.go",
		"internal/shein/product/skc/skc_build_input.go",
		"internal/shein/product/skc/skc_build_output.go",
		"internal/shein/product/skc/translation.go",
		"internal/shein/product/skc/translation_test.go",
		"internal/shein/product/skc/validation.go",
		"internal/shein/product/skc/validation_test.go",
		"internal/shein/product/skc/variant.go",
		"internal/shein/product/skc/variant_runtime_test.go",
		"internal/shein/product/sku/builder.go",
		"internal/shein/product/sku/creator.go",
		"internal/shein/product/sku/generator.go",
		"internal/shein/product/sku/image_fixer.go",
		"internal/shein/product/sku/image_processor.go",
		"internal/shein/product/sku/sku_runtime_input.go",
		"internal/shein/product/sku/strategy.go",
		"internal/shein/product/sku/strategy_test.go",
		"internal/shein/product/sku/utils.go",
		"internal/shein/product/sku/utils_test.go",
		"internal/shein/product/variant/composite_matcher.go",
		"internal/shein/product/variant/exact_matcher.go",
		"internal/shein/product/variant/exact_matcher_test.go",
		"internal/shein/product/variant/fuzzy_matcher.go",
		"internal/shein/product/variant/match_helpers.go",
		"internal/shein/product/variant/matcher.go",
		"internal/shein/product/variant/matcher_helpers.go",
		"internal/shein/productdata/fetch_and_cache.go",
		"internal/shein/productdata/fetch_and_cache_variants.go",
		"internal/shein/productdata/raw_json.go",
		"internal/shein/productdata/raw_json_test.go",
		"internal/shein/productdata/variant_data.go",
		"internal/shein/productdata/variant_limit.go",
		"internal/shein/productdata/variant_limit_test.go",
		"internal/shein/publish/checker.go",
		"internal/shein/publish/error_handler.go",
		"internal/shein/publish/exists_check.go",
		"internal/shein/publish/handler.go",
		"internal/shein/publish/handler_test.go",
		"internal/shein/publish/mapping_helper.go",
		"internal/shein/publish/publish_input.go",
		"internal/shein/publish/publish_input_test.go",
		"internal/shein/publish/repository_helpers.go",
		"internal/shein/publish/repository_helpers_test.go",
		"internal/shein/publish/result.go",
		"internal/shein/publish/result_test.go",
		"internal/shein/publish/sale_attribute_policy.go",
		"internal/shein/publish/saver.go",
		"internal/shein/publish/validator.go",
		"internal/shein/publish/validator_attributes.go",
		"internal/shein/publish/validator_duplicate_test.go",
		"internal/shein/publish/validator_test.go",
		"internal/shein/publish/variant_success.go",
		"internal/shein/siteconfig/sites.go",
		"internal/shein/siteconfig/sites_test.go",
		"internal/shein/store/region.go",
		"internal/shein/store/region_test.go",
		"internal/shein/store/site.go",
		"internal/shein/store/site_test.go",
		"internal/shein/store/store_id.go",
		"internal/shein/store/store_id_test.go",
		"internal/shein/store/store_info.go",
		"internal/shein/store/store_info_test.go",
		"internal/shein/store/supplier.go",
		"internal/shein/store/warehouse.go",
		"internal/shein/store/warehouse_repo.go",
		"internal/shein/translate/translate.go",
		"internal/shein/translate/translate_repo.go",
		"internal/shein/translate/translate_test.go",
		"internal/shein/validation/build_attribute.go",
		"internal/shein/validation/build_attribute_test.go",
		"internal/shein/validation/daily_limit.go",
		"internal/shein/validation/daily_limit_test.go",
		"internal/shein/validation/filter_rule.go",
		"internal/shein/validation/filter_rule_test.go",
		"internal/shein/validation/get_rule.go",
		"internal/shein/validation/get_rule_test.go",
		"internal/shein/validation/quantity.go",
		"internal/shein/validation/quantity_test.go",
		"internal/shein/validation/reapply_filter.go",
		"internal/shein/validation/rule_checker.go",
		"internal/sheinbridge/saleattribute/handler.go",
		"internal/sheinbridge/saleattribute/handler_test.go",
		"internal/shein/product",
		"internal/shein/productdata",
		"internal/shein/publish",
		"internal/shein/mapping",
		"internal/shein/validation",
		"internal/shein/translate",
		"internal/shein/addresscopy",
		"internal/shein/managedclient",
		"internal/shein/store",
		"internal/shein/context",
		"internal/shein/languageconfig",
		"internal/shein/siteconfig",
		"internal/sheinbridge/saleattribute",
		"internal/listingkit/api", "internal/listingkit/httpapi", "internal/listingkit/core",
		"internal/listingkit/temporal", "internal/listingkit/workflow", "internal/listingkit/workspace",
		"internal/listingkit/sheinadapter",
		"cmd/listing-scheduler", "cmd/shein-listing", "cmd/temu-listing",
		"cmd/listing-control-plane", "cmd/shein-import-platform-recovery",
		"internal/app/runtime/listingcontrol", "internal/app/runtime/sheinplatformrecovery", "internal/listingcontrol",
		"internal/app/task", "internal/app/taskstatus", "internal/processor", "internal/app/crawler/distributed",
		"internal/amazon/processor.go", "internal/amazon/task_status.go", "internal/amazon/task_status_test.go",
		"internal/crawler/fetcher/distributed_fetcher.go", "internal/crawler/fetcher/builder_test.go",
		"internal/temu/sync/inventory_sync_factory.go",
		"internal/shein/inventory", "internal/temu/sync", "internal/temu/bulkrelist",
		"internal/temu/category", "internal/temu/template",
		"internal/temu/product/save_handler.go", "internal/temu/product/submit_handler.go",
		"internal/temu/product/commit_create_handler.go", "internal/temu/product/commit_detail_handler.go",
		"internal/temu/product/save_publish_result_handler.go", "internal/temu/product/publish_result_steps.go",
		"internal/temu/product/publish_result_input.go", "internal/temu/product/publish_result_input_test.go",
		"internal/temu/product/request_models.go", "internal/temu/product/result_output.go",
		"internal/temu/product/submit_utils.go", "internal/temu/product/submit_error_analyzer.go",
		"internal/temu/product/submit_fixer.go", "internal/temu/product/submit_validator.go",
		"internal/temu/product/check_daily_limit_handler.go",
		"internal/temu/product/exists_check_handler.go", "internal/temu/product/out_goods_sn_check_handler.go",
		"internal/temu/product/raw_json_data_handler.go", "internal/temu/product/cache_handler.go",
		"internal/temu/product/build_spu_handler.go", "internal/temu/product/spu_builder.go",
		"internal/temu/product/spu_validator.go", "internal/temu/product/price_query_handler.go",
		"internal/temu/product/name_validator.go", "internal/temu/product/name_optimizer.go",
		"internal/temu/product/description_validator.go", "internal/temu/product/description_validator_rules.go",
		"internal/temu/product/description_validator_enhance.go", "internal/temu/product/description_validator_score.go",
		"internal/temu/product/brand_clear_handler.go", "internal/temu/product/name_validator_test.go",
		"internal/temu/product/brand_clear_handler_test.go",
		"internal/marketplace/temu/publishing/name_validation_policy.go", "internal/marketplace/temu/publishing/name_validation_policy_test.go",
		"internal/marketplace/temu/publishing/description_sanitization_policy.go", "internal/marketplace/temu/publishing/description_sanitization_policy_test.go",
		"internal/marketplace/temu/publishing/brand_text_policy.go", "internal/marketplace/temu/publishing/brand_text_policy_test.go",
		"internal/marketplace/temu/publishing/name_format_policy_test.go",
		"internal/temu/rules/text_check_handler.go", "internal/temu/rules/text_processor.go",
		"internal/temu/rules/bullet_points_validator.go", "internal/temu/rules/bullet_points_optimizer.go",
		"internal/temu/rules/bullet_points_validator_test.go",
		"internal/temu/property/mapper_adapter.go", "internal/temu/property/orchestrator.go",
		"internal/temu/property/pipeline.go", "internal/temu/property/mapping_stage.go",
		"internal/temu/property/validation_stage.go", "internal/temu/property/fixing_stage.go",
		"internal/temu/property/finalization_stage.go", "internal/temu/property/stage.go",
		"internal/temu/property/context.go", "internal/temu/property/cache.go",
		"internal/temu/property/feature_detector.go",
		"internal/marketplace/temu/publishing/submission_name_policy.go",
		"internal/marketplace/temu/publishing/submission_name_policy_test.go",
		"internal/marketplace/temu/publishing/name_format_policy.go",
		"internal/temu/sku/variant_json_data_handler.go", "internal/temu/sku/parallel_variant_handler.go",
		"internal/temu/sku/cache_variants_handler.go", "internal/temu/sku/variant_data_processor.go",
		"internal/temu/sku/variant_filter_handler.go", "internal/temu/sku/ai_mapping_handler.go",
		"internal/temu/sku/builder.go", "internal/temu/sku/item_builder.go", "internal/temu/sku/skc_builder.go",
		"internal/temu/sku/parallel_builder.go", "internal/temu/sku/build_input.go", "internal/temu/sku/price_calculator.go",
		"internal/temu/filter/rule_handler.go", "internal/temu/filter/rule_manager.go", "internal/temu/filter/rule_stats_provider.go",
		"internal/temu/product/filter_checker.go", "internal/temu/rules/rule_validator.go", "internal/temu/rules/rule_validator_test.go",
		"internal/temu/handlerbase/fulfillment_checker.go", "internal/temu/handlerbase/filter_types.go",
		"internal/temu/store", "internal/temu/handlerbase/init_handler.go",
		"internal/shein/pricing/auto_pricing.go", "internal/shein/pricing/pricing_evaluator.go",
		"internal/shein/pricing/pricing_calculator.go", "internal/shein/pricing/pricing_builder.go",
		"internal/shein/pricing/time_helper.go", "internal/shein/pricing/auto_pricing_repo_test.go",
		"internal/shein/pricing/time_helper_test.go",
		"internal/shein/activity/registration.go", "internal/shein/activity/mixed.go",
		"internal/shein/activity/profit.go", "internal/shein/activity/product_data_helper.go",
		"internal/shein/activity/config.go", "internal/shein/activity/config_test.go",
		"internal/shein/activity/repository_test.go",
		"internal/temu/pricing/auto_pricing_service.go", "internal/temu/pricing/pricing_decision_service.go",
		"internal/temu/pricing/pricing_rules.go", "internal/temu/pricing/pricing_data_service.go",
		"internal/temu/pricing/store_config_service.go", "internal/temu/pricing/runtime_adapter.go",
		"internal/temu/pricing/pricing_rules_test.go", "internal/temu/pricing/pricing_data_service_test.go",
		"internal/temu/pricing/store_config_service_test.go",
		"internal/shein/productsync/product_sync.go", "internal/shein/productsync/product_sync_fetcher.go",
		"internal/shein/productsync/product_sync_enricher.go", "internal/shein/productsync/product_snapshot.go",
		"internal/shein/productsync/product_snapshot_repository.go",
		"internal/shein/productsync/product_sync_repository_test.go", "internal/shein/productsync/product_sync_save_test.go",
		"internal/shein/productsync/product_snapshot_test.go", "internal/shein/productsync/product_snapshot_repository_test.go",
		"internal/listingadmin/inventory_record.go", "internal/listingadmin/inventory_record_repository.go",
		"internal/listingadmin/gorm_inventory_record_api.go", "internal/listingadmin/inventory_record_query_test.go",
		"internal/listingadmin/platform_recovery.go", "internal/listingadmin/platform_recovery_test.go",
		"internal/listingadmin/import_task.go", "internal/listingadmin/import_task_model.go",
		"internal/listingadmin/import_task_query.go", "internal/listingadmin/import_task_repository.go",
		"internal/listingadmin/dispatch_event_repository.go", "internal/listingadmin/paused_task_recovery.go",
		"internal/listingadmin/scheduled_task_config.go", "internal/listingadmin/scheduled_task_config_model.go",
		"internal/listingadmin/scheduled_task_config_query.go", "internal/listingadmin/scheduled_task_config_repository.go",
		"internal/listingadmin/store_statistics.go", "internal/listingadmin/runtime_prerequisites.go",
		"internal/listingadmin/import_task_dispatch_test.go", "internal/listingadmin/import_task_model_test.go",
		"internal/listingadmin/import_task_query_test.go", "internal/listingadmin/dispatch_event_repository_test.go",
		"internal/listingadmin/paused_task_recovery_test.go", "internal/listingadmin/scheduled_task_config_repository_test.go",
		"internal/listingadmin/store_statistics_test.go",
		"internal/app/runtime/listing", "internal/app/runtime/listingscheduler",
		"internal/app/bootstrap", "internal/app/consumer", "internal/app/runner", "internal/app/scheduler",
		"internal/listingruntime", "internal/listingkit/sheinsync", "internal/listingkit/store",
		"internal/platforms", "internal/shein/scheduler", "internal/temu/scheduler",
		"internal/platformtask", "internal/scheduler", "internal/shein/pipeline",
		"internal/platformbase/base_factory.go", "internal/platformbase/display.go", "internal/platformbase/task_type.go",
		"internal/app/httpapi/bootstrap.go", "internal/app/httpapi/image_agent_asset_catalog.go",
		"internal/pkg/httpapicmd", "cmd/product-listing-api", "scripts/listingkit-shein-pod-image-index-backfill",
	} {
		t.Run(relative, func(t *testing.T) {
			path := filepath.Join("..", filepath.FromSlash(relative))
			err := filepath.WalkDir(path, func(name string, entry os.DirEntry, err error) error {
				if os.IsNotExist(err) {
					return nil
				}
				if err != nil {
					return err
				}
				if !entry.IsDir() && strings.HasSuffix(name, ".go") {
					t.Errorf("retired Task backend source remains: %s", name)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	for relative, retired := range map[string][]string{
		"internal/app/runtime/listingkitschemamigrate/runtime_test.go": {
			"func openRuntimeSchemaTestDB(", "\"listing_product_import_task\"",
		},
	} {
		content, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range retired {
			if strings.Contains(string(content), token) {
				t.Errorf("%s must not recreate retired numeric task contract %q", relative, token)
			}
		}
	}
	for relative, required := range map[string][]string{
		"internal/marketplace/temu/goods/images.go": {
			"func ValidImageObservation(", "func resolveImages(", "inventory asset.ApprovedAssetInventory",
		},
		"internal/shared/ptr/pointer.go":          {"func IntPtr(", "func Float32Ptr("},
		"internal/storecenter/gorm_repository.go": {"type GormStoreRepository struct"},
		"internal/pkg/appenv/apputil.go":          {"func ComponentLoggers(", "func SetupLoggerWithLevel(", "type VersionInfo struct", "func PrintVersionInfo("},
		"internal/marketplace/temu/goods/builder.go": {
			"func Build(", "snapshot.SelectableLeaf", "func checkLocal(",
			"p.ServicePromise.ShipmentLimitDay", "value.ID == p.ServicePromise.CostTemplateID",
		},
		"internal/marketplace/temu/goods/supplier_category.go":                  {"func checkSupplierCategory(", "SelectableLeaf()"},
		"internal/marketplace/temu/rules/local_variations.go":                   {"func CheckLocalVariations("},
		"internal/marketplace/temu/goods/local_properties.go":                   {"func CheckLocalProperties("},
		"internal/marketplace/temu/goods/supplier_properties.go":                {"func CheckSupplierProperties("},
		"internal/marketplace/temu/goods/supplier_specs.go":                     {"func checkSupplierSpecifications("},
		"internal/marketplace/temu/rules/local_numbers.go":                      {"func (c *Catalog) CheckLocalNumbers("},
		"internal/marketplace/shein/publishing/attribute_value_match_policy.go": {"func MatchPlatformAttributeValue("},
		"internal/marketplace/shein/publishing/site_policy.go":                  {"func DefaultSiteListByRegion("},
		"internal/shein/content/cleanup_context.go":                             {"type CleanupContext struct", "type ValidationResult struct", "type SKCValidationMessages struct", "SourceBrand", "other_language_message_map", "otherLanguageMessageMap"},
		"internal/publishing/shein/submit_site_sku_policy.go":                   {"sheinmarketpub.DefaultSiteListByRegion("},
		"internal/publishing/shein/sensitive_content_sanitizer.go":              {"content.CleanupContext", "ensureSanitizerContext", "authorizedbrand.FromContext"},

		"internal/publishing/shein/submit_prep_sensitive_adapter.go": {"content.NewSensitiveWordServiceInMemory(logs)"},
	} {
		content, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range required {
			if !strings.Contains(strings.ReplaceAll(string(content), "\r\n", "\n"), token) {
				t.Errorf("%s must preserve retained platform/shared contract %q", relative, token)
			}
		}
	}
	productRoot := filepath.Join("..", "internal", "temu", "product")
	if err := filepath.WalkDir(filepath.Join("..", "internal"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !strings.HasPrefix(path, productRoot+string(os.PathSeparator)) {
			return nil
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, retired := range []string{
			"ProductSaveHandler", "ProductSubmitHandler", "CommitCreateHandler", "CommitDetailHandler", "SavePublishResultHandler", "CheckDailyLimitHandler",
			"ProductRequestInput", "CommitDetailOutput", "SaveProductOutput", "SubmitProductOutput", "SavePublishResultInput",
			"ProductSubmitUtils", "ProductSubmitErrorAnalyzer", "ProductSubmitFixer", "ProductSubmitValidator",
			"ProductExistsCheckHandler", "OutGoodsSnCheckHandler", "RawJsonDataHandlerV2", "CacheProductHandler",
		} {
			if strings.Contains(string(content), retired) {
				t.Errorf("%s must not recreate retired TEMU Task orchestration %q", path, retired)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	retiredTEMUSymbols := map[string]bool{
		"VariantJsonDataHandler": true, "NewVariantJsonDataHandler": true,
		"ParallelVariantHandler": true, "NewParallelVariantHandler": true,
		"CacheVariantsHandler": true, "NewCacheVariantsHandler": true,
		"VariantProcessor": true, "NewVariantProcessor": true, "extractAsinListFromMap": true,
		"VariantFilterHandler": true, "NewVariantFilterHandler": true,
		"AISkuMappingHandler": true, "NewAISkuMappingHandler": true,
		"FilterRuleHandler": true, "NewFilterRuleHandler": true,
		"FilterRuleManager": true, "NewFilterRuleManager": true,
		"FilterRuleStatsProvider": true, "NewFilterRuleStatsProvider": true,
		"ProductFilterChecker": true, "NewProductFilterChecker": true,
		"RuleValidator": true, "NewRuleValidator": true,
		"FulfillmentChecker": true, "NewFulfillmentChecker": true, "buildFulfillmentProductValue": true,
		"FilterCheckResult": true, "AIHandlerInput": true, "buildAIHandlerInput": true,
	}
	retiredProductSPUSymbols := map[string]bool{
		"BuildSpuHandler": true, "NewBuildSpuHandler": true,
		"SpuBuilder": true, "NewSpuBuilder": true,
		"SpuValidator": true, "NewSpuValidator": true,
		"PriceQueryHandler": true, "NewPriceQueryHandler": true,
	}
	retiredProductContentSymbols := map[string]bool{
		"ProductNameValidator": true, "NewProductNameValidator": true,
		"ProductNameOptimizer": true, "NewProductNameOptimizer": true,
		"ProductDescriptionValidator": true, "NewProductDescriptionValidator": true,
		"BrandClearHandler": true, "NewBrandClearHandler": true,
		"DescriptionValidationResult": true,
	}
	retiredRulesTextSymbols := map[string]bool{
		"TextCheckHandler": true, "NewTextCheckHandler": true,
		"TextProcessor": true, "NewTextProcessor": true,
		"BulletPointsValidator": true, "NewBulletPointsValidator": true,
		"BulletPointsOptimizer": true, "NewBulletPointsOptimizer": true,
		"BulletPointValidationResult": true, "OptimizationStrategy": true,
	}
	retiredPropertyPipelineSymbols := map[string]bool{
		"PropertyMapper": true, "NewPropertyMapper": true,
		"PropertyMappingOrchestrator": true, "NewPropertyMappingOrchestrator": true,
		"PropertyProcessingPipeline": true, "DefaultPropertyProcessingPipeline": true, "NewPropertyProcessingPipeline": true,
		"PropertyMappingStage": true, "NewPropertyMappingStage": true,
		"PropertyValidationStage": true, "NewPropertyValidationStage": true, "NewPropertyValidationStageWithEngine": true,
		"PropertyFixingStage": true, "NewPropertyFixingStage": true,
		"PropertyFinalizationStage": true, "NewPropertyFinalizationStage": true,
		"PropertyStage": true, "BasePropertyStage": true, "NewBasePropertyStage": true,
		"StageError": true, "NewStageError": true, "StageResult": true, "NewStageResult": true,
		"PropertyStageChain": true, "NewPropertyStageChain": true,
		"PropertyContext": true, "PropertyFeature": true, "ProcessingStats": true, "StageStats": true,
		"ProcessingConfig": true, "NewPropertyContext": true, "NewProcessingStats": true, "NewDefaultProcessingConfig": true,
		"PropertyCache": true, "InMemoryPropertyCache": true, "CacheItem": true,
		"NewPropertyCache": true, "NewPropertyCacheWithTTL": true, "GenerateFeatureCacheKey": true, "CacheStats": true,
		"PropertyFeatureDetector": true, "NewPropertyFeatureDetector": true,
	}
	for _, pkg := range []string{"sku", "filter", "product", "rules", "handlerbase", "property"} {
		root := filepath.Join("..", "internal", "temu", pkg)
		if err := filepath.WalkDir(filepath.Join("..", "internal"), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !strings.HasPrefix(path, root+string(os.PathSeparator)) {
				return nil
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			if pkg == "sku" {
				for _, violation := range retiredSKUConstructionViolations(file) {
					t.Errorf("%s: %s", path, violation)
				}
			}
			if pkg == "product" || pkg == "rules" || pkg == "property" {
				for _, declaration := range file.Decls {
					var names []*ast.Ident
					switch declaration := declaration.(type) {
					case *ast.FuncDecl:
						if declaration.Recv == nil {
							names = append(names, declaration.Name)
						}
					case *ast.GenDecl:
						for _, spec := range declaration.Specs {
							switch spec := spec.(type) {
							case *ast.TypeSpec:
								names = append(names, spec.Name)
							case *ast.ValueSpec:
								names = append(names, spec.Names...)
							}
						}
					}
					for _, name := range names {
						if (pkg == "product" && retiredProductContentSymbols[name.Name]) ||
							(pkg == "rules" && retiredRulesTextSymbols[name.Name]) ||
							(pkg == "property" && retiredPropertyPipelineSymbols[name.Name]) {
							t.Errorf("%s recreates retired TEMU content declaration %s", path, name.Name)
						}
					}
				}
			}
			if pkg == "handlerbase" {
				for _, declaration := range file.Decls {
					if group, ok := declaration.(*ast.GenDecl); ok {
						for _, spec := range group.Specs {
							if typ, ok := spec.(*ast.TypeSpec); ok && typ.Name.Name == "ImageRequirement" {
								t.Errorf("%s must not recreate retired handlerbase.ImageRequirement", path)
							}
						}
					}
				}
			}
			ast.Inspect(file, func(node ast.Node) bool {
				ident, ok := node.(*ast.Ident)
				if !ok {
					return true
				}
				retiredInit := pkg == "handlerbase" && (ident.Name == "InitDataHandler" || ident.Name == "NewInitDataHandler")
				retiredSPU := pkg == "product" && retiredProductSPUSymbols[ident.Name]
				if retiredTEMUSymbols[ident.Name] || retiredInit || retiredSPU {
					t.Errorf("%s must not recreate retired TEMU Task orchestration %q", path, ident.Name)
				}
				return true
			})
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	publishingPaths, err := filepath.Glob(filepath.Join("..", "internal", "marketplace", "temu", "publishing", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	retiredPublishingContentSymbols := map[string]bool{
		"SanitizeProductName": true, "ProductNameSanitizationResult": true,
		"SanitizeProductDescription": true, "ProductDescriptionSanitizationResult": true,
		"RemoveBrandFromText": true, "NormalizeParenthesesSpacing": true,
		"NormalizeProductSubmissionName": true,
	}
	for _, path := range publishingPaths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			var names []*ast.Ident
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				if declaration.Recv == nil {
					names = append(names, declaration.Name)
				}
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						names = append(names, spec.Name)
					case *ast.ValueSpec:
						names = append(names, spec.Names...)
					}
				}
			}
			for _, name := range names {
				if retiredPublishingContentSymbols[name.Name] {
					t.Errorf("%s recreates retired TEMU publishing content declaration %s", path, name.Name)
				}
			}
		}
	}
	// The complete strx owner, including its retired text declarations, is
	// protected by TestLegacyRedisProxyAndSharedClosureIsRetired.
	contextPaths, err := filepath.Glob(filepath.Join("..", "internal", "temu", "context", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	retiredFields := map[string]bool{
		"SaveResult": true, "SavedToDraft": true, "CommitDetail": true, "SubmitResponse": true, "ProductData": true,
		"DailyQuotaReserved": true, "DailyQuotaDate": true, "DailyQuotaIncrement": true,
	}
	retiredMethods := map[string]bool{
		"SetPublishProductData": true, "SetSaveResponse": true, "SetCommitDetailResponse": true, "SetSubmitResponse": true,
		"SetSavedToDraft": true, "SetDailyQuotaReservation": true, "ClearDailyQuotaReservation": true, "ApplySaveResult": true,
	}
	retiredAssetFields := map[string]bool{
		"CurrentSkuContext": true, "RequiresImageUpload": true, "TotalImageCount": true,
		"PaddedImages": true, "PaddedImageSizes": true,
	}
	for _, contextPath := range contextPaths {
		if strings.HasSuffix(contextPath, "_test.go") {
			continue
		}
		contextFile, err := parser.ParseFile(token.NewFileSet(), contextPath, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		// Inspect package owners, including aliases in new context files, without
		// banning unrelated selectors or local names in other domains.
		for _, declaration := range contextFile.Decls {
			switch declaration := declaration.(type) {
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					typ, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if typ.Name.Name == "AssetState" {
						t.Errorf("%s must not recreate retired context.AssetState", contextPath)
					}
					state, ok := typ.Type.(*ast.StructType)
					if !ok {
						continue
					}
					for _, field := range state.Fields.List {
						if len(field.Names) == 0 {
							embedded := field.Type
							if pointer, ok := embedded.(*ast.StarExpr); ok {
								embedded = pointer.X
							}
							if name, ok := embedded.(*ast.Ident); ok && name.Name == "AssetState" {
								t.Errorf("%s must not embed retired context.AssetState", contextPath)
							}
						}
						for _, name := range field.Names {
							if (typ.Name.Name == "PublishState" || typ.Name.Name == "TemuTaskContext") && retiredFields[name.Name] {
								t.Errorf("TEMU %s must not retain exclusive publish field %s", typ.Name, name)
							}
							if retiredAssetFields[name.Name] || name.Name == "AssetState" {
								t.Errorf("TEMU %s must not retain exclusive image field %s", typ.Name, name)
							}
						}
					}
				}
			case *ast.FuncDecl:
				if declaration.Recv != nil && retiredMethods[declaration.Name.Name] {
					t.Errorf("TEMU context must not retain exclusive publish method %s", declaration.Name)
				}
			}
		}
		ast.Inspect(contextFile, func(node ast.Node) bool {
			literal, ok := node.(*ast.CompositeLit)
			if !ok {
				return true
			}
			owner, ok := literal.Type.(*ast.Ident)
			if !ok || (owner.Name != "TemuTaskContext" && owner.Name != "AssetState") {
				return true
			}
			for _, element := range literal.Elts {
				if entry, ok := element.(*ast.KeyValueExpr); ok {
					if key, ok := entry.Key.(*ast.Ident); ok && (key.Name == "AssetState" || retiredAssetFields[key.Name]) {
						t.Errorf("%s must not initialize exclusive image field %s", contextPath, key.Name)
					}
				}
			}
			return true
		})
	}
	policyIndex, err := loadGoFileIndex(filepath.Join("..", "internal", "marketplace", "temu"), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, policyName := range []string{"submission_validation_policy.go", "sku_value_policy.go"} {
		for policyPath := range policyIndex.files {
			if policyPath != filepath.Join("..", "internal", "marketplace", "temu", "publishing", policyName) {
				continue
			}
			policyFile, err := parser.ParseFile(token.NewFileSet(), policyPath, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, imported := range policyFile.Imports {
				if imported.Path.Value != "\"fmt\"" && imported.Path.Value != "\"strings\"" {
					t.Errorf("submission validation must remain a value-only rule, found import %s", imported.Path.Value)
				}
			}
			for _, declaration := range policyFile.Decls {
				if function, ok := declaration.(*ast.FuncDecl); ok && function.Name.IsExported() {
					t.Errorf("submission extraction must not add exported contract %s", function.Name)
				}
				if group, ok := declaration.(*ast.GenDecl); ok {
					for _, spec := range group.Specs {
						if typ, ok := spec.(*ast.TypeSpec); ok && typ.Name.IsExported() {
							t.Errorf("submission extraction must not add exported type %s", typ.Name)
						}
					}
				}
			}
		}
	}
	for _, platform := range []string{"shein", "temu"} {
		root := filepath.Join("..", "internal", platform, "pricing")
		index, err := loadGoFileIndex(filepath.Join("..", "internal"), "")
		if err != nil {
			t.Fatal(err)
		}
		for path := range index.files {
			if filepath.Dir(path) != root || strings.HasSuffix(path, "_test.go") {
				continue
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, retired := range []string{
				"type AutoPricingService ", "NewAutoPricingService", "autoPricingServiceImpl",
				"type PricingDecisionService ", "type PricingRuntime ", "NewPricingRuntime",
				"type PricingDataService ", "type StoreConfigService ", "type PricingRequestBuilder ", "type TimeHelper ",
				"type StoreConfigProvider interface", "type ProductDataProvider interface", "type ProductFetcher interface",
				"type DecisionMaker interface", "type PricingProcessor interface",
			} {
				if strings.Contains(string(content), retired) {
					t.Errorf("%s must not recreate retired automatic pricing %q", path, retired)
				}
			}
		}
	}
}

func TestCurrentCommandsDoNotCompileLegacyBusinessBackends(t *testing.T) {
	if _, err := os.Lstat(filepath.Join("..", "internal", "listingruntime", "types.go")); !os.IsNotExist(err) {
		t.Errorf("retired numeric mapping contract types.go must be absent: %v", err)
	}
	assertLegacyPlatformAPICallClosureRetired(t)
	if _, err := os.Stat(filepath.Join("..", "internal", "core", "config", "type_platform.go")); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{".github/workflows/ci.yml", ".github/workflows/commercial-readiness.yml"} {
		content, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "go test -race ./internal/listingadmin -count=1") {
			t.Errorf("%s must run the retained listingadmin tests with the race detector", relative)
		}
		for _, retired := range []string{
			"TestConcurrentClaimForDispatchOnlyOneWorkerWins",
			"TestConcurrentRollbackDispatchOnlyOriginalQueuedClaimIsRestoredOnce",
			"TestConcurrentRecoveryOnlyUpdatesStillEligibleRowsOnce",
		} {
			if strings.Contains(string(content), retired) {
				t.Errorf("%s must not select retired task test %q", relative, retired)
			}
		}
	}
	for _, command := range []string{"current-application", "shein-login-worker", "image-agent-temporal-worker", "source-account-registry-schema-init", "product-listing-api-schema-migrate", "1688-local-agent", "1688-batch-import", "1688-public-browser-collector"} {
		t.Run(command, func(t *testing.T) {
			cmd := exec.Command("go", "list", "-deps", "./cmd/"+command)
			cmd.Dir = ".."
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go list current command: %v\n%s", err, output)
			}
			for _, dependency := range strings.Fields(string(output)) {
				if dependency == "task-processor/internal/shein/api" {
					t.Errorf("%s compiles retired root SHEIN API", command)
				}
				for _, root := range []string{"internal/amazon/api", "internal/temu/api", "internal/shein/api/category", "internal/shein/api/image", "internal/shein/api/marketing", "internal/shein/api/pricing", "internal/shein/api/translate", "internal/shein/pricing"} {
					if importMatchesPrefix(dependency, "task-processor/"+root) {
						t.Errorf("%s compiles retired platform API %s", command, dependency)
					}
				}

				if dependency == "task-processor/internal/shein" {
					t.Errorf("%s still compiles retired root SHEIN contract %s", command, dependency)
				}
				for _, relative := range retiredSheinPolicyContractPackages {
					if importMatchesPrefix(dependency, "task-processor/"+relative) {
						t.Errorf("%s still compiles retired SHEIN policy/contract %s", command, dependency)
					}
				}
				for _, relative := range append(append(append([]string{}, retiredTEMUListingPackages...), retiredAmazonListingPackages...), retiredHostRuntimePackages...) {
					retired := "task-processor/" + relative
					if dependency == retired || strings.HasPrefix(dependency, retired+"/") {
						t.Errorf("%s still compiles retired listing dependency %s", command, dependency)
					}
				}
				for _, retired := range []string{"task-processor/internal/prompt", "task-processor/internal/promptmgmt", "task-processor/internal/shared/tenantctx", "task-processor/internal/platformbase", "task-processor/internal/crawler/fetcher", "task-processor/internal/marketplace/sourceproduct", "task-processor/internal/domain/task", "task-processor/internal/crawler/shared", "task-processor/internal/shein/product", "task-processor/internal/shein/productdata", "task-processor/internal/shein/publish", "task-processor/internal/shein/mapping", "task-processor/internal/shein/validation", "task-processor/internal/shein/translate", "task-processor/internal/shein/addresscopy", "task-processor/internal/shein/managedclient", "task-processor/internal/shein/store", "task-processor/internal/shein/context", "task-processor/internal/shein/languageconfig", "task-processor/internal/shein/siteconfig", "task-processor/internal/sheinbridge/saleattribute", "task-processor/internal/shein/inventory", "task-processor/internal/temu/sync", "task-processor/internal/temu/bulkrelist", "task-processor/internal/temu/store", "task-processor/internal/app/bootstrap", "task-processor/internal/app/consumer", "task-processor/internal/app/runner", "task-processor/internal/app/scheduler", "task-processor/internal/app/runtime/listingcontrol", "task-processor/internal/app/runtime/sheinplatformrecovery", "task-processor/internal/listingcontrol", "task-processor/internal/app/task", "task-processor/internal/app/taskstatus", "task-processor/internal/processor", "task-processor/internal/app/crawler/distributed", "task-processor/internal/listingruntime", "task-processor/internal/listingruntime/local", "task-processor/internal/platforms", "task-processor/internal/listingkit", "task-processor/internal/tenantbridge", "task-processor/internal/compatibility", "task-processor/internal/sourceaccount", "task-processor/internal/integration/persistence/sourceaccount/ownershipmigration", "task-processor/internal/taskrpcapi", "task-processor/internal/workspace/shein", "task-processor/internal/core/metrics", "task-processor/internal/publishing/sheinmanaged", "task-processor/internal/shein/category", "task-processor/internal/shein/aicache"} {
					if dependency == retired || (retired != "task-processor/internal/crawler/shared" && strings.HasPrefix(dependency, retired+"/")) {
						t.Errorf("%s still compiles retired business dependency %s", command, dependency)
					}
				}
			}
		})
	}
}

func TestStandaloneSheinWorkerDoesNotCompileCurrentHTTPApplication(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "./cmd/shein-login-worker")
	cmd.Dir = ".."
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list standalone SHEIN worker: %v\n%s", err, output)
	}
	for _, dependency := range strings.Fields(string(output)) {
		if dependency == "task-processor/internal/sourceaccount" || strings.HasPrefix(dependency, "task-processor/internal/sourceaccount/") {
			t.Errorf("standalone SHEIN worker still compiles retired source-account persistence %s", dependency)
		}
		if dependency == "task-processor/internal/app/httpapi" {
			t.Fatal("standalone SHEIN worker must use its own command runtime, not the current HTTP application assembly")
		}
		if dependency == "task-processor/internal/tenantbridge" || strings.HasPrefix(dependency, "task-processor/internal/tenantbridge/") {
			t.Errorf("standalone SHEIN worker still compiles legacy numeric tenant transport %s", dependency)
		}
	}
	for _, name := range []string{"shein_login_worker.go", "shein_login_worker_test.go"} {
		if _, err := os.Stat(filepath.Join("..", "internal", "app", "httpapi", name)); !os.IsNotExist(err) {
			t.Fatalf("shared HTTP worker source %s must be retired, without a forwarding wrapper", name)
		}
	}
	for _, name := range []string{"api.go", "api_test.go", "http_module.go", "http_module_test.go", "tenant_context.go", "tenant_context_test.go"} {
		if _, err := os.Stat(filepath.Join("..", "internal", "sheinlogin", name)); !os.IsNotExist(err) {
			t.Errorf("unused SHEIN login HTTP source %s must be retired; the standalone worker retains its service and health endpoint", name)
		}
	}
}

func TestLegacyReleaseIdentityPreflightIsRetired(t *testing.T) {
	for _, relative := range []string{
		"cmd/listingkit-identity-preflight", "internal/app/runtime/listingkitidentitypreflight",
		"internal/listingkit/identitypreflight", "internal/listingkit/ownerreconcile",
		"internal/listingkit/userdirectory", "internal/tenantbridge",
		"deployments/docker/Dockerfile.listingkit-identity-preflight",
		"deployments/kubernetes/listingkit-workbench/jobs/listingkit-identity-preflight-job.yaml",
		"scripts/listingkit-identity-preflight-job.sh", "scripts/tests/listingkit-identity-preflight-job-test.sh",
		"scripts/listingkit-clean-legacy-identity-secret.sh", "scripts/tests/listingkit-clean-legacy-identity-secret-test.sh",
		"scripts/migrate-yudao-users-to-zitadel.ps1",
		"scripts/migrate-yudao-users-to-local-zitadel.ps1",
		"scripts/migrate-yudao-all-users-to-local-zitadel.ps1",
		"scripts/yudao-zitadel-owner-recovery-dry-run.ps1",
	} {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired identity-preflight artifact remains: %s (stat: %v)", relative, err)
		}
	}
	for _, relative := range []string{
		"deployments/docker/Dockerfile.product-listing-api",
		"deployments/kubernetes/listingkit-workbench/release-authority/release-policy.yaml",
		"deployments/kubernetes/listingkit-workbench/release-authority/listingkit-release-gate-runners.yaml",
	} {
		content, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), "listingkit-identity-preflight") {
			t.Errorf("%s still packages or authorizes retired identity preflight", relative)
		}
	}
}

func TestListingAdminLegacyHTTPProjectionIsRetired(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "internal", "listingadmin", "*handler*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("retired ListingAdmin HTTP projection remains: %v", files)
	}
	for _, name := range []string{"request_context.go", "list_query_scope.go"} {
		if _, err := os.Stat(filepath.Join("..", "internal", "listingadmin", name)); !os.IsNotExist(err) {
			t.Fatalf("retired ListingAdmin HTTP helper %s must be absent", name)
		}
	}
}

func TestCurrentBuildDoesNotPackageRetiredTaskEntrypoints(t *testing.T) {
	for _, relative := range []string{"Makefile", "deployments/docker/Dockerfile.product-listing-api", "scripts/test-fast.ps1"} {
		contents, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		for _, retired := range []string{"./cmd/product-listing-api ", "./cmd/product-listing-api\"", "cmd/product-listing-api/", "./internal/listingkit ", "./internal/listingkit\"", "listingkit-shein-pod-image-index-backfill"} {
			if strings.Contains(string(contents), retired) {
				t.Errorf("%s still builds or packages retired entrypoint %q", relative, retired)
			}
		}
	}
}

func TestRetiredLocalLaunchersRejectBeforeOperations(t *testing.T) {
	for _, relative := range []string{
		"scripts/start-listingkit-local-api.ps1",
		"scripts/start-listingkit-local-api.Tests.ps1",
		"scripts/start-listingkit-local-dev.ps1",
		"scripts/start-listingkit-local-dev.Tests.ps1",
		"scripts/image-agent-local-acceptance.ps1",
		"scripts/image-agent-local-acceptance.Tests.ps1",
		"scripts/tests/image-agent-local-acceptance-script-test.ps1",
		"deployments/docker/image-agent-acceptance/docker-compose.yml",
		"scripts/listingkit-fetch-machine-token.ps1",
		"scripts/listingkit-save-token.ps1",
		"scripts/listingkit-auth-check.ps1",
	} {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired local launch/acceptance source must be absent: %s (stat: %v)", relative, err)
		}
	}
	if _, err := os.Stat(filepath.Join("..", "scripts", "start-listingkit-api-local-replay.ps1")); !os.IsNotExist(err) {
		t.Fatal("dedicated retired API replay entry must be absent")
	}
}

func TestRetiredListingCLIsHaveNoBuildOrDeploymentEntrypoints(t *testing.T) {
	for _, relative := range []string{
		"deployments/docker/Dockerfile.listing",
		"deployments/kubernetes/shein-listing/base/deployment.yaml",
		"deployments/kubernetes/temu-listing/base/deployment.yaml",
		"deployments/kubernetes/shein-listing/overlays/prod-auto-shard-statefulset/statefulset.yaml",
		"deployments/kubernetes/shein-listing/overlays/prod-auto-shard-statefulset/listing-scheduler.yaml",
		"deployments/kubernetes/shein-listing/templates/single-store-deployment.yaml.tpl",
		"deployments/kubernetes/shein-listing/stores.single-pod.json",
		"scripts/build-push-listing-scheduler.ps1", "scripts/build-push-deploy-shein-listing.ps1",
		"scripts/deploy-single-store-listing.ps1", "scripts/sync-single-store-listings.ps1",
		"scripts/rollout-shein-shard-statefulset.ps1",
		"deployments/docker/Dockerfile.listing-control-plane",
		"scripts/build-push-deploy-listing-control-plane.ps1", "scripts/shein-import-platform-recovery.ps1",
		"deployments/kubernetes/shein-listing/overlays/prod-auto-shard-statefulset/listing-control-plane.yaml",
		"deployments/kubernetes/shein-listing/overlays/prod-auto-shard-statefulset/ownership-controller.yaml",
		"deployments/kubernetes/shein-listing/overlays/prod-auto-shard-statefulset/kustomization.yaml",
		"deployments/kubernetes/shein-listing/overlays/prod-store-auto-shard/configmap-heavy.yaml",
		"deployments/kubernetes/shein-listing/overlays/prod-store-auto-shard/configmap-lite.yaml",
		"deployments/kubernetes/shein-listing/overlays/prod-store-auto-shard/kustomization.yaml",
	} {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired CLI operation entrypoint remains: %s (stat: %v)", relative, err)
		}
	}
	for _, relative := range []string{"Makefile", "scripts/test-fast.ps1", ".github/workflows/ci.yml", ".github/workflows/commercial-readiness.yml"} {
		content, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		for _, command := range []string{"listing-scheduler", "shein-listing", "temu-listing", "listing-control-plane", "shein-import-platform-recovery"} {
			if strings.Contains(string(content), "./cmd/"+command) {
				t.Errorf("%s still invokes retired CLI %s", relative, command)
			}
		}
		for _, retired := range []string{"./internal/app/runtime/listingcontrol", "./internal/app/runtime/sheinplatformrecovery", "./internal/listingcontrol", "listing-control-plane.yaml"} {
			if strings.Contains(string(content), retired) {
				t.Errorf("%s still references retired runtime or workload %s", relative, retired)
			}
		}
	}
	content, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"current-application", "shein-login-worker"} {
		if !strings.Contains(string(content), "go build ./cmd/"+command) {
			t.Errorf("CI must compile retained command %s", command)
		}
	}
	for _, relative := range []string{
		"internal/core/config/type_rabbitmq.go",
		"internal/core/config/config.go",
	} {
		content, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		for _, retired := range []string{"AutoShard", "rabbitmq.autoShard", "RABBITMQ_AUTO_SHARD_"} {
			if strings.Contains(string(content), retired) {
				t.Errorf("%s still declares or loads retired auto-shard configuration %s", relative, retired)
			}
		}
	}
}

func TestTemuImageAPIAndVisionResourcesAreRetired(t *testing.T) {
	apiPaths, err := filepath.Glob(filepath.Join("..", "internal", "temu", "api", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	retiredNames := map[string]bool{
		"ImageAPI": true, "NewImageAPI": true, "UploadSignature": true,
		"SignatureResponse": true, "UploadResult": true, "ValidationResult": true,
	}
	for _, path := range apiPaths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		// Only package declarations recreate the retired root API contract;
		// selectors, fields, methods and local names can belong to retained APIs.
		for _, declaration := range file.Decls {
			var names []*ast.Ident
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				if declaration.Recv == nil {
					names = append(names, declaration.Name)
				}
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						names = append(names, spec.Name)
					case *ast.ValueSpec:
						names = append(names, spec.Names...)
					}
				}
			}
			for _, name := range names {
				if retiredNames[name.Name] {
					t.Errorf("%s recreates retired TEMU root API declaration %s", path, name.Name)
				}
			}
		}
	}
	for _, relative := range []string{"internal/prompt", "internal/promptmgmt", "internal/shared/tenantctx", "prompts"} {
		if _, err := os.Lstat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired tenant Prompt root %s must be absent: %v", relative, err)
		}
	}
	for _, relative := range trackedFiles(t, ".") {
		if !strings.HasSuffix(relative, ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", filepath.FromSlash(relative)), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			imported, err := decodeGoImportPath(spec.Path.Value)
			if err != nil {
				t.Fatalf("decode import in %s: %v", relative, err)
			}
			for _, retired := range []string{"internal/prompt", "internal/promptmgmt", "internal/shared/tenantctx"} {
				if importMatchesPrefix(imported, "task-processor/"+retired) {
					t.Errorf("%s imports retired tenant Prompt owner %s", relative, imported)
				}
			}
		}
	}
}

func TestNumericStoreManagementClosureIsRetired(t *testing.T) {
	retiredMethods := strings.Fields("CreateStore UpdateStore DeleteStore ListDeletedStores RestoreStore PermanentlyDeleteStore ExtendStoreValidity")
	retiredHelpers := strings.Fields("listingStoreFromStore applyStoreCreateDefaults findStoreRows parseTenantID deleteStoreAccessRow")
	configurationFields := strings.Fields("DailyLimit DailyLimitType FixedStockCount SKUGenerateStrategy Prefix Suffix EnableAutoListing EnableAutoLogin EnableDraft EnableAutoPrice EnableRebargain EnableBrandAuthorization AuthorizedBrandCode AuthorizedBrandName TemuPriceRejectStrategy PriceType DedicatedQueueEnabled")
	retiredFields := map[string][]string{
		"Store": configurationFields, "listingStore": configurationFields,
		"StoreRespDTO":    append([]string{"SkuGenerateStrategy"}, append(append([]string{}, configurationFields[:3]...), configurationFields[4:]...)...),
		"StoreQuery":      strings.Fields("StoreID Name Username ShopType Region SKUGenerate EnableAutoListing EnableAutoLogin EnableDraft EnableAutoPrice EnableRebargain PriceType Status Expired Deleted"),
		"StorePageReqDTO": {"EnableAutoPrice"},
	}
	seen := map[string]bool{}
	paths, err := filepath.Glob(filepath.Join("..", "internal", "listingadmin", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("retained listingadmin package is absent")
	}
	contains := func(names []string, name string) bool {
		for _, candidate := range names {
			if candidate == name {
				return true
			}
		}
		return false
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				if declaration.Recv == nil {
					if contains(retiredHelpers, declaration.Name.Name) {
						t.Errorf("%s retains retired Store helper %s", path, declaration.Name.Name)
					}
					if declaration.Name.Name == "takeStoreAccessRow" || declaration.Name.Name == "updateStoreAccessRow" {
						seen[declaration.Name.Name] = true
						for _, parameter := range declaration.Type.Params.List {
							for _, name := range parameter.Names {
								if name.Name == "deleted" {
									t.Errorf("%s retains recyclable-row parameter on %s", path, declaration.Name.Name)
								}
							}
						}
					}
					continue
				}
				if len(declaration.Recv.List) != 1 {
					continue
				}
				receiver := declaration.Recv.List[0].Type
				if pointer, ok := receiver.(*ast.StarExpr); ok {
					receiver = pointer.X
				}
				if owner, ok := receiver.(*ast.Ident); ok && owner.Name == "GormStoreRepository" && contains(retiredMethods, declaration.Name.Name) {
					t.Errorf("%s retains retired Store method %s.%s", path, owner.Name, declaration.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					if binding, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range binding.Names {
							if contains(retiredHelpers, name.Name) {
								t.Errorf("%s recreates retired Store helper binding %s", path, name.Name)
							}
						}
					}
					typ, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if typ.Name.Name == "StoreRepository" {
						seen[typ.Name.Name] = true
						contract, ok := typ.Type.(*ast.InterfaceType)
						if !ok {
							t.Fatal("StoreRepository must remain an explicit interface")
						}
						for _, method := range contract.Methods.List {
							for _, name := range method.Names {
								if contains(retiredMethods, name.Name) {
									t.Errorf("StoreRepository retains retired management method %s", name.Name)
								}
							}
							if len(method.Names) == 0 {
								t.Error("StoreRepository must not embed a second management contract")
							}
						}
					}
					fields, scoped := retiredFields[typ.Name.Name]
					if !scoped {
						continue
					}
					seen[typ.Name.Name] = true
					shape, ok := typ.Type.(*ast.StructType)
					if !ok {
						t.Fatalf("%s must remain an explicit value struct", typ.Name.Name)
					}
					for _, field := range shape.Fields.List {
						for _, name := range field.Names {
							if contains(fields, name.Name) {
								t.Errorf("%s retains retired Store field %s.%s", path, typ.Name.Name, name.Name)
							}
						}
						if len(field.Names) == 0 {
							t.Errorf("%s must not embed retired configuration", typ.Name.Name)
						}
					}
				}
			}
		}
	}
	// #863 supersedes the earlier numeric Store retention boundary.
	for _, name := range []string{"StoreRepository", "Store", "listingStore", "StoreRespDTO", "StoreQuery", "StorePageReqDTO", "takeStoreAccessRow", "updateStoreAccessRow"} {
		if seen[name] {
			t.Errorf("retired Store declaration remains: %s", name)
		}
	}
}

func TestSKURetirementGuardPreservesSharedNames(t *testing.T) {
	for _, tt := range []struct {
		name, source string
		wantRetired  bool
	}{
		{name: "retained selector", source: `package sku; import handlerbase "task-processor/internal/temu/handlerbase"; var _ handlerbase.SkuBuilder`},
		{name: "retained alias with current name", source: `package sku; import handlerbase "task-processor/internal/temu/handlerbase"; type CurrentBuilder = handlerbase.SkuBuilder`},
		{name: "local names and fields", source: `package sku; type Metadata struct { SpecValue string }; func retained(SkuBuilder int) { SpecValue := SkuBuilder; _ = SpecValue; type SKUProcessInput struct{}; _ = SKUProcessInput{}; type SkuVariantProcessor struct { itemBuilder int }; _ = SkuVariantProcessor{} }`},
		{name: "concrete type", source: `package sku; type SkuBuilder struct{}`, wantRetired: true},
		{name: "retired alias", source: `package sku; type SkuBuilder = CurrentBuilder`, wantRetired: true},
		{name: "constructor", source: `package sku; func NewSkuBuilder() {}`, wantRetired: true},
		{name: "constructor binding", source: `package sku; var NewSkuBuilder = factory`, wantRetired: true},
		{name: "retired method", source: `package sku; func (*SkuVariantProcessor) BuildVariantSkcs() {}`, wantRetired: true},
		{name: "retired field", source: `package sku; type SkuVariantProcessor struct { itemBuilder any }`, wantRetired: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "sku.go", tt.source, 0)
			if err != nil {
				t.Fatal(err)
			}
			violations := retiredSKUConstructionViolations(file)
			if (len(violations) != 0) != tt.wantRetired {
				t.Fatalf("retirement violations = %v, wantRetired=%v", violations, tt.wantRetired)
			}
		})
	}
}

func retiredSKUConstructionViolations(file *ast.File) []string {
	retiredSKUSymbols := map[string]bool{
		"SkuBuilder": true, "NewSkuBuilder": true, "SkuItemBuilder": true, "NewSkuItemBuilder": true,
		"SkuSkcBuilder": true, "NewSkuSkcBuilder": true, "SkuParallelBuilder": true, "NewSkuParallelBuilder": true,
		"SKUVariantBuildInput": true, "SKUProcessInput": true, "buildSKUVariantBuildInputWithRuntime": true,
		"buildSKUVariantBuildInput": true, "buildSKUProcessInput": true,
		"SkuPriceCalculator": true, "NewSkuPriceCalculator": true,
		"skuPackagingInfo": true, "skuExpressInfo": true, "skuPricingInfo": true, "SpecDimension": true, "SpecValue": true,
	}
	retiredSKUMethods := map[string]map[string]bool{
		"SkuVariantProcessor": {
			"BuildVariantSkcs": true, "resolveVariantAIMapping": true, "resolveAndBuildVariantSkcs": true,
			"assignBuiltVariantSkcs": true, "BuildSkcsFromAIMapping": true, "prepareAndBuildVariantSkcs": true,
			"buildSkcsFromPreparedAIMapping": true, "resolveSkcBuildSpecs": true, "buildMultipleSkcs": true, "buildSingleSkc": true,
			"CreateDefaultSkc": true, "resolveDefaultBuildProduct": true, "resolveAndBuildDefaultSkc": true,
			"prepareAndBuildDefaultSkc": true, "buildDefaultSkcFromPreparedMapping": true, "selectDefaultAIMapping": true,
			"buildDefaultSkuFromAIMapping": true, "resolveDefaultAIMapping": true, "prepareDefaultAIMappingForBuild": true,
		},
		"SkuSpecHandler": {
			"collectNonColorSpecsForColor": true, "generateNonColorSpecCombinations": true, "generateCartesianProduct": true,
			"createSpecCombinationKey": true, "createNonColorSpecKey": true, "createSpecCombinationKeyFromSpecs": true, "isColorSpec": true,
		},
	}
	var violations []string
	checkName := func(name string) {
		if retiredSKUSymbols[name] {
			violations = append(violations, "retired SKU declaration "+name)
		}
	}
	// Inspect package declarations only: retained selectors and local names do
	// not recreate the retired concrete package owner.
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			if declaration.Recv == nil {
				checkName(declaration.Name.Name)
				continue
			}
			if len(declaration.Recv.List) != 1 {
				continue
			}
			receiver := declaration.Recv.List[0].Type
			if pointer, ok := receiver.(*ast.StarExpr); ok {
				receiver = pointer.X
			}
			if owner, ok := receiver.(*ast.Ident); ok && retiredSKUMethods[owner.Name][declaration.Name.Name] {
				violations = append(violations, "retired SKU method "+owner.Name+"."+declaration.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range declaration.Specs {
				switch spec := spec.(type) {
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						checkName(name.Name)
					}
				case *ast.TypeSpec:
					checkName(spec.Name.Name)
					if spec.Name.Name != "SkuVariantProcessor" {
						continue
					}
					if state, ok := spec.Type.(*ast.StructType); ok {
						for _, field := range state.Fields.List {
							for _, name := range field.Names {
								if name.Name == "skcBuilder" || name.Name == "itemBuilder" {
									violations = append(violations, "exclusive SKU construction field "+name.Name)
								}
							}
						}
					}
				}
			}
		}
	}
	return violations
}

func TestSheinListingMixedDeclarationsAreRetired(t *testing.T) {
	for relative, retired := range map[string][]string{
		"internal/shein":          {"ImageUploadResult", "AttributeImportance", "EnrichedSkuInfo", "EnrichedSkcInfo", "InventoryInfo", "SkuInfo", "MappingInfo", "CostPriceInfo", "Task", "TaskContext", "StepHandler", "VariantFilterInfo", "PreValidResult", "SkcErrorMessage", "NewTaskContext", "AttributeValue", "AttributePriorityConfig", "AttributeValueIDManager", "ResultSaleAttribute", "VariantInfo", "Variant", "AttributeImportanceCalculator", "AttributeMetadata", "AttributeConfig", "ImportanceRules", "ResultAttribute", "SaleAttribute", "AttributeData", "AttributeImportanceResult", "GenerationRequest", "AttributeNameMapping", "ProductVariantData", "GenerateAttributeValue", "GenerateAttribute", "BuildAttributeInfo", "EnhancedGenerateAttribute", "AttributeStrategy", "CustomAttributeResult", "NewAttributeValueIDManager", "NewAttributeImportanceCalculator", "NewAttributeImportanceCalculatorWithRules", "RetryableError", "CookieLoadError", "FilteredError", "TaskHandledError", "NewRetryableError", "NewNonRetryableError", "NewFilteredError", "NewTaskHandledError", "AsTaskHandledError", "IsFilteredError", "IsTaskHandledError", "IsRetryableError", "IsAuthenticationExpiredError", "NewCookieLoadError", "IsCookieLoadError", "SheinProductResponse", "SkcInfo", "ParseTime", "SKUCreationParams", "SKCCreationParams", "SKUBuildRequest", "SensitiveWordsFilter", "NewSensitiveWordsFilterWithDefaults", "NewSensitiveWordsFilter", "initHardcodedWords", "FormatTaskReasonMessage", "FormatTaskStageMessage"},
		"internal/shein/category": {"AICategorySelectorHandler", "NewAICategorySelectorHandler", "CategoryAPI", "NewCategoryAPI", "GetCategoryTreeHandler", "NewGetCategoryTreeHandler"},
		"internal/shein/content":  {"SensitiveWordsFilter", "SensitiveWordsMode", "ModeBlock", "ModeClean", "ModeWarn", "SensitiveWordsProcessor", "NewSensitiveWordsProcessor", "NewSensitiveWordsBlockHandler", "NewSensitiveWordsCleanHandler", "NewSensitiveWordsWarnHandler"},
		"internal/shein/aicache":  {"TypeAttribute", "TypeSaleAttr", "TypeSKCTranslate", "TypeAttrValueFallback"},
	} {
		if relative == "internal/shein/category" || relative == "internal/shein/aicache" {
			if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
				t.Errorf("retired SHEIN implementation root remains: %s (%v)", relative, err)
			}
			continue
		}

		paths, err := filepath.Glob(filepath.Join("..", filepath.FromSlash(relative), "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		if relative == "internal/shein" {
			if len(paths) != 0 {
				t.Errorf("retired root SHEIN Go package must be absent: %v", paths)
			}
		} else if len(paths) == 0 {
			t.Fatalf("retained mixed package is absent: %s", relative)
		}
		for _, path := range paths {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, declaration := range file.Decls {
				var names []*ast.Ident
				switch declaration := declaration.(type) {
				case *ast.FuncDecl:
					if declaration.Recv == nil || (relative == "internal/shein/category" && declaration.Name.Name == "GetCategoryIDListWithTree") {
						names = append(names, declaration.Name)
					}
				case *ast.GenDecl:
					for _, spec := range declaration.Specs {
						switch spec := spec.(type) {
						case *ast.TypeSpec:
							names = append(names, spec.Name)
						case *ast.ValueSpec:
							names = append(names, spec.Names...)
						}
					}
				}
				for _, name := range names {
					for _, symbol := range retired {
						if name.Name == symbol {
							t.Errorf("%s recreates retired SHEIN declaration %s", path, symbol)
						}
					}
					if relative == "internal/shein/category" && name.Name == "GetCategoryIDListWithTree" {
						t.Errorf("%s retains old category Task method", path)
					}
				}
			}
		}
	}
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "internal", "shein", "content", "cleanup_context.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		typ, ok := node.(*ast.TypeSpec)
		if !ok || typ.Name.Name != "CleanupContext" {
			return true
		}
		shape, ok := typ.Type.(*ast.StructType)
		if !ok {
			t.Fatal("CleanupContext must be a narrow value struct")
		}
		expected := map[string]bool{"Context": true, "AuthorizedBrand": true, "SourceBrand": true, "ProductData": true}
		for _, field := range shape.Fields.List {
			if len(field.Names) != 1 || !expected[field.Names[0].Name] {
				t.Errorf("CleanupContext must not embed or add old Task state: %v", field.Names)
				continue
			}
			delete(expected, field.Names[0].Name)
		}
		if len(expected) != 0 {
			t.Errorf("missing cleanup inputs: %v", expected)
		}
		return false
	})
}

func TestNumericContentConfigurationClosureIsRetired(t *testing.T) {
	// Full decoded-import protection lives in TestListingKitTaskBackendSourcesAreRetired.
	if _, err := os.Lstat(filepath.Join("..", "internal", "shein", "generationtopics")); !os.IsNotExist(err) {
		t.Errorf("retired SHEIN generationtopics root must be absent: %v", err)
	}
	for _, relative := range []string{
		"internal/listingadmin/sensitive_word.go",
		"internal/listingadmin/sensitive_word_repository.go",
		"internal/listingadmin/sensitive_word_model.go",
		"internal/listingadmin/sensitive_word_query.go",
		"internal/listingadmin/generation_topic_policy.go",
		"internal/listingadmin/generation_topic_policy_repository.go",
		"internal/listingadmin/generation_topic_override.go",
		"internal/listingadmin/generation_topic_override_repository.go",
		"internal/publishing/shein/generation_topic_runtime.go",
		"internal/shein/submitprep/sensitive_words.go",
		"internal/listingadmin/sensitive_word_model_test.go",
		"internal/listingadmin/sensitive_word_query_test.go",
		"internal/listingadmin/generation_topic_policy_test.go",
		"internal/listingadmin/generation_topic_policy_query_test.go",
		"internal/listingadmin/owner_scoped_repository_test.go",
		"internal/publishing/shein/generation_topic_runtime_test.go",
		"internal/publishing/shein/generation_topic_policy_integration_test.go",
		"internal/shein/submitprep/generation_topic_overlay_test.go",
	} {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired numeric content source remains: %s (%v)", relative, err)
		}
	}
	if paths, err := filepath.Glob(filepath.Join("..", "internal", "shein", "submitprep", "*.go")); err != nil || len(paths) != 0 {
		t.Errorf("retired submitprep package remains: %v (%v)", paths, err)
	}
	retired := strings.Fields(`SensitiveWord SensitiveWordQuery SensitiveWordPage SensitiveWordPageReqDTO SensitiveWordRespDTO SensitiveWordRepository GormSensitiveWordRepository listingSensitiveWord NewGormSensitiveWordRepository AutoMigrateSensitiveWords
		GenerationTopicPolicy GenerationTopicPolicyQuery GenerationTopicPolicyPage GenerationTopicPolicyRepository GormGenerationTopicPolicyRepository listingGenerationTopicPolicy NewGormGenerationTopicPolicyRepository AutoMigrateGenerationTopicPolicies
		GenerationTopicOverride GenerationTopicOverrideQuery GenerationTopicOverridePage GenerationTopicOverrideRepository GormGenerationTopicOverrideRepository listingGenerationTopicOverride NewGormGenerationTopicOverrideRepository AutoMigrateGenerationTopicOverrides
		requireOwnerUserID ErrOwnerUserIDRequired takeOwnedTenantRow updateOwnedTenantRow postgresColumnDefinition postgresColumnTypeMigration sensitiveWordLegacyColumnMigrations ensurePostgresColumnTypeMigrations lookupPostgresColumnDefinition postgresColumnDefinitionNeedsTypeMigration normalizePostgresColumnTypeName normalizeExpectedPostgresType ensureUniqueIndex
		SetSensitiveWordRepository SetGenerationTopicPolicyRepository SetGenerationTopicOverrideRepository tenantGenerationTopicPolicyPromptBlock NewSensitiveWordServiceForContext`)
	for _, relative := range []string{"internal/listingadmin", "internal/publishing/shein"} {
		paths, err := filepath.Glob(filepath.Join("..", filepath.FromSlash(relative), "*.go"))
		if err != nil || len(paths) == 0 {
			t.Fatalf("retained package is absent: %s (%v)", relative, err)
		}
		for _, path := range paths {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, declaration := range file.Decls {
				var names []*ast.Ident
				switch declaration := declaration.(type) {
				case *ast.FuncDecl:
					if declaration.Recv == nil {
						names = append(names, declaration.Name)
					}
				case *ast.GenDecl:
					for _, spec := range declaration.Specs {
						switch spec := spec.(type) {
						case *ast.TypeSpec:
							names = append(names, spec.Name)
						case *ast.ValueSpec:
							names = append(names, spec.Names...)
						}
					}
				}
				for _, name := range names {
					for _, symbol := range retired {
						if name.Name == symbol {
							t.Errorf("%s recreates retired numeric content declaration %s", path, symbol)
						}
					}
				}
			}
		}
	}
	for relative, required := range map[string][]string{
		"internal/publishing/shein/submit_prep_sensitive_adapter.go": {"content.NewSensitiveWordServiceInMemory(logs)"},
		"internal/shein/content/cleanup_context.go":                  {"type CleanupContext struct"},
	} {
		content, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		for _, symbol := range required {
			if !strings.Contains(string(content), symbol) {
				t.Errorf("%s is missing retained content/Store rule %s", relative, symbol)
			}
		}
	}
}

func TestSheinCanonicalDraftContractIsRetained(t *testing.T) {
	required := map[string]bool{}
	for _, name := range strings.Fields("Package Package.DraftPayload Package.PreviewPayload Package.FinalSubmissionDraft RequestDraft FinalDraft DraftPayloadOf SetDraftPayload SetPreviewPayload DecodePersistedPackageStrict ClonePackageForPersistence") {
		required[name] = false
	}
	for _, source := range trackedProductionTextSources(t, []string{"internal/publishing/shein"}, func(path string) bool { return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") }) {
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", filepath.FromSlash(source.path)), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.FuncDecl:
				if n.Recv == nil {
					if _, ok := required[n.Name.Name]; ok {
						required[n.Name.Name] = true
					}
				}
			case *ast.TypeSpec:
				if _, ok := required[n.Name.Name]; ok {
					required[n.Name.Name] = true
				}
				if n.Name.Name == "Package" {
					shape, ok := n.Type.(*ast.StructType)
					if !ok {
						t.Fatal("Package must remain a struct")
					}
					for _, f := range shape.Fields.List {
						for _, name := range f.Names {
							key := "Package." + name.Name
							if _, ok := required[key]; ok {
								required[key] = true
							}
						}
					}
				}
			}
			return true
		})
	}
	for name, found := range required {
		if !found {
			t.Errorf("current SHEIN contract missing: %s", name)
		}
	}
}

func assertLegacyPlatformAPICallClosureRetired(t *testing.T) {
	t.Helper()
	retiredRoots := []string{"internal/amazon/api", "internal/temu/api", "internal/shein/api/category", "internal/shein/api/image", "internal/shein/api/marketing", "internal/shein/api/pricing", "internal/shein/api/translate", "internal/shein/pricing", "internal/shein/sherr"}
	for _, relative := range append(append([]string{}, retiredRoots...), []string{"internal/shein/authorizedbrand/resolver.go", "internal/shein/authorizedbrand/resolver_test.go", "internal/shein/api/product/response.go", "internal/core/config/amazon_marketplace.go", "internal/core/config/amazon_marketplace_test.go", "internal/core/config/loader_platform.go", "internal/shein/api/attribute/client.go", "internal/shein/api/other/client.go", "internal/shein/api/product/client.go", "internal/shein/api/product/client_test.go", "internal/shein/api/product/cost_price.go", "internal/shein/api/product/cost_price_test.go", "internal/shein/api/product/image_helpers.go", "internal/shein/api/product/image_helpers_test.go", "internal/shein/api/product/interface.go", "internal/shein/api/product/inventory.go", "internal/shein/api/product/inventory_manager.go", "internal/shein/api/product/language_list.go", "internal/shein/api/product/language_list_test.go", "internal/shein/api/product/name_length_config.go", "internal/shein/api/product/name_length_config_test.go", "internal/shein/api/product/price.go", "internal/shein/api/product/price_manager.go", "internal/shein/api/product/product_manager.go", "internal/shein/api/product/request.go", "internal/shein/api/product/shelf.go", "internal/shein/api/product/site_list.go", "internal/shein/api/product/site_list_test.go", "internal/shein/api/product/stock.go", "internal/shein/api/types.go", "internal/shein/api/warehouse/client.go", "internal/shein/api/warehouse/client_test.go", "internal/shein/client/base_client.go", "internal/shein/client/base_client_test.go", "internal/shein/client/error_handler.go", "internal/shein/client/logger_port.go"}...) {
		if _, err := os.Lstat(filepath.Join("..", filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired platform API artifact %s must be absent: %v", relative, err)
		}
	}
	rootFiles, err := filepath.Glob(filepath.Join("..", "internal", "shein", "api", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rootFiles) != 0 {
		t.Errorf("retired root SHEIN API package remains: %v", rootFiles)
	}
	retiredByOwner := map[string][]string{
		"internal/shein/authorizedbrand": {"Config", "ProductAPI", "Resolver", "NewResolver", "resolveFromItems", "findByCode", "findByName", "newResolved", "trimMatchKey"},
		"internal/core/config":           {"ResolveAmazonMarketplaceID", "ResolveAmazonMarketplaceConfig", "firstEnabledAmazonMarketplaceID", "BuildPlatformConfig", "SPAPIConfig", "MarketplaceConfig", "AmazonConfigPaths", "PlatformConfig", "setTemuDefaults", "setSheinDefaults"},
		"internal/shein/api":             {"APIResponse", "APIError", "AuthenticationExpiredError", "IsAuthenticationExpired"},
		"internal/shein/api/attribute":   {"Client", "NewClient", "AttributeAPI", "AttributeTemplate", "AttributeTemplateInfo", "ValidateAttributeResponse", "AttributeValueNameMulti", "PreAttributeValue", "AddCustomAttributeValueRequest", "AddCustomAttributeValueResponse"},
		"internal/shein/api/category":    {"Client", "NewClient", "CategoryAPI", "CategoryInfo", "CategoryTreeResponse", "CategoryTreeNode", "SuggestCategoryResponse", "SuggestCategoryItem"},
		"internal/shein/api/image":       {"imageDownloadProvider", "Client", "NewClient", "NewClientWithImageDownloader", "plainHTTPImageDownloader", "downloadImageWithPlainHTTP", "ImageAPI"},
		"internal/shein/api/marketing":   {"CreateActivityRequest", "ActivityBaseInfo", "ActivityRule", "CostAndStockInfo", "SkuCostInfo", "CreateActivityResponse", "ActivityCreateInfo", "Client", "NewClient", "MarketingAPI", "AutoPartakeActivityTypeRegular", "AutoPartakeActivityTypeLimited", "DefaultAutoPartakeActivityType", "AutoPartakeConfigStateOpen", "AutoPartakeConfigStateClosed", "GetAvailableSkcListRequest", "GetAvailableSkcListResponse", "AvailableSkcListInfo", "SkcInfo", "SitePriceInfo", "SkuSitePriceInfo", "SkuCostPriceInfo", "SaveConfigRequest", "ActivityConfig", "ActivitySitePriceInfo", "SaveConfigResponse", "GetConfigListRequest", "GetConfigListResponse", "ConfigListInfo", "ActivityConfigInfo", "ActivityConfigDetail", "UpdateConfigStateRequest", "UpdateConfigStateResponse", "CalculateSupplyPriceRequest", "SkcPriceInfo", "SkuPriceInfo", "CalculateSupplyPriceResponse", "SkcCalculationResult", "SkuCalculationInfo", "PriceInfo", "QueryPromotionGoodsRequest", "ActivityBaseInfoRequest", "QueryPromotionGoodsResponse", "PromotionGoodsInfo", "MetaInfo", "PromotionGoodsData", "PromotionSkuInfo", "SkuAttributeInfo", "SupplyPriceInfo", "CheckSupplyPrice", "CheckStock"},
		"internal/shein/api/other":       {"Client", "NewClient", "OtherAPI", "SpuLimitCountResponse", "SpuLimitCountInfoWrapper", "SpuLimitCountInfo", "BatchCheckOnWayResponse", "GetUserResponse", "UserInfo", "ShelfQuotaResponse", "ShelfQuotaInfo"},
		"internal/shein/api/pricing":     {"Client", "NewClient", "PricingAPI", "PageRequest", "BatchHandleCostDiscussRequest", "ConfirmInfo", "CreateCostDiscuss", "SkuCostInfoForReappeal", "BatchHandleCostDiscussResponse", "BargainPageResponse", "BargainPageData", "SkuCostPrice", "CostPriceHistory", "SkuCostPriceHistory", "BargainPageMeta", "BargainPageCustomObj", "BatchReQuoteRequest", "ReQuoteConfirmInfo", "BatchReQuoteResponse"},
		"internal/shein/api/product":     {"Client", "NewClient", "CostPriceQueryRequest", "CostPriceQueryResponse", "CostPriceInfo", "SkcCostData", "CostSaleAttribute", "SkuCostInfo", "SaleAttributeItem", "CostPrice", "ChangeCostReason", "IsCostPriceUnavailable", "CollectSizeMapImageURLs", "ProductAPI", "InventoryQueryRequest", "InventoryQueryResponse", "InventoryInfo", "SkcInventory", "SkuInventory", "WarehouseInventory", "SkuSaleNameInfo", "InventoryUpdateRequest", "SkcInventoryUpdate", "SkuInventoryUpdate", "WarehouseInventoryUpdate", "InventoryUpdateResponse", "inventoryManager", "newInventoryManager", "InventoryManager", "NewInventoryManager", "LanguageListItem", "NameLengthConfigItem", "PriceQueryRequest", "PriceQueryResponse", "SkcPriceData", "SkuPriceInfo", "SaleNameInfo", "SkuPriceDetail", "PreCheckInfo", "priceManager", "newPriceManager", "PriceManager", "NewPriceManager", "productManager", "newProductManager", "ProductRecordRequest", "ProductListRequest", "ShelfOperateRequest", "SkcSiteInfo", "SubSite", "ShelfOperateResponse", "ShelfOperateResult", "SiteListSubSite", "SiteListGroup", "StockQueryRequest", "SpuSkcItem", "SkcNameItem", "StockQueryResponse", "StockData", "ProductListing", "SheinResponse", "ResponseInfo", "ResponseSKC", "ResponseSKU", "RecordResponse", "RecordItem", "PartInfoResponse", "PartInfo", "ProductType", "ProductResponse", "ProductInfoWrapper", "ConfirmPublishResponse", "ConfirmInfo", "ConfirmData", "ProductListResponse", "ProductListItem", "SkcInfoItem", "SkuInfo", "PreValidResult", "SkcErrorMessage", "BrandItem", "BrandListResponse"},
		"internal/shein/api/translate":   {"Request", "Item", "Response", "Info", "TranslationData", "Meta", "Client", "NewClient", "TranslateAPI"},
		"internal/shein/api/warehouse":   {"Client", "NewClient", "WarehouseAPI", "StoreAddressAddRequest", "StoreAddressCheckRequest", "StoreAddressCheckInfo", "CollectionOption", "LatLng", "CollectionJudgeRecord", "ProviderInfo"},
		"internal/shein/client":          {"BaseAPIClient", "NewBaseAPIClient", "isCostPriceEndpoint", "isSheinAuthFailurePayload", "extractResponseCodeMsg", "fieldString", "APIErrorHandler", "NewAPIErrorHandler", "loggerDependency", "componentLogger", "dpasPrefix", "ssoPrefix", "mbrsPrefix", "productRecordEndpoint", "batchCheckOnWayEndpoint", "validateAttributeEndpoint", "addAttributeValueEndpoint", "getCategoryEndpoint", "getCategoryTreeEndpoint", "cvTextSuggestCategoryEndpoint", "getProductEndpoint", "queryBrandListEndpoint", "updateProductEndpoint", "deleteProductEndpoint", "confirmPublishEndpoint", "saveDraftEndpoint", "publishProductEndpoint", "productNameLengthConfigEndpoint", "languageListEndpoint", "siteListEndpoint", "uploadImageEndpoint", "translateTextEndpoint", "storeAddressAddEndpoint", "storeAddressCheckEndpoint", "getAttributeTemplatesEndpoint", "getPartInfoEndpoint", "batchHandleCostDiscussEndpoint", "bargainPageNewEndpoint", "batchReQuoteEndpoint", "getSpuLimitCountEndpoint", "getUserEndpoint", "productListEndpoint", "queryStockEndpoint", "queryPriceEndpoint", "queryCostPriceEndpoint", "queryInventoryEndpoint", "updateInventoryEndpoint", "operateShelfStatusEndpoint", "getAvailableSkcListEndpoint", "saveConfigEndpoint", "getConfigListEndpoint", "updateConfigStateEndpoint", "mrsPrefix", "queryPromotionGoodsEndpoint", "createActivityEndpoint", "capitalPrefix", "calculateSupplyPriceEndpoint", "queryShelfQuotaEndpoint", "GetEndpoint", "GetProductRecordEndpoint", "GetBatchCheckOnWayEndpoint", "GetValidateAttributeEndpoint", "GetAddAttributeValueEndpoint", "GetCategoryEndpoint", "GetCategoryTreeEndpoint", "GetCvTextSuggestCategoryEndpoint", "GetProductEndpoint", "GetQueryBrandListEndpoint", "GetUpdateProductEndpoint", "GetDeleteProductEndpoint", "GetConfirmPublishEndpoint", "GetSaveDraftEndpoint", "GetPublishProductEndpoint", "GetProductNameLengthConfigEndpoint", "GetLanguageListEndpoint", "GetSiteListEndpoint", "GetUploadImageEndpoint", "GetTranslateTextEndpoint", "GetStoreAddressAddEndpoint", "GetStoreAddressCheckEndpoint", "GetAttributeTemplatesEndpoint", "GetPartInfoEndpoint", "GetBatchHandleCostDiscussEndpoint", "GetBargainPageNewEndpoint", "GetBatchReQuoteEndpoint", "GetSpuLimitCountEndpoint", "GetUserEndpoint", "GetProductListEndpoint", "GetQueryStockEndpoint", "GetQueryPriceEndpoint", "GetQueryCostPriceEndpoint", "GetQueryInventoryEndpoint", "GetUpdateInventoryEndpoint", "GetOperateShelfStatusEndpoint", "GetAvailableSkcListEndpoint", "GetSaveConfigEndpoint", "GetConfigListEndpoint", "GetUpdateConfigStateEndpoint", "GetQueryPromotionGoodsEndpoint", "GetCalculateSupplyPriceEndpoint", "GetCreateActivityEndpoint", "GetQueryShelfQuotaEndpoint"},
		"internal/shein/pricing":         {"PricingAPI", "NewPricingAPI"},
	}
	requiredByFile := map[string][]string{
		"internal/shein/authorizedbrand/types.go":            {"Resolved"},
		"internal/shein/authorizedbrand/context.go":          {"WithResolved", "FromContext"},
		"internal/core/config/common_types.go":               {"HTTPClientConfig", "DefaultHTTPClientConfig"},
		"internal/core/config/type_platform.go":              {"PlatformsConfig", "Alibaba1688Config"},
		"internal/core/config/type_rabbitmq.go":              {"RetryConfig", "DefaultRetryConfig"},
		"internal/shein/api/attribute/interface.go":          {"AttributeInfo", "AttributeValue", "AttributeValueGroup", "CustomAttributeRelation"},
		"internal/shein/api/other/interface.go":              {"FlexibleString", "SupplierOperateInfo", "SupplierOperateInfoResponse"},
		"internal/shein/api/product/model.go":                {"Product", "Extra", "LanguageContent", "ProductAttribute", "ProductVideo", "SiteInfo", "SizeAttribute"},
		"internal/shein/api/product/skc_sku.go":              {"SKC", "SkcExtra", "SaleAttribute", "SKU", "SkuExtra", "FieldDisabledInfo", "QuantityInfo", "CostInfo", "PriceInfo", "StockInfo", "ImageInfo", "ImageDetail", "SiteDetailImageInfo", "DetailImage"},
		"internal/shein/api/warehouse/address_conversion.go": {"WarehousesFromStoreAddresses", "collectSaleCountries"},
		"internal/shein/api/warehouse/interface.go":          {"Warehouse", "WarehouseResponse", "StoreAddressListRequest", "StoreAddress", "StoreSiteInfo", "StoreAddressListInfo"},
		"internal/shein/client/cookie_values.go":             {"CookieData", "buildCookiesFromJSON"},
		"internal/shein/client/session_transport.go":         {"SessionTransport", "NewSessionTransport", "CookiesFromBrowserState", "sessionHeaders"},
	}
	for relative, names := range requiredByFile {
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", filepath.FromSlash(relative)), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		found := map[string]bool{}
		for _, declaration := range file.Decls {
			switch d := declaration.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil {
					found[d.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					if v, ok := spec.(*ast.TypeSpec); ok {
						found[v.Name.Name] = true
					}
				}
			}
		}
		for _, name := range names {
			if !found[name] {
				t.Errorf("%s must retain current declaration %s", relative, name)
			}
		}
	}
	for _, relative := range trackedFiles(t, ".") {
		if !strings.HasSuffix(relative, ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", filepath.FromSlash(relative)), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			imported, err := decodeGoImportPath(spec.Path.Value)
			if err != nil {
				t.Fatalf("decode import in %s: %v", relative, err)
			}
			if imported == "task-processor/internal/shein/api" {
				t.Errorf("%s imports retired root SHEIN API", relative)
			}
			for _, root := range retiredRoots {
				if importMatchesPrefix(imported, "task-processor/"+root) {
					t.Errorf("%s imports retired platform API %s", relative, imported)
				}
			}
		}
		owner := filepath.ToSlash(filepath.Dir(relative))
		check := func(name string) {
			if slices.Contains(retiredByOwner[owner], name) {
				t.Errorf("%s recreates retired platform declaration %s", relative, name)
			}
		}
		for _, declaration := range file.Decls {
			switch d := declaration.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil {
					check(d.Name.Name)
				} else {
					receiver := d.Recv.List[0].Type
					if p, ok := receiver.(*ast.StarExpr); ok {
						receiver = p.X
					}
					if id, ok := receiver.(*ast.Ident); ok {
						check(id.Name)
					}
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						check(s.Name.Name)
						if owner == "internal/core/config" {
							if st, ok := s.Type.(*ast.StructType); ok {
								for _, field := range st.Fields.List {
									for _, name := range field.Names {
										if (s.Name.Name == "AmazonConfig" && (name.Name == "SPAPI" || name.Name == "ConfigPaths")) || (s.Name.Name == "PlatformsConfig" && (name.Name == "Temu" || name.Name == "Shein")) {
											t.Errorf("%s retains retired config field %s.%s", relative, s.Name.Name, name.Name)
										}
									}
								}
							}
						}
					case *ast.ValueSpec:
						for _, name := range s.Names {
							check(name.Name)
						}
					}
				}
			}
		}
	}
	endpointSources := map[string][]string{
		"internal/shein/client/endpoint.go":  {`apiPrefix`, `"/spmp-api-prefix/spmp"`, `getSupplierOperateInfoEndpoint`, `"/sso/public/account/supplier/getSupplierOperateInfo"`, `getWarehousesEndpoint`, `"/inventory/query_merchant_warehouse"`, `storeAddressListEndpoint`, `"/gsp/store/address/list"`},
		"internal/shein/client/endpoints.go": {`func GetSupplierOperateInfoEndpoint() string`, `return getSupplierOperateInfoEndpoint`, `func GetWarehousesEndpoint() string`, `return getWarehousesEndpoint`, `func GetStoreAddressListEndpoint() string`, `return storeAddressListEndpoint`},
	}
	for relative, required := range endpointSources {
		content, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range required {
			if !strings.Contains(string(content), value) {
				t.Errorf("%s must retain endpoint contract %s", relative, value)
			}
		}
	}
}
