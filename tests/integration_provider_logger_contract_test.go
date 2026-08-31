package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	geminiimage "task-processor/internal/integration/geminiimage"
	grsai "task-processor/internal/integration/grsai"
	openaiclient "task-processor/internal/integration/openai"
)

func TestIntegrationProviderLoggerMethodSetsMatch(t *testing.T) {
	interfaces := []struct {
		name   string
		typeOf reflect.Type
	}{
		{"openai", reflect.TypeOf((*openaiclient.Logger)(nil)).Elem()},
		{"geminiimage", reflect.TypeOf((*geminiimage.Logger)(nil)).Elem()},
		{"grsai", reflect.TypeOf((*grsai.Logger)(nil)).Elem()},
	}
	want := []string{"Debug", "Error", "Info", "Warn"}
	for _, provider := range interfaces {
		if provider.typeOf.NumMethod() != len(want) {
			t.Fatalf("%s Logger methods = %d, want %d", provider.name, provider.typeOf.NumMethod(), len(want))
		}
		for index, methodName := range want {
			method := provider.typeOf.Method(index)
			if method.Name != methodName || method.Type.NumIn() != 2 || method.Type.In(0).Kind() != reflect.String || method.Type.In(1) != reflect.TypeOf(map[string]any{}) {
				t.Errorf("%s Logger method %d = %s %s", provider.name, index, method.Name, method.Type)
			}
		}
	}
}

func TestProviderConstructorLoggerAnalysisRejectsCrossCallAdapterAccounting(t *testing.T) {
	source := `package sample

import (
	"github.com/sirupsen/logrus"
	openai "task-processor/internal/integration/openai"
)

func build(entry *logrus.Entry) {
	configured := &openai.ClientConfig{Logger: openai.AdaptLogrus(entry)}
	openai.NewClient(configured)
	missing := &openai.ClientConfig{}
	_ = openai.AdaptLogrus(entry)
	openai.NewClient(missing)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "cross_call.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	violations := providerConstructorLoggerViolations(fset, file)
	if len(violations) != 1 || !strings.Contains(violations[0], "cross_call.go:13") {
		t.Fatalf("logger binding violations = %v, want only the unbound second constructor", violations)
	}
}

func TestProviderConstructorLoggerAnalysisRejectsPartiallyBoundHelperReturn(t *testing.T) {
	source := `package sample

import (
	"github.com/sirupsen/logrus"
	openai "task-processor/internal/integration/openai"
)

func maybeConfigured(entry *logrus.Entry, skip bool) *openai.ClientConfig {
	config := &openai.ClientConfig{}
	if skip {
		return config
	}
	config.Logger = openai.AdaptLogrus(entry)
	return config
}

func build(entry *logrus.Entry) {
	openai.NewClient(maybeConfigured(entry, true))
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "partial_helper.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	violations := providerConstructorLoggerViolations(fset, file)
	if len(violations) != 1 || !strings.Contains(violations[0], "partial_helper.go:18") {
		t.Fatalf("logger binding violations = %v, want partially bound helper constructor", violations)
	}
}

type providerLoggerBinding struct {
	position token.Pos
	provider string
}

func providerConstructorLoggerViolations(fset *token.FileSet, file *ast.File) []string {
	providers := providerImportAliases(file)
	helpers := providerLoggerHelperSummaries(file, providers)
	var violations []string

	var analyzeBody func(*ast.BlockStmt)
	analyzeBody = func(body *ast.BlockStmt) {
		bindings := make(map[string][]providerLoggerBinding)
		ast.Inspect(body, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.FuncLit:
				analyzeBody(typed.Body)
				return false
			case *ast.AssignStmt:
				recordProviderLoggerAssignments(bindings, typed.Lhs, typed.Rhs, typed.End(), providers, helpers)
			case *ast.DeclStmt:
				declaration, ok := typed.Decl.(*ast.GenDecl)
				if !ok || declaration.Tok != token.VAR {
					return true
				}
				for _, rawSpec := range declaration.Specs {
					spec := rawSpec.(*ast.ValueSpec)
					lhs := make([]ast.Expr, 0, len(spec.Names))
					for _, name := range spec.Names {
						lhs = append(lhs, name)
					}
					recordProviderLoggerAssignments(bindings, lhs, spec.Values, spec.End(), providers, helpers)
				}
			case *ast.CallExpr:
				provider, constructor, ok := providerConstructor(typed, providers)
				if !ok || provider == "gemini" || len(typed.Args) == 0 {
					return true
				}
				if got := providerForLoggerConfigExpression(typed.Args[0], typed.Pos(), bindings, providers, helpers); got != provider {
					position := fset.Position(typed.Pos())
					violations = append(violations, position.String()+": "+provider+"."+constructor+" config is not bound to "+provider+".AdaptLogrus")
				}
			}
			return true
		})
	}

	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Body != nil {
			analyzeBody(function.Body)
		}
	}
	return violations
}

