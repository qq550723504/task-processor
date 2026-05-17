package promptmgmt

import (
	"regexp"
	"slices"
	"strings"

	"task-processor/internal/prompt"
)

var templateVariablePattern = regexp.MustCompile(`\{\{\s*\.(\w+)\s*\}\}`)

var promptTemplateCatalogKeys = []string{
	prompt.KSheinAttributeSelectorSystem,
	prompt.KSheinCategorySelectorSelectCategorySystem,
	prompt.KSheinCategorySelectorSelectCategoryUser,
	prompt.KSheinCategorySelectorExtractCoreItemSystem,
	prompt.KSheinCategorySelectorSemanticValidation,
	prompt.KSheinContentOptimizerOptimizeTitleDescriptionSystem,
	prompt.KSheinContentOptimizerOptimizeTitleDescriptionUser,
	prompt.KSheinDisplayAttributeFieldSelection,
	prompt.KSheinDisplayAttributeValueMapping,
	prompt.KSheinDisplayAttributeValueMappingBatch,
	prompt.KSheinDisplayAttributeMissingText,
	prompt.KSheinDisplayAttributeMissingValue,
	prompt.KSheinDisplayAttributeFieldSelectionBatch,
	prompt.KSheinDisplayAttributeBatchInference,
	prompt.KSheinDisplayAttributeRequiredRepair,
	prompt.KSheinSaleAttributeMapping,
	prompt.KSheinSaleAttributeSourceDimension,
	prompt.KSheinSaleAttributeValueBatchMapping,
	prompt.KSheinSaleAttributePromptValueExtraction,
	prompt.KSheinTranslationBatchOptimizeSystem,
	prompt.KTemuAttributeMappingSystem,
	prompt.KTemuContentRewriterSystem,
	prompt.KTemuSkuVariantMappingSystem,
	prompt.KTemuVisionDetectorDetect,
	prompt.KProductEnrichLlmScorerTextScoring,
	prompt.KProductEnrichLlmScorerImageScoring,
	prompt.KProductEnrichUnderstandingAnalyzeImage,
	prompt.KProductEnrichUnderstandingExtractText,
	prompt.KProductEnrichUnderstandingFuseMultimodal,
	prompt.KProductEnrichGenerationProductJSON,
	prompt.KProductEnrichGenerationSpecs,
	prompt.KProductEnrichGenerationVariants,
	prompt.KProductEnrichGenerationExtractDimensions,
	prompt.KProductEnrichGenerationExtractWeight,
	prompt.KProductImageSubjectExtract,
	prompt.KProductImageWhiteBackgroundDefault,
	prompt.KProductImageSceneDefault,
	prompt.KProductImageSceneShoes,
	prompt.KProductImageSceneJewelry,
	prompt.KProductImageSceneBags,
	prompt.KProductImageReviewDefault,
	prompt.KProductImageStudioGenerationPodDesign,
	prompt.KProductImageStudioGenerationAmazonProductImage,
}

var promptCategoryLabels = map[string]string{
	"shein":         "SHEIN",
	"temu":          "Temu",
	"productenrich": "商品信息生成",
	"productimage":  "商品图片",
}

var promptGroupLabels = map[string]string{
	"marketplace": "平台运营",
	"enrichment":  "商品理解",
	"image":       "图片生成",
}

func buildTemplateCatalog() []TemplateSchema {
	defaultContentByKey := defaultPromptContentByKey()
	items := make([]TemplateSchema, 0, len(promptTemplateCatalogKeys))
	for _, key := range promptTemplateCatalogKeys {
		items = append(items, buildTemplateSchema(key, defaultContentByKey[key]))
	}
	return items
}

func defaultPromptContentByKey() map[string]string {
	if prompt.GlobalRegistry == nil {
		return map[string]string{}
	}
	keys := prompt.GlobalRegistry.Keys()
	contentByKey := make(map[string]string, len(keys))
	for _, key := range keys {
		contentByKey[key] = prompt.GlobalRegistry.Get(key, "")
	}
	return contentByKey
}

func buildTemplateSchema(key string, defaultContent string) TemplateSchema {
	category := promptCategory(key)
	group := promptGroup(category)
	metadata := promptMetadataForKey(key)
	scopes := promptScopesForSchema(metadata)
	return TemplateSchema{
		Key:                    key,
		Label:                  metadata.Label,
		Description:            metadata.Description,
		Group:                  group,
		GroupLabel:             promptGroupLabel(group),
		Category:               category,
		CategoryLabel:          promptCategoryLabel(category),
		SupportedScopes:        scopes,
		Variables:              promptVariablesForSchema(metadata, defaultContent),
		HasDefaultContent:      strings.TrimSpace(defaultContent) != "",
		SupportsTenantOverride: supportsScope(scopes, "tenant"),
	}
}

func promptMetadataForKey(key string) promptMetadata {
	if metadata, ok := promptCatalogMetadata[key]; ok {
		return metadata
	}
	return promptMetadata{
		Label:       promptLabel(key),
		Description: promptDescription(key),
	}
}

func promptCategory(key string) string {
	head, _, _ := strings.Cut(key, ".")
	return head
}

func promptCategoryLabel(category string) string {
	if label, ok := promptCategoryLabels[category]; ok {
		return label
	}
	return humanizeToken(category)
}

func promptGroup(category string) string {
	switch category {
	case "shein", "temu":
		return "marketplace"
	case "productenrich":
		return "enrichment"
	case "productimage":
		return "image"
	default:
		return "workflow"
	}
}

func promptGroupLabel(group string) string {
	if label, ok := promptGroupLabels[group]; ok {
		return label
	}
	return humanizeToken(group)
}

func promptLabel(key string) string {
	parts := strings.Split(key, ".")
	if len(parts) <= 1 {
		return humanizeToken(key)
	}
	labels := make([]string, 0, len(parts)-1)
	for _, part := range parts[1:] {
		labels = append(labels, humanizeToken(part))
	}
	return strings.Join(labels, " / ")
}

func promptDescription(key string) string {
	category := promptCategory(key)
	return promptGroupLabel(promptGroup(category)) + " / " + promptCategoryLabel(category) + " 提示词模板，可被租户覆盖。"
}

func extractPromptVariables(content string) []TemplateVariableDefinition {
	matches := templateVariablePattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	variables := make([]TemplateVariableDefinition, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		key := strings.TrimSpace(match[1])
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		variables = append(variables, TemplateVariableDefinition{
			Key:   key,
			Label: humanizeToken(key),
		})
	}
	slices.SortFunc(variables, func(a, b TemplateVariableDefinition) int {
		return strings.Compare(a.Key, b.Key)
	})
	return variables
}

func promptVariablesForSchema(metadata promptMetadata, defaultContent string) []TemplateVariableDefinition {
	if len(metadata.Variables) > 0 {
		return slices.Clone(metadata.Variables)
	}
	return extractPromptVariables(defaultContent)
}

func promptScopesForSchema(metadata promptMetadata) []TemplateScopeDefinition {
	if len(metadata.Scopes) > 0 {
		return slices.Clone(metadata.Scopes)
	}
	return nil
}

func supportsScope(scopes []TemplateScopeDefinition, scopeID string) bool {
	for _, scope := range scopes {
		if scope.ID == scopeID {
			return true
		}
	}
	return false
}

func humanizeToken(raw string) string {
	raw = strings.ReplaceAll(raw, "_", " ")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	words := strings.Fields(raw)
	for i, word := range words {
		if len(word) == 0 {
			continue
		}
		if strings.ToUpper(word) == word {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}
