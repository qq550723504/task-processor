package listingadmin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/shein/generationtopics"
)

type GenerationTopicCatalogHandler struct {
	repo GenerationTopicOverrideRepository
}

type GenerationTopicCatalogPage struct {
	Items []GenerationTopicCatalogItem `json:"items"`
}

type GenerationTopicCatalogItem struct {
	Key                 string                        `json:"key"`
	Priority            int                           `json:"priority"`
	PromptDirectives    []string                      `json:"promptDirectives,omitempty"`
	LexiconByLanguage   map[string][]string           `json:"lexiconByLanguage,omitempty"`
	TenantOverride      *GenerationTopicOverrideView  `json:"tenantOverride"`
	EffectiveDefinition GenerationTopicDefinitionView `json:"effectiveDefinition"`
}

type GenerationTopicOverrideView struct {
	ID                          int64               `json:"id,omitempty"`
	Status                      int16               `json:"status"`
	Remark                      string              `json:"remark,omitempty"`
	AdditionalPromptDirectives  []string            `json:"additionalPromptDirectives,omitempty"`
	AdditionalLexiconByLanguage map[string][]string `json:"additionalLexiconByLanguage,omitempty"`
}

type GenerationTopicDefinitionView struct {
	PromptDirectives  []string            `json:"promptDirectives,omitempty"`
	LexiconByLanguage map[string][]string `json:"lexiconByLanguage,omitempty"`
}

func NewGenerationTopicCatalogHandler(repo GenerationTopicOverrideRepository) *GenerationTopicCatalogHandler {
	return &GenerationTopicCatalogHandler{repo: repo}
}

func (h *GenerationTopicCatalogHandler) ListGenerationTopicCatalog(c *gin.Context) {
	platform := generationtopics.NormalizeKey(c.Query("platform"))
	if platform == "" {
		writeValidationError(c, "invalid_generation_topic_catalog", errors.New("platform is required"))
		return
	}
	if platform != "shein" {
		writeValidationError(c, "invalid_generation_topic_catalog", errors.New("platform must be shein"))
		return
	}

	definitions, _ := generationtopics.ResolveSheinTopicKeys(knownSheinTopicKeys())
	items := make([]GenerationTopicCatalogItem, 0, len(definitions))
	tenantID := requestTenantID(c)
	reqCtx := requestIdentityContext(c)
	for _, definition := range definitions {
		item := GenerationTopicCatalogItem{
			Key:               definition.Key,
			Priority:          definition.Priority,
			PromptDirectives:  append([]string(nil), definition.PromptDirectives...),
			LexiconByLanguage: cloneStringMapList(definition.LexiconByLanguage),
			EffectiveDefinition: GenerationTopicDefinitionView{
				PromptDirectives:  append([]string(nil), definition.PromptDirectives...),
				LexiconByLanguage: cloneStringMapList(definition.LexiconByLanguage),
			},
		}
		if h.repo != nil && tenantID > 0 {
			override, err := h.repo.GetGenerationTopicOverrideByTopicKey(reqCtx, tenantID, platform, definition.Key)
			if err != nil && !errors.Is(err, ErrGenerationTopicOverrideNotFound) {
				writeInternalHandlerError(c, "generation_topic_catalog_list_failed", err)
				return
			}
			if err == nil && override != nil {
				item.TenantOverride = &GenerationTopicOverrideView{
					ID:                          override.ID,
					Status:                      override.Status,
					Remark:                      override.Remark,
					AdditionalPromptDirectives:  append([]string(nil), override.AdditionalPromptDirectives...),
					AdditionalLexiconByLanguage: cloneStringMapList(override.AdditionalLexiconByLanguage),
				}
				item.EffectiveDefinition = mergeCatalogDefinition(definition, override)
			}
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, GenerationTopicCatalogPage{Items: items})
}

func knownSheinTopicKeys() []string {
	definitions := generationtopics.SheinGenerationTopicDefinitions()
	keys := make([]string, 0, len(definitions))
	for key := range definitions {
		keys = append(keys, strings.TrimSpace(key))
	}
	return keys
}

func mergeCatalogDefinition(definition generationtopics.Definition, override *GenerationTopicOverride) GenerationTopicDefinitionView {
	merged := GenerationTopicDefinitionView{
		PromptDirectives:  append([]string(nil), definition.PromptDirectives...),
		LexiconByLanguage: cloneStringMapList(definition.LexiconByLanguage),
	}
	if override == nil || override.Status != 1 {
		return merged
	}
	merged.PromptDirectives = mergeStringLists(merged.PromptDirectives, override.AdditionalPromptDirectives)
	merged.LexiconByLanguage = mergeLexiconMaps(merged.LexiconByLanguage, override.AdditionalLexiconByLanguage)
	return merged
}

func mergeStringLists(base []string, additions []string) []string {
	return normalizeStringList(append(append([]string(nil), base...), additions...))
}

func mergeLexiconMaps(base map[string][]string, additions map[string][]string) map[string][]string {
	merged := cloneStringMapList(base)
	if merged == nil {
		merged = map[string][]string{}
	}
	for language, words := range additions {
		merged[language] = mergeStringLists(merged[language], words)
	}
	return normalizeLexiconMap(merged)
}

func cloneStringMapList(values map[string][]string) map[string][]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string][]string, len(values))
	for key, items := range values {
		cloned[key] = append([]string(nil), items...)
	}
	return cloned
}