func providerImportAliases(file *ast.File) map[string]string {
	providers := make(map[string]string)
	for _, spec := range file.Imports {
		importPath, err := decodeGoImportPath(spec.Path.Value)
		if err != nil {
			continue
		}
		provider := ""
		switch importPath {
		case "task-processor/internal/integration/openai":
			provider = "openai"
		case "task-processor/internal/integration/grsai":
			provider = "grsai"
		case "task-processor/internal/integration/geminiimage":
			provider = "gemini"
		}
		if provider == "" {
			continue
		}
		alias := filepath.Base(importPath)
		if spec.Name != nil {
			alias = spec.Name.Name
		}
		providers[alias] = provider
	}
	return providers
}

func providerConstructor(call *ast.CallExpr, providers map[string]string) (string, string, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", "", false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return "", "", false
	}
	provider := providers[qualifier.Name]
	if provider == "" {
		return "", "", false
	}
	switch selector.Sel.Name {
	case "NewClient", "NewManager", "NewRequestPool", "NewCachedClient", "NewResilientClient":
		return provider, selector.Sel.Name, true
	default:
		return "", "", false
	}
}

func providerLoggerHelperSummaries(file *ast.File, providers map[string]string) map[string]string {
	summaries := make(map[string]string)
	for iteration := 0; iteration <= len(file.Decls); iteration++ {
		changed := false
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			provider := providerLoggerReturnSummary(function.Body, providers, summaries)
			if provider != "" && summaries[function.Name.Name] != provider {
				summaries[function.Name.Name] = provider
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return summaries
}

func providerLoggerReturnSummary(body *ast.BlockStmt, providers, helpers map[string]string) string {
	bindings := make(map[string][]providerLoggerBinding)
	provider := ""
	valid := true
	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			recordProviderLoggerAssignments(bindings, typed.Lhs, typed.Rhs, typed.End(), providers, helpers)
		case *ast.DeclStmt:
			declaration, ok := typed.Decl.(*ast.GenDecl)
			if !ok || declaration.Tok != token.VAR {
				return true
			}
			for _, rawSpec := range declaration.Specs {
				spec := rawSpec.(*ast.ValueSpec)
				lhs := make([]ast.Expr, 0, len(spec.Names))
				for _, name := range spec.Names {
					lhs = append(lhs, name)
				}
				recordProviderLoggerAssignments(bindings, lhs, spec.Values, spec.End(), providers, helpers)
			}
		case *ast.ReturnStmt:
			for _, expression := range typed.Results {
				if identifier, ok := expression.(*ast.Ident); ok && identifier.Name == "nil" {
					continue
				}
				got := providerForLoggerConfigExpression(expression, typed.Pos(), bindings, providers, helpers)
				if got == "" {
					valid = false
					continue
				}
				if provider != "" && provider != got {
					valid = false
				}
				provider = got
			}
		}
		return true
	})
	if !valid {
		return ""
	}
	return provider
}

func recordProviderLoggerAssignments(bindings map[string][]providerLoggerBinding, lhs, rhs []ast.Expr, position token.Pos, providers, helpers map[string]string) {
	for index, left := range lhs {
		if index >= len(rhs) {
			continue
		}
		right := rhs[index]
		switch target := left.(type) {
		case *ast.Ident:
			provider := providerForLoggerConfigExpression(right, position, bindings, providers, helpers)
			bindings[target.Name] = append(bindings[target.Name], providerLoggerBinding{position: position, provider: provider})
		case *ast.SelectorExpr:
			if target.Sel.Name != "Logger" {
				continue
			}
			identifier, ok := target.X.(*ast.Ident)
			if !ok {
				continue
			}
			provider := providerAdaptLogrusCall(right, providers)
			bindings[identifier.Name] = append(bindings[identifier.Name], providerLoggerBinding{position: position, provider: provider})
		}
	}
}

