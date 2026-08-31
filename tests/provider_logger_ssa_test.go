package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

type providerLoggerRule struct {
	PackagePath     string
	ConstructorName string
	AdapterName     string
	LoggerField     string
	ConfigArgument  int
}

type typedNilCallRule struct {
	PackagePath  string
	FunctionName string
	Argument     int
}

func TestProviderLoggerSSAMustBoundRejectsUnsafeControlAndTypeFlows(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		violations int
	}{
		{
			name: "conditional assignment",
			source: `package sample

import openai "example/root/openai"

func build(entry *openai.Entry, enabled bool) {
	config := &openai.ClientConfig{}
	if enabled {
		config.Logger = openai.AdaptLogrus(entry)
	}
	openai.NewClient(config)
}
`,
			violations: 1,
		},
		{
			name: "zero iteration loop",
			source: `package sample

import openai "example/root/openai"

func build(entry *openai.Entry, enabled bool) {
	config := &openai.ClientConfig{}
	for enabled {
		config.Logger = openai.AdaptLogrus(entry)
		enabled = false
	}
	openai.NewClient(config)
}
`,
			violations: 1,
		},
		{
			name: "lexical shadow",
			source: `package sample

import openai "example/root/openai"

func build(entry *openai.Entry) {
	config := &openai.ClientConfig{}
	{
		config := &openai.ClientConfig{}
		config.Logger = openai.AdaptLogrus(entry)
	}
	openai.NewClient(config)
}
`,
			violations: 1,
		},
		{
			name: "same name free function",
			source: `package sample

import openai "example/root/openai"

type localLogger struct{}
func (localLogger) Log() {}
func AdaptLogrus(*openai.Entry) openai.Logger { return localLogger{} }

func build(entry *openai.Entry) {
	config := &openai.ClientConfig{Logger: AdaptLogrus(entry)}
	openai.NewClient(config)
}
`,
			violations: 1,
		},
		{
			name: "same name method",
			source: `package sample

import openai "example/root/openai"

type localLogger struct{}
func (localLogger) Log() {}
type adapterFactory struct{}
func (adapterFactory) AdaptLogrus(*openai.Entry) openai.Logger { return localLogger{} }

func build(entry *openai.Entry) {
	config := &openai.ClientConfig{Logger: (adapterFactory{}).AdaptLogrus(entry)}
	openai.NewClient(config)
}
`,
			violations: 1,
		},
		{
			name: "wrong provider adapter",
			source: `package sample

import (
	grsai "example/root/grsai"
	openai "example/root/openai"
)

func build(entry *grsai.Entry) {
	config := &openai.ClientConfig{Logger: grsai.AdaptLogrus(entry)}
	openai.NewClient(config)
}
`,
			violations: 1,
		},
		{
			name: "discarded adapter cannot satisfy another call",
			source: `package sample

import openai "example/root/openai"

func build(entry *openai.Entry) {
	configured := &openai.ClientConfig{Logger: openai.AdaptLogrus(entry)}
	openai.NewClient(configured)
	missing := &openai.ClientConfig{}
	_ = openai.AdaptLogrus(entry)
	openai.NewClient(missing)
}
`,
			violations: 1,
		},
		{
			name: "typed nil",
			source: `package sample

import openai "example/root/openai"

type localLogger struct{}
func (*localLogger) Log() {}

func build() {
	var logger openai.Logger = (*localLogger)(nil)
	config := &openai.ClientConfig{Logger: logger}
	openai.NewClient(config)
}
`,
			violations: 1,
		},
		{
			name: "typed nil adapter argument",
			source: `package sample

import openai "example/root/openai"

func build() {
	config := &openai.ClientConfig{Logger: openai.AdaptLogrus((*openai.Entry)(nil))}
	openai.NewClient(config)
}
`,
			violations: 1,
		},
		{
			name: "constructor call inside method body",
			source: `package sample

import openai "example/root/openai"

type service struct{}

func (service) build() {
	config := &openai.ClientConfig{}
	openai.NewClient(config)
}
`,
			violations: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			loaded := loadProviderLoggerSSAFixture(t, test.source)
			violations := loggerContractSSAViolations(loaded, fixtureProviderLoggerRules(), nil)
			if len(violations) != test.violations {
				t.Fatalf("violations = %v, want %d", violations, test.violations)
			}
		})
	}
}

