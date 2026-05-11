package submitprep

import (
	"path/filepath"
	"runtime"
	"sync"

	sheinctx "task-processor/internal/shein/context"
	sheincontent "task-processor/internal/shein/content"
	sheinproduct "task-processor/internal/shein/api/product"
)

var (
	sensitiveWordsPathMu       sync.RWMutex
	sensitiveWordsPathOverride string
)

func CleanSensitiveWords(product *sheinproduct.Product) error {
	if product == nil {
		return nil
	}
	service := sheincontent.NewSensitiveWordServiceWithPath(sensitiveWordsConfigPath())
	taskCtx := &sheinctx.TaskContext{}
	taskCtx.ProductData = product
	return service.ProcessProductData(taskCtx)
}

func RetrySensitiveWordCleanup(product *sheinproduct.Product, validationNotes []string) bool {
	if product == nil || len(validationNotes) == 0 {
		return false
	}
	service := sheincontent.NewSensitiveWordServiceWithPath(sensitiveWordsConfigPath())
	taskCtx := &sheinctx.TaskContext{}
	taskCtx.ProductData = product
	results := []sheinctx.PreValidResult{{Messages: append([]string(nil), validationNotes...)}}
	return service.HandleValidationErrors(taskCtx, results)
}

func sensitiveWordsConfigPath() string {
	sensitiveWordsPathMu.RLock()
	override := sensitiveWordsPathOverride
	sensitiveWordsPathMu.RUnlock()
	if override != "" {
		return override
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "data/sensitive_words_shein.json"
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "data", "sensitive_words_shein.json")
}

func SetSensitiveWordsConfigPathForTesting(path string) func() {
	sensitiveWordsPathMu.Lock()
	previous := sensitiveWordsPathOverride
	sensitiveWordsPathOverride = path
	sensitiveWordsPathMu.Unlock()
	return func() {
		sensitiveWordsPathMu.Lock()
		sensitiveWordsPathOverride = previous
		sensitiveWordsPathMu.Unlock()
	}
}
