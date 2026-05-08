package tests

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListingKitDoesNotImportLegacySheinRuntime(t *testing.T) {
	root := filepath.Join("..", "internal", "listingkit")
	bannedImports := []string{
		`"task-processor/internal/shein/pipeline"`,
		`"task-processor/internal/shein/publish"`,
		`"task-processor/internal/shein/product/build"`,
	}

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(content)
		for _, banned := range bannedImports {
			if strings.Contains(text, banned) {
				t.Errorf("%s imports legacy SHEIN runtime package %s", path, banned)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
