package listingkit

import (
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
	return func() {}
}
