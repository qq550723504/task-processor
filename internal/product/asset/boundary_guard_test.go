package asset_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestProductionCodeDoesNotImportAssetTestHelpers(t *testing.T) {
	t.Parallel()

	assetDir := currentPackageDir(t)
	internalDir := filepath.Clean(filepath.Join(assetDir, "..", ".."))
	err := filepath.WalkDir(internalDir, func(path string, entry fs.DirEntry, walkErr error) error {
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
			if value == "task-processor/internal/product/asset/assettest" {
				t.Errorf("production file %s imports test-only assettest package", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
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