func providerForLoggerConfigExpression(expression ast.Expr, at token.Pos, bindings map[string][]providerLoggerBinding, providers, helpers map[string]string) string {
	switch typed := expression.(type) {
	case *ast.ParenExpr:
		return providerForLoggerConfigExpression(typed.X, at, bindings, providers, helpers)
	case *ast.UnaryExpr:
		return providerForLoggerConfigExpression(typed.X, at, bindings, providers, helpers)
	case *ast.Ident:
		entries := bindings[typed.Name]
		for index := len(entries) - 1; index >= 0; index-- {
			if entries[index].position < at {
				return entries[index].provider
			}
		}
	case *ast.CompositeLit:
		for _, element := range typed.Elts {
			field, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			name, ok := field.Key.(*ast.Ident)
			if ok && name.Name == "Logger" {
				return providerAdaptLogrusCall(field.Value, providers)
			}
		}
	case *ast.CallExpr:
		switch function := typed.Fun.(type) {
		case *ast.Ident:
			return helpers[function.Name]
		case *ast.SelectorExpr:
			return helpers[function.Sel.Name]
		}
	}
	return ""
}

func providerAdaptLogrusCall(expression ast.Expr, providers map[string]string) string {
	call, ok := expression.(*ast.CallExpr)
	if !ok {
		return ""
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "AdaptLogrus" {
		return ""
	}
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return ""
	}
	return providers[qualifier.Name]
}

type providerConstructorCounts struct {
	openAI int
	grsai  int
	gemini int
}

func scanProviderConstructors(t *testing.T, roots ...string) map[string]providerConstructorCounts {
	t.Helper()
	result := make(map[string]providerConstructorCounts)
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || pathIsWithin(path, filepath.Join("..", "internal", "integration")) {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			providers := make(map[string]string)
			for _, spec := range file.Imports {
				importPath, err := decodeGoImportPath(spec.Path.Value)
				if err != nil {
					return err
				}
				provider := ""
				switch importPath {
				case "task-processor/internal/integration/openai":
					provider = "openai"
				case "task-processor/internal/integration/grsai":
					provider = "grsai"
				case "task-processor/internal/integration/geminiimage":
					provider = "gemini"
				}
				if provider == "" {
					continue
				}
				alias := filepath.Base(importPath)
				if spec.Name != nil {
					alias = spec.Name.Name
				}
				providers[alias] = provider
			}
			counts := providerConstructorCounts{}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				qualifier, ok := selector.X.(*ast.Ident)
				if !ok {
					return true
				}
				provider := providers[qualifier.Name]
				switch selector.Sel.Name {
				case "NewClient", "NewManager", "NewRequestPool", "NewCachedClient", "NewResilientClient":
					switch provider {
					case "openai":
						counts.openAI++
					case "grsai":
						counts.grsai++
					case "gemini":
						counts.gemini++
					}
				}
				return true
			})
			if counts.openAI+counts.grsai+counts.gemini > 0 {
				rel, err := filepath.Rel("..", path)
				if err != nil {
					return err
				}
				result[filepath.ToSlash(rel)] = counts
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func TestProductionProviderConstructorInventoryIsComplete(t *testing.T) {
	want := map[string]providerConstructorCounts{
		"hack/debug/replay-sale-attribute/main.go":                {openAI: 1},
		"internal/amazon/processor.go":                            {openAI: 1},
		"internal/app/httpapi/adapters_openai.go":                 {openAI: 1},
		"internal/app/worker/imageagent/dependencies.go":          {openAI: 1},
		"internal/listingkit/httpapi/ai_client_builders.go":       {openAI: 1, grsai: 2, gemini: 2},
		"internal/listingkit/httpapi/ai_client_strict_chat.go":    {openAI: 1},
		"internal/productenrich/llm_adapter.go":                   {openAI: 1},
		"internal/productimage/httpapi/model_provider_builder.go": {grsai: 1},
		"internal/shein/pipeline/pipeline.go":                     {openAI: 1},
		"internal/temu/pipeline_registry.go":                      {openAI: 1},
		"internal/temu/sku/ai_mapping_handler.go":                 {openAI: 1},
	}
	got := scanProviderConstructors(t,
		filepath.Join("..", "internal"),
		filepath.Join("..", "cmd"),
		filepath.Join("..", "hack"),
		filepath.Join("..", "tools"),
		filepath.Join("..", "tmp"),
	)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("production provider constructor inventory = %#v, want %#v", got, want)
	}
}