func TestProviderLoggerSSAMustBoundAcceptsAllPathBindingsAndStaticHelpers(t *testing.T) {
	loaded := loadProviderLoggerSSAFixture(t, `package sample

import openai "example/root/openai"

func configured(entry *openai.Entry) *openai.ClientConfig {
	config := &openai.ClientConfig{}
	config.Logger = openai.AdaptLogrus(entry)
	return config
}

type factory struct{}

func (factory) configured(entry *openai.Entry) *openai.ClientConfig {
	base := &openai.ClientConfig{}
	cloned := *base
	cloned.Logger = openai.AdaptLogrus(entry)
	return &cloned
}

func build(entry *openai.Entry, first bool) {
	config := &openai.ClientConfig{}
	if first {
		config.Logger = openai.AdaptLogrus(entry)
	} else {
		config.Logger = openai.AdaptLogrus(entry)
	}
	openai.NewClient(config)
	openai.NewClient(configured(entry))
	openai.NewClient((factory{}).configured(entry))
}
`)
	if violations := loggerContractSSAViolations(loaded, fixtureProviderLoggerRules(), nil); len(violations) != 0 {
		t.Fatalf("valid must-bound flows reported violations: %v", violations)
	}
}

func TestProviderLoggerSSAMustBoundUsesSecondS3OptionsArgument(t *testing.T) {
	t.Run("rejects unbound uploader options", func(t *testing.T) {
		loaded := loadProviderLoggerSSAFixture(t, `package sample

import s3 "example/root/s3"

func build() { s3.NewUploaderWithOptions(nil, s3.UploaderOptions{}) }
`)
		violations := loggerContractSSAViolations(loaded, fixtureProviderLoggerRules(), nil)
		if len(violations) != 1 {
			t.Fatalf("violations = %v, want exactly one unbound S3 options violation", violations)
		}
	})

	t.Run("accepts bound uploader options", func(t *testing.T) {
		loaded := loadProviderLoggerSSAFixture(t, `package sample

import s3 "example/root/s3"

func build(entry *s3.Entry) {
		opts := s3.UploaderOptions{Logger: s3.AdaptLogrus(entry)}
		s3.NewUploaderWithOptions(nil, opts)
}
`)
		if violations := loggerContractSSAViolations(loaded, fixtureProviderLoggerRules(), nil); len(violations) != 0 {
			t.Fatalf("valid S3 logger binding reported violations: %v", violations)
		}
	})
}

func TestTypedNilLoggerCallAnalysisRejectsExplicitAndAliasedNil(t *testing.T) {
	loaded := loadProviderLoggerSSAFixture(t, `package sample

import (
	openai "example/root/openai"
	productenrich "example/root/productenrich"
)

func explicitNil() {
	productenrich.NewLLMManagerAdapter("config", (*openai.Entry)(nil))
}

func aliasedNil() {
	var logger *openai.Entry
	productenrich.NewLLMManagerAdapter("config", logger)
}
`)
	violations := loggerContractSSAViolations(loaded, nil, []typedNilCallRule{{
		PackagePath:  "example/root/productenrich",
		FunctionName: "NewLLMManagerAdapter",
		Argument:     1,
	}})
	if len(violations) != 2 {
		t.Fatalf("typed nil logger violations = %v, want explicit and aliased nil", violations)
	}
}

