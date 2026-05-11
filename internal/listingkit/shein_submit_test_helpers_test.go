package listingkit

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/submitprep"
)

func findSheinLanguageContent(items []sheinproduct.LanguageContent, language string) string {
	return submitprep.FindLanguageContent(items, language)
}

func localizedSubmitSnapshotText(items []sheinpub.LocalizedText, language string) string {
	for _, item := range items {
		if submitprep.NormalizeLanguage(item.Language) == submitprep.NormalizeLanguage(language) {
			return item.Name
		}
	}
	return ""
}

func overrideSensitiveWordsConfigForTest(t *testing.T) func() {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	sourcePath := filepath.Join(filepath.Dir(file), "..", "..", "data", "sensitive_words_shein.json")
	bytes, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read sensitive words config: %v", err)
	}
	tempPath := filepath.Join(t.TempDir(), "sensitive_words_shein.json")
	if err := os.WriteFile(tempPath, bytes, 0o600); err != nil {
		t.Fatalf("write temp sensitive words config: %v", err)
	}
	return submitprep.SetSensitiveWordsConfigPathForTesting(tempPath)
}
