package record

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestOnlyCurrentAdapterOwnsListingRecordTable(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		normalized := filepath.ToSlash(path)
		if strings.Contains(string(raw), "listing_shein_records") && !strings.Contains(normalized, "/app/listingrecordstore/") {
			t.Errorf("second table owner: %s", path)
		}
		if !strings.Contains(normalized, "/listing/record/") && !strings.Contains(normalized, "/marketplace/shein/draft/") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, raw, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, item := range file.Imports {
			pkg, _ := strconv.Unquote(item.Path.Value)
			if pkg == "task-processor/internal/listingkit" || strings.HasPrefix(pkg, "task-processor/internal/compatibility/") || strings.HasPrefix(pkg, "task-processor/internal/tenantbridge") {
				t.Errorf("legacy dependency in %s: %s", path, pkg)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