func TestProductionProviderConstructorsReceiveMatchingLoggerAdapters(t *testing.T) {
	var violations []string
	for _, root := range []string{
		filepath.Join("..", "internal"),
		filepath.Join("..", "cmd"),
		filepath.Join("..", "hack"),
		filepath.Join("..", "tools"),
		filepath.Join("..", "tmp"),
	} {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || pathIsWithin(path, filepath.Join("..", "internal", "integration")) {
				return nil
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			violations = append(violations, providerConstructorLoggerViolations(fset, file)...)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(violations) > 0 {
		t.Fatalf("provider constructors without matching logger data flow:\n%s", strings.Join(violations, "\n"))
	}
}

func TestProductionLLMManagerAdapterCallersPassLogger(t *testing.T) {
	callers := make(map[string]int)
	for _, root := range []string{
		filepath.Join("..", "internal"),
		filepath.Join("..", "cmd"),
		filepath.Join("..", "hack"),
		filepath.Join("..", "tools"),
		filepath.Join("..", "tmp"),
	} {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			aliases := make(map[string]struct{})
			for _, spec := range file.Imports {
				importPath, err := decodeGoImportPath(spec.Path.Value)
				if err != nil {
					return err
				}
				if importPath != "task-processor/internal/productenrich" {
					continue
				}
				alias := "productenrich"
				if spec.Name != nil {
					alias = spec.Name.Name
				}
				aliases[alias] = struct{}{}
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "NewLLMManagerAdapter" {
					return true
				}
				qualifier, ok := selector.X.(*ast.Ident)
				if !ok {
					return true
				}
				if _, ok := aliases[qualifier.Name]; !ok {
					return true
				}
				rel, err := filepath.Rel("..", path)
				if err != nil {
					t.Error(err)
					return true
				}
				rel = filepath.ToSlash(rel)
				callers[rel]++
				if len(call.Args) != 2 {
					t.Errorf("%s: NewLLMManagerAdapter arguments = %d, want config and component logger", fset.Position(call.Pos()), len(call.Args))
					return true
				}
				if logger, ok := call.Args[1].(*ast.Ident); ok && logger.Name == "nil" {
					t.Errorf("%s: NewLLMManagerAdapter uses nil production logger", fset.Position(call.Pos()))
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	want := map[string]int{
		"hack/debug/test-analyzeimage/main.go":      1,
		"hack/debug/test-productenrich/adapters.go": 1,
	}
	if !reflect.DeepEqual(callers, want) {
		t.Fatalf("NewLLMManagerAdapter production callers = %v, want %v", callers, want)
	}
}

func TestOpenAIConfigsExposeExplicitNonSerializedLogger(t *testing.T) {
	for _, config := range []any{
		openaiclient.ClientConfig{}, openaiclient.PoolConfig{}, openaiclient.CachedClientConfig{},
		openaiclient.ManagerConfig{}, openaiclient.ResilientClientConfig{},
	} {
		configType := reflect.TypeOf(config)
		field, ok := configType.FieldByName("Logger")
		if !ok {
			t.Errorf("%s has no Logger field", configType.Name())
			continue
		}
		if field.Type != reflect.TypeOf((*openaiclient.Logger)(nil)).Elem() || field.Tag.Get("json") != "-" {
			t.Errorf("%s.Logger = %s json:%q", configType.Name(), field.Type, field.Tag.Get("json"))
		}
	}
	grsaiType := reflect.TypeOf(grsai.Config{})
	if field, ok := grsaiType.FieldByName("Logger"); !ok || field.Type != reflect.TypeOf((*grsai.Logger)(nil)).Elem() {
		t.Errorf("grsai.Config.Logger missing or wrong type")
	}
	if _, ok := reflect.TypeOf(geminiimage.Config{}).FieldByName("Logger"); ok {
		t.Error("geminiimage.Config must not carry an unused Logger field")
	}
}

func TestIntegrationProviderLoggingUsesOnlyLocalExplicitPorts(t *testing.T) {
	providerRoot := filepath.Join("..", "internal", "integration")
	for _, name := range []string{"openai", "geminiimage", "grsai"} {
		root := filepath.Join(providerRoot, name)
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			logrusAliases := make(map[string]struct{})
			for _, spec := range file.Imports {
				importPath, err := decodeGoImportPath(spec.Path.Value)
				if err != nil {
					return err
				}
				if importMatchesPrefix(importPath, "task-processor/internal/core/logger") ||
					importMatchesPrefix(importPath, "task-processor/internal/platform") ||
					importMatchesPrefix(importPath, "task-processor/internal/infra") {
					t.Errorf("%s imports forbidden logging/runtime package %s", filepath.ToSlash(path), importPath)
				}
				if importPath == "github.com/sirupsen/logrus" {
					alias := "logrus"
					if spec.Name != nil {
						alias = spec.Name.Name
					}
					logrusAliases[alias] = struct{}{}
				}
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				qualifier, ok := selector.X.(*ast.Ident)
				if !ok {
					return true
				}
				if _, ok := logrusAliases[qualifier.Name]; !ok {
					return true
				}
				switch selector.Sel.Name {
				case "StandardLogger", "WithField", "WithFields", "WithError", "Debug", "Debugf", "Info", "Infof", "Warn", "Warnf", "Error", "Errorf":
					t.Errorf("%s calls forbidden package-global logrus.%s", filepath.ToSlash(path), selector.Sel.Name)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestHTTPImageResolverIsImmutableFunction(t *testing.T) {
	path := filepath.Join("..", "internal", "integration", "httpimage", "client.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	functions := 0
	ast.Inspect(file, func(node ast.Node) bool {
		switch declaration := node.(type) {
		case *ast.FuncDecl:
			if declaration.Name.Name == "resolvePublicImageHostIPs" {
				functions++
			}
		case *ast.ValueSpec:
			for _, name := range declaration.Names {
				if name.Name == "resolvePublicImageHostIPs" {
					t.Error("resolvePublicImageHostIPs must not be a mutable package variable")
				}
			}
		}
		return true
	})
	if functions != 1 {
		t.Fatalf("resolvePublicImageHostIPs function declarations = %d, want 1", functions)
	}
}

func TestOpenAILoggerConfigFieldsAreASTComplete(t *testing.T) {
	want := map[string]bool{
		"ClientConfig": false, "PoolConfig": false, "CachedClientConfig": false,
		"ManagerConfig": false, "ResilientClientConfig": false,
	}
	root := filepath.Join("..", "internal", "integration", "openai")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.TYPE {
				continue
			}
			for _, rawSpec := range generic.Specs {
				typeSpec := rawSpec.(*ast.TypeSpec)
				if _, tracked := want[typeSpec.Name.Name]; !tracked {
					continue
				}
				structure, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					t.Errorf("%s is not a struct", typeSpec.Name.Name)
					continue
				}
				for _, field := range structure.Fields.List {
					if len(field.Names) != 1 || field.Names[0].Name != "Logger" {
						continue
					}
					identifier, ok := field.Type.(*ast.Ident)
					if !ok || identifier.Name != "Logger" {
						t.Errorf("%s.Logger is not local Logger", typeSpec.Name.Name)
					}
					if field.Tag == nil || field.Tag.Value != "`json:\"-\"`" {
						t.Errorf("%s.Logger json tag = %v", typeSpec.Name.Name, field.Tag)
					}
					want[typeSpec.Name.Name] = true
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, found := range want {
		if !found {
			t.Errorf("%s missing Logger Logger `json:\"-\"`", name)
		}
	}
}