func TestProductionProviderAndLLMLoggerContractsInSSA(t *testing.T) {
	repositoryRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	modules, err := discoverRepositoryGoModules(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	rules := productionProviderLoggerRules()
	nilRules := []typedNilCallRule{{
		PackagePath:  "task-processor/internal/productenrich",
		FunctionName: "NewLLMManagerAdapter",
		Argument:     1,
	}}
	var violations []string
	for _, goModPath := range modules {
		loaded := loadRepositoryModulePackages(t, repositoryRoot, goModPath)
		if len(loaded) == 0 {
			t.Logf("provider SSA module %s: zero packages (N/A)", relativeModulePath(repositoryRoot, filepath.Dir(goModPath)))
			continue
		}
		violations = append(violations, loggerContractSSAViolations(loaded, rules, nilRules)...)
	}
	if len(violations) != 0 {
		t.Fatalf("provider constructors without must-bound matching logger:\n%s", strings.Join(violations, "\n"))
	}
}

func productionProviderLoggerRules() []providerLoggerRule {
	const openAIPath = "task-processor/internal/integration/openai"
	return []providerLoggerRule{
		{PackagePath: openAIPath, ConstructorName: "NewClient", AdapterName: "AdaptLogrus", LoggerField: "Logger", ConfigArgument: 0},
		{PackagePath: openAIPath, ConstructorName: "NewManager", AdapterName: "AdaptLogrus", LoggerField: "Logger", ConfigArgument: 0},
		{PackagePath: openAIPath, ConstructorName: "NewRequestPool", AdapterName: "AdaptLogrus", LoggerField: "Logger", ConfigArgument: 0},
		{PackagePath: openAIPath, ConstructorName: "NewCachedClient", AdapterName: "AdaptLogrus", LoggerField: "Logger", ConfigArgument: 0},
		{PackagePath: openAIPath, ConstructorName: "NewResilientClient", AdapterName: "AdaptLogrus", LoggerField: "Logger", ConfigArgument: 0},
		{PackagePath: "task-processor/internal/integration/grsai", ConstructorName: "NewClient", AdapterName: "AdaptLogrus", LoggerField: "Logger", ConfigArgument: 0},
		{PackagePath: "task-processor/internal/integration/s3", ConstructorName: "NewUploaderWithOptions", AdapterName: "AdaptLogrus", LoggerField: "Logger", ConfigArgument: 1},
	}
}

func loadRepositoryModulePackages(t *testing.T, repositoryRoot, goModPath string) []*packages.Package {
	t.Helper()
	moduleDir := filepath.Dir(goModPath)
	configuration := &packages.Config{
		Mode: packages.LoadAllSyntax,
		Dir:  moduleDir,
		Env:  append(os.Environ(), "GOWORK=off"),
	}
	if filepath.Clean(moduleDir) != filepath.Clean(repositoryRoot) {
		temporaryDir := t.TempDir()
		temporaryModPath := filepath.Join(temporaryDir, "module.mod")
		if err := writeIsolatedModfile(goModPath, temporaryModPath); err != nil {
			t.Fatal(err)
		}
		if err := copyFileIfPresent(filepath.Join(moduleDir, "go.sum"), filepath.Join(temporaryDir, "module.sum")); err != nil {
			t.Fatal(err)
		}
		configuration.BuildFlags = []string{"-mod=mod", "-modfile=" + temporaryModPath}
	}
	loaded, err := packages.Load(configuration, "./...")
	if err != nil {
		t.Fatal(err)
	}
	for _, loadedPackage := range loaded {
		if len(loadedPackage.Errors) > 0 {
			t.Fatalf("module %s package %s failed to load: %s", relativeModulePath(repositoryRoot, moduleDir), loadedPackage.PkgPath, packageErrors(loadedPackage.Errors))
		}
	}
	return loaded
}

func fixtureProviderLoggerRules() []providerLoggerRule {
	return []providerLoggerRule{
		{PackagePath: "example/root/openai", ConstructorName: "NewClient", AdapterName: "AdaptLogrus", LoggerField: "Logger", ConfigArgument: 0},
		{PackagePath: "example/root/grsai", ConstructorName: "NewClient", AdapterName: "AdaptLogrus", LoggerField: "Logger", ConfigArgument: 0},
		{PackagePath: "example/root/s3", ConstructorName: "NewUploaderWithOptions", AdapterName: "AdaptLogrus", LoggerField: "Logger", ConfigArgument: 1},
	}
}

func loadProviderLoggerSSAFixture(t *testing.T, source string) []*packages.Package {
	t.Helper()
	root := t.TempDir()
	writeModuleFixtureFile(t, root, "go.mod", "module example/root\n\ngo 1.26.0\n")
	writeModuleFixtureFile(t, root, "openai/provider.go", providerLoggerFixturePackage("openai"))
	writeModuleFixtureFile(t, root, "grsai/provider.go", providerLoggerFixturePackage("grsai"))
	writeModuleFixtureFile(t, root, "s3/provider.go", `package s3

type Logger interface { Log() }
type UploaderOptions struct { Logger Logger }
type Entry struct{}
type adaptedLogger struct{}
func (adaptedLogger) Log() {}
func AdaptLogrus(*Entry) Logger { return adaptedLogger{} }
func NewUploaderWithOptions(any, UploaderOptions) {}
`)
	writeModuleFixtureFile(t, root, "productenrich/manager.go", `package productenrich

import openai "example/root/openai"

func NewLLMManagerAdapter(any, *openai.Entry) {}
`)
	writeModuleFixtureFile(t, root, "sample/sample.go", source)
	configuration := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedDeps | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
		Dir:  root,
		Env:  append(os.Environ(), "GOWORK=off"),
	}
	loaded, err := packages.Load(configuration, "./...")
	if err != nil {
		t.Fatal(err)
	}
	for _, loadedPackage := range loaded {
		if len(loadedPackage.Errors) > 0 {
			t.Fatalf("fixture package %s failed to load: %s", loadedPackage.PkgPath, packageErrors(loadedPackage.Errors))
		}
	}
	return loaded
}

func providerLoggerFixturePackage(name string) string {
	return `package ` + name + `

type Logger interface { Log() }
type ClientConfig struct { Logger Logger }
type Entry struct{}
type adaptedLogger struct{}
func (adaptedLogger) Log() {}
func AdaptLogrus(*Entry) Logger { return adaptedLogger{} }
func NewClient(*ClientConfig) {}
`
}

func packageErrors(errors []packages.Error) string {
	messages := make([]string, 0, len(errors))
	for _, packageError := range errors {
		messages = append(messages, packageError.Error())
	}
	return strings.Join(messages, "; ")
}
