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
)

func TestS3IntegrationAndImageAgentBoundaries(t *testing.T) {
	assertNoBannedImportPrefixes(t, filepath.Join("..", "internal", "integration", "s3"), []string{
		"task-processor/internal/core/logger",
		"task-processor/internal/platform",
		"task-processor/internal/infra",
	}, nil)
	assertNoBannedImportPrefixes(t, filepath.Join("..", "internal", "imageagent"), []string{
		"task-processor/internal/infra/" + "storage",
		"task-processor/internal/integration/s3",
		"github.com/aws/aws-sdk-go",
	}, nil)
	assertNoBannedImportPrefixes(t, filepath.Join("..", "internal"), []string{
		"task-processor/internal/infra/" + "storage",
	}, nil)
}

func TestProductionS3UploaderConstructorInventoryIsComplete(t *testing.T) {
	got := scanProductionS3Uses(t, true)
	want := map[string]int{
		"internal/app/worker/imageagent/dependencies.go":      1,
		"internal/listingkit/httpapi/builders_image_store.go": 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NewUploaderWithOptions production callers = %v, want %v", got, want)
	}
}

func TestLegacyS3IntegrationConsumersRemainFrozen(t *testing.T) {
	got := scanProductionS3Uses(t, false)
	want := map[string]int{
		"internal/listingkit/httpapi/builders_image_store.go": 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy direct S3 integration consumers = %v, want frozen debt %v", got, want)
	}
}

func scanProductionS3Uses(t *testing.T, constructorsOnly bool) map[string]int {
	t.Helper()
	result := make(map[string]int)
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
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			aliases := make(map[string]struct{})
			for _, spec := range file.Imports {
				importPath, err := decodeGoImportPath(spec.Path.Value)
				if err != nil {
					return err
				}
				if importPath != "task-processor/internal/integration/s3" {
					continue
				}
				alias := "s3"
				if spec.Name != nil {
					alias = spec.Name.Name
				}
				aliases[alias] = struct{}{}
			}
			if len(aliases) == 0 {
				return nil
			}
			count := 0
			if constructorsOnly {
				ast.Inspect(file, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					selector, ok := call.Fun.(*ast.SelectorExpr)
					if !ok || selector.Sel.Name != "NewUploaderWithOptions" {
						return true
					}
					qualifier, ok := selector.X.(*ast.Ident)
					if ok {
						_, ok = aliases[qualifier.Name]
					}
					if ok {
						count++
					}
					return true
				})
			} else if strings.Contains(filepath.ToSlash(path), "/listingkit/") || strings.Contains(filepath.ToSlash(path), "/productimage/") {
				count = 1
			}
			if count > 0 {
				rel, err := filepath.Rel("..", path)
				if err != nil {
					return err
				}
				result[filepath.ToSlash(rel)] = count
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}
