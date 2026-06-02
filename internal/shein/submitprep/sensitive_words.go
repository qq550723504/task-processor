package submitprep

import (
	"context"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit/tenantctx"
	sheinproduct "task-processor/internal/shein/api/product"
	sheincontent "task-processor/internal/shein/content"
	sheinctx "task-processor/internal/shein/context"
)

var (
	sensitiveWordsPathMu       sync.RWMutex
	sensitiveWordsPathOverride string
	sensitiveWordRepoMu        sync.RWMutex
	sensitiveWordRepo          listingadmin.SensitiveWordRepository
)

func CleanSensitiveWords(product *sheinproduct.Product) error {
	return CleanSensitiveWordsWithContext(context.Background(), product)
}

func CleanSensitiveWordsWithContext(ctx context.Context, product *sheinproduct.Product) error {
	if product == nil {
		return nil
	}
	service := NewSensitiveWordServiceForContext(ctx)
	taskCtx := &sheinctx.TaskContext{}
	taskCtx.ProductData = product
	return service.ProcessProductData(taskCtx)
}

func NewSensitiveWordService() *sheincontent.SensitiveWordService {
	return NewSensitiveWordServiceForContext(context.Background())
}

func NewSensitiveWordServiceForContext(ctx context.Context) *sheincontent.SensitiveWordService {
	service := sheincontent.NewSensitiveWordServiceWithPath(sensitiveWordsConfigPath())
	overlaySensitiveWordsFromRepository(ctx, service)
	return service
}

func RetrySensitiveWordCleanup(product *sheinproduct.Product, validationNotes []string) bool {
	return RetrySensitiveWordCleanupWithContext(context.Background(), product, validationNotes)
}

func RetrySensitiveWordCleanupWithContext(ctx context.Context, product *sheinproduct.Product, validationNotes []string) bool {
	if product == nil || len(validationNotes) == 0 {
		return false
	}
	service := NewSensitiveWordServiceForContext(ctx)
	taskCtx := &sheinctx.TaskContext{}
	taskCtx.ProductData = product
	results := []sheinctx.PreValidResult{{Messages: append([]string(nil), validationNotes...)}}
	return service.HandleValidationErrors(taskCtx, results)
}

func SetSensitiveWordRepository(repo listingadmin.SensitiveWordRepository) func() {
	sensitiveWordRepoMu.Lock()
	previous := sensitiveWordRepo
	sensitiveWordRepo = repo
	sensitiveWordRepoMu.Unlock()
	return func() {
		sensitiveWordRepoMu.Lock()
		sensitiveWordRepo = previous
		sensitiveWordRepoMu.Unlock()
	}
}

func currentSensitiveWordRepository() listingadmin.SensitiveWordRepository {
	sensitiveWordRepoMu.RLock()
	repo := sensitiveWordRepo
	sensitiveWordRepoMu.RUnlock()
	return repo
}

func overlaySensitiveWordsFromRepository(ctx context.Context, service *sheincontent.SensitiveWordService) {
	if ctx == nil || service == nil {
		return
	}
	repo := currentSensitiveWordRepository()
	if repo == nil {
		return
	}
	tenantID, ok := tenantIDFromContext(ctx)
	if !ok {
		return
	}
	wordsByLanguage, err := loadTenantSensitiveWords(ctx, repo, tenantID)
	if err != nil || len(wordsByLanguage) == 0 {
		return
	}
	for language, words := range wordsByLanguage {
		service.AddStaticSensitiveWordsByLanguage(language, words)
	}
}

func tenantIDFromContext(ctx context.Context) (int64, bool) {
	tenantID, ok := tenantctx.TenantScopeFromContext(ctx)
	if !ok {
		return 0, false
	}
	if tenantID = strings.TrimSpace(tenantID); tenantID == "" || tenantID == tenantctx.DefaultTenantID {
		return 0, false
	}
	parsed, err := strconv.ParseInt(tenantID, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}

func loadTenantSensitiveWords(ctx context.Context, repo listingadmin.SensitiveWordRepository, tenantID int64) (map[string][]string, error) {
	const pageSize = 200
	enabledStatus := int16(1)
	page := 1
	wordsByLanguage := make(map[string][]string)
	seenByLanguage := make(map[string]map[string]struct{})
	for {
		result, err := repo.ListSensitiveWords(ctx, listingadmin.SensitiveWordQuery{
			TenantID: tenantID,
			Page:     page,
			PageSize: pageSize,
			Status:   &enabledStatus,
		})
		if err != nil {
			return nil, err
		}
		if result == nil || len(result.Items) == 0 {
			return wordsByLanguage, nil
		}
		for _, item := range result.Items {
			language := strings.TrimSpace(item.Language)
			word := strings.TrimSpace(item.Word)
			if language == "" || word == "" {
				continue
			}
			languageSeen := seenByLanguage[language]
			if languageSeen == nil {
				languageSeen = make(map[string]struct{})
				seenByLanguage[language] = languageSeen
			}
			key := strings.ToLower(word)
			if _, exists := languageSeen[key]; exists {
				continue
			}
			languageSeen[key] = struct{}{}
			wordsByLanguage[language] = append(wordsByLanguage[language], word)
		}
		if len(result.Items) < result.PageSize || int64(result.Page*result.PageSize) >= result.Total {
			return wordsByLanguage, nil
		}
		page++
	}
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
