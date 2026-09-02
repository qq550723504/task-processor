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
	s3integration "task-processor/internal/integration/s3"
)

func TestIntegrationProviderLoggerMethodSetsMatch(t *testing.T) {
	interfaces := []struct {
		name   string
		typeOf reflect.Type
	}{
		{"openai", reflect.TypeOf((*openaiclient.Logger)(nil)).Elem()},
		{"geminiimage", reflect.TypeOf((*geminiimage.Logger)(nil)).Elem()},
		{"grsai", reflect.TypeOf((*grsai.Logger)(nil)).Elem()},
		{"s3", reflect.TypeOf((*s3integration.Logger)(nil)).Elem()},
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
		"hack/debug/replay-sale-attribute/main.go":             {openAI: 1},
		"internal/amazon/processor.go":                         {openAI: 1},
		"internal/app/httpapi/adapters_openai.go":              {openAI: 1},
		"internal/app/worker/imageagent/dependencies.go":       {openAI: 1},
		"internal/listingkit/httpapi/ai_client_builders.go":    {openAI: 1, grsai: 2, gemini: 2},
		"internal/listingkit/httpapi/ai_client_strict_chat.go": {openAI: 1},
		"internal/shein/pipeline/pipeline.go":                  {openAI: 1},
		"internal/temu/pipeline_registry.go":                   {openAI: 1},
		"internal/temu/sku/ai_mapping_handler.go":              {openAI: 1},
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
	want := map[string]int{}
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
