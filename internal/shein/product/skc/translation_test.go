package skc

import (
	"context"
	"strings"
	"testing"

	"task-processor/internal/shein/aicache"
	"task-processor/internal/shein/api/product"
)

func TestOptimizeMultiLanguageContent_TruncatesEnglishTitlesToSheinLimit(t *testing.T) {
	t.Parallel()

	cache := aicache.New(nil)
	overlong := strings.Repeat("A", sheinSKCTitleMaxLength+25)
	cache.Set(aicache.TypeSKCTranslate, aicache.HashKey(overlong), []string{overlong})

	handler := &SKCTranslationHandler{runtime: &SKCRuntimeInput{AICache: cache}}
	items := []product.LanguageContent{{
		Language: "en",
		Name:     overlong,
	}}

	handler.optimizeMultiLanguageContent(context.Background(), &items, "source title")

	if len(items[0].Name) != sheinSKCTitleMaxLength {
		t.Fatalf("english title length = %d, want %d", len(items[0].Name), sheinSKCTitleMaxLength)
	}
}
