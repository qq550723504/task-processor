package submitprep

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"task-processor/internal/listingadmin"
	"task-processor/internal/shared/tenantctx"
	sheinproduct "task-processor/internal/shein/api/product"
	sheincontent "task-processor/internal/shein/content"
	sheinctx "task-processor/internal/shein/context"
	generationtopics "task-processor/internal/shein/generationtopics"
)

var (
	sensitiveWordRepoMu           sync.RWMutex
	sensitiveWordRepo             listingadmin.SensitiveWordRepository
	generationTopicRepoMu         sync.RWMutex
	generationTopicRepo           listingadmin.GenerationTopicPolicyRepository
	generationTopicOverrideRepoMu sync.RWMutex
	generationTopicOverrideRepo   listingadmin.GenerationTopicOverrideRepository
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
	service := sheincontent.NewSensitiveWordServiceInMemory()
	overlaySensitiveWordsFromRepository(ctx, service)
	overlayGenerationTopicLexicons(ctx, service)
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
	persistValidationSensitiveWords(ctx, service, validationNotes)
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

func SetGenerationTopicPolicyRepository(repo listingadmin.GenerationTopicPolicyRepository) func() {
	generationTopicRepoMu.Lock()
	previous := generationTopicRepo
	generationTopicRepo = repo
	generationTopicRepoMu.Unlock()
	return func() {
		generationTopicRepoMu.Lock()
		generationTopicRepo = previous
		generationTopicRepoMu.Unlock()
	}
}

func currentGenerationTopicPolicyRepository() listingadmin.GenerationTopicPolicyRepository {
	generationTopicRepoMu.RLock()
	repo := generationTopicRepo
	generationTopicRepoMu.RUnlock()
	return repo
}

func SetGenerationTopicOverrideRepository(repo listingadmin.GenerationTopicOverrideRepository) func() {
	generationTopicOverrideRepoMu.Lock()
	previous := generationTopicOverrideRepo
	generationTopicOverrideRepo = repo
	generationTopicOverrideRepoMu.Unlock()
	return func() {
		generationTopicOverrideRepoMu.Lock()
		generationTopicOverrideRepo = previous
		generationTopicOverrideRepoMu.Unlock()
	}
}

func currentGenerationTopicOverrideRepository() listingadmin.GenerationTopicOverrideRepository {
	generationTopicOverrideRepoMu.RLock()
	repo := generationTopicOverrideRepo
	generationTopicOverrideRepoMu.RUnlock()
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

func overlayGenerationTopicLexicons(ctx context.Context, service *sheincontent.SensitiveWordService) {
	if ctx == nil || service == nil {
		return
	}
	repo := currentGenerationTopicPolicyRepository()
	if repo == nil {
		return
	}
	tenantID, ok := tenantIDFromContext(ctx)
	if !ok {
		return
	}
	keys, err := repo.ListEnabledTopicKeys(ctx, tenantID, "shein")
	if err != nil || len(keys) == 0 {
		return
	}
	overrideRepo := currentGenerationTopicOverrideRepository()
	var definitions []generationtopics.Definition
	if overrideRepo == nil {
		definitions, _ = generationtopics.ResolveSheinTopicKeys(keys)
	} else {
		definitions = generationtopics.ResolveSheinTopicDefinitionsWithOverlay(keys, func(topicKey string) (generationtopics.DefinitionOverlay, error) {
			override, err := overrideRepo.GetGenerationTopicOverrideByTopicKey(ctx, tenantID, "shein", topicKey)
			if err != nil || override == nil || override.Status != 1 {
				return generationtopics.DefinitionOverlay{}, err
			}
			return generationtopics.DefinitionOverlay{
				Enabled:           true,
				PromptDirectives:  override.AdditionalPromptDirectives,
				LexiconByLanguage: override.AdditionalLexiconByLanguage,
			}, nil
		})
	}
	if len(definitions) == 0 {
		return
	}
	for language, words := range generationtopics.CollectLexiconsFromDefinitions(definitions) {
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
	records, err := loadTenantSensitiveWordRecords(ctx, repo, tenantID, int16Ptr(1))
	if err != nil {
		return nil, err
	}
	wordsByLanguage := make(map[string][]string)
	seenByLanguage := make(map[string]map[string]struct{})
	for _, item := range records {
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
	return wordsByLanguage, nil
}

func loadTenantSensitiveWordRecords(ctx context.Context, repo listingadmin.SensitiveWordRepository, tenantID int64, status *int16) ([]listingadmin.SensitiveWord, error) {
	const pageSize = 200
	page := 1
	records := make([]listingadmin.SensitiveWord, 0, pageSize)
	for {
		result, err := repo.ListSensitiveWords(ctx, listingadmin.SensitiveWordQuery{
			TenantID: tenantID,
			Page:     page,
			PageSize: pageSize,
			Status:   status,
		})
		if err != nil {
			return nil, err
		}
		if result == nil || len(result.Items) == 0 {
			return records, nil
		}
		records = append(records, result.Items...)
		if len(result.Items) < result.PageSize || int64(result.Page*result.PageSize) >= result.Total {
			return records, nil
		}
		page++
	}
}

func persistValidationSensitiveWords(ctx context.Context, service *sheincontent.SensitiveWordService, validationNotes []string) {
	if ctx == nil || service == nil || len(validationNotes) == 0 {
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
	words := service.ExtractSensitiveWordsFromValidationNotes(validationNotes)
	if len(words) == 0 {
		return
	}
	existingRecords, err := loadTenantSensitiveWordRecords(ctx, repo, tenantID, nil)
	if err != nil {
		return
	}
	existingByKey := make(map[string]listingadmin.SensitiveWord, len(existingRecords))
	for _, record := range existingRecords {
		key := sensitiveWordRecordKey(record.Language, record.Word)
		if key == "" {
			continue
		}
		existingByKey[key] = record
	}
	for _, word := range words {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		language := detectSensitiveWordLanguage(word)
		key := sensitiveWordRecordKey(language, word)
		if key == "" {
			continue
		}
		if existing, ok := existingByKey[key]; ok {
			if existing.Status == 1 {
				continue
			}
			existing.Status = 1
			existing.Tags = mergeSensitiveWordTags(existing.Tags, "shein,auto-discovered,validation-retry")
			existing.Remark = mergeSensitiveWordRemark(existing.Remark, autoDiscoveredSensitiveWordRemark())
			if updated, updateErr := repo.UpdateSensitiveWord(ctx, &existing); updateErr == nil && updated != nil {
				existingByKey[key] = *updated
			}
			continue
		}
		created, createErr := repo.CreateSensitiveWord(ctx, &listingadmin.SensitiveWord{
			TenantID: tenantID,
			Word:     word,
			Language: language,
			Tags:     "shein,auto-discovered,validation-retry",
			Level:    2,
			Remark:   autoDiscoveredSensitiveWordRemark(),
			Status:   1,
		})
		if createErr == nil && created != nil {
			existingByKey[key] = *created
		}
	}
}

func detectSensitiveWordLanguage(word string) string {
	for _, r := range word {
		switch {
		case unicode.Is(unicode.Han, r):
			return "zh"
		case unicode.In(r, unicode.Hiragana, unicode.Katakana):
			return "ja"
		case unicode.In(r, unicode.Hangul):
			return "ko"
		case unicode.IsLetter(r):
			return "en"
		}
	}
	return "en"
}

func sensitiveWordRecordKey(language, word string) string {
	language = strings.TrimSpace(strings.ToLower(language))
	word = strings.TrimSpace(strings.ToLower(word))
	if language == "" || word == "" {
		return ""
	}
	return language + "\x00" + word
}

func mergeSensitiveWordTags(existing, added string) string {
	parts := append(splitCSV(existing), splitCSV(added)...)
	seen := make(map[string]struct{}, len(parts))
	merged := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key := strings.ToLower(part)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, part)
	}
	return strings.Join(merged, ",")
}

func mergeSensitiveWordRemark(existing, added string) string {
	existing = strings.TrimSpace(existing)
	added = strings.TrimSpace(added)
	switch {
	case existing == "":
		return added
	case added == "":
		return existing
	case strings.Contains(strings.ToLower(existing), strings.ToLower(added)):
		return existing
	default:
		return existing + "; " + added
	}
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(value, ",")
}

func autoDiscoveredSensitiveWordRemark() string {
	return "auto discovered from SHEIN validation retry on " + time.Now().Format("2006-01-02")
}

func int16Ptr(value int16) *int16 {
	return &value
}
