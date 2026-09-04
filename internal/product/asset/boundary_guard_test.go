package asset_test

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestModuleRootProductionImportScanFindsEntrypointsAndSkipsExternalTrees(t *testing.T) {
	t.Parallel()

	moduleRoot := t.TempDir()
	writeBoundaryFixture(t, filepath.Join(moduleRoot, "go.mod"), "module example.test/fixture\n\ngo 1.26\n")
	writeBoundaryFixture(t, filepath.Join(moduleRoot, "cmd", "server", "main.go"), "package main\nimport _ \"task-processor/internal/product/asset/assettest\"\n")
	writeBoundaryFixture(t, filepath.Join(moduleRoot, "internal", "safe.go"), "package internal\nimport \"context\"\nvar _ = context.Background\n")
	for _, directory := range []string{".git", ".worktrees", ".cache", "_deps", "vendor", "node_modules", "testdata", "third_party"} {
		writeBoundaryFixture(t, filepath.Join(moduleRoot, directory, "ignored.go"), "package ignored\nimport _ \"task-processor/internal/product/asset/assettest\"\n")
	}
	writeBoundaryFixture(t, filepath.Join(moduleRoot, "external-tool", "go.mod"), "module example.test/external\n\ngo 1.26\n")
	writeBoundaryFixture(t, filepath.Join(moduleRoot, "external-tool", "ignored.go"), "package external\nimport _ \"task-processor/internal/product/asset/assettest\"\n")

	foundRoot, err := moduleRootFrom(filepath.Join(moduleRoot, "cmd", "server"))
	if err != nil {
		t.Fatal(err)
	}
	if foundRoot != moduleRoot {
		t.Fatalf("moduleRootFrom() = %q, want %q", foundRoot, moduleRoot)
	}

	violations, err := assetTestProductionImports(moduleRoot)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join("cmd", "server", "main.go")}
	if !reflect.DeepEqual(violations, want) {
		t.Fatalf("assetTestProductionImports() = %v, want %v", violations, want)
	}
}

func writeBoundaryFixture(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProductionCodeDoesNotImportAssetTestHelpers(t *testing.T) {
	t.Parallel()

	assetDir := currentPackageDir(t)
	moduleRoot, err := moduleRootFrom(assetDir)
	if err != nil {
		t.Fatal(err)
	}
	violations, err := assetTestProductionImports(moduleRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range violations {
		t.Errorf("production file %s imports test-only assettest package", path)
	}
}

func moduleRootFrom(start string) (string, error) {
	directory := filepath.Clean(start)
	for {
		if info, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil && !info.IsDir() {
			return directory, nil
		} else if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", fmt.Errorf("go.mod not found above %s", start)
		}
		directory = parent
	}
}

func assetTestProductionImports(moduleRoot string) ([]string, error) {
	var violations []string
	err := filepath.WalkDir(moduleRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path == moduleRoot {
				return nil
			}
			if excludedModuleScanDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			if info, err := os.Stat(filepath.Join(path, "go.mod")); err == nil && !info.IsDir() {
				return filepath.SkipDir
			} else if err != nil && !os.IsNotExist(err) {
				return err
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse imports from %s: %w", path, err)
		}
		for _, imported := range parsed.Imports {
			value, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			if value != "task-processor/internal/product/asset/assettest" {
				continue
			}
			relative, err := filepath.Rel(moduleRoot, path)
			if err != nil {
				return err
			}
			violations = append(violations, relative)
		}
		return nil
	})
	sort.Strings(violations)
	return violations, err
}

func excludedModuleScanDirectory(name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return true
	}
	switch name {
	case "vendor", "node_modules", "testdata", "third_party":
		return true
	default:
		return false
	}
}

func TestProductAssetDoesNotDependOnImageOrPersistence(t *testing.T) {
	t.Parallel()

	assetDir := currentPackageDir(t)
	banned := []string{
		"task-processor/internal/product/" + "image",
		"task-processor/internal/" + "product" + "image",
		"gorm" + ".io/",
	}
	err := filepath.WalkDir(assetDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range parsed.Imports {
			value, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			for _, prefix := range banned {
				if strings.HasPrefix(value, prefix) {
					t.Errorf("product asset file %s imports banned dependency %s", path, value)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func currentPackageDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve package directory")
	}
	return filepath.Dir(filename)
}
