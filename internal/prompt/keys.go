package prompt

// Prompt key 常量，与 prompts/ 目录下 YAML 文件的展平路径对应。
// 命名规则：K + 大驼峰路径，例如 shein.category_selector.select_category_system → KSheinCategorySelectorSelectCategorySystem
const (
	// ── shein/attribute_selector.yaml ──────────────────────────────────────
	KSheinAttributeSelectorSystem      = "shein.attribute_selector.system"
	KSheinAttributeValueFallbackSystem = "shein.attribute_value_fallback.system"
	KSheinAttributeValueFallbackUser   = "shein.attribute_value_fallback.user"

	// ── shein/category_selector.yaml ───────────────────────────────────────
	KSheinCategorySelectorSelectCategorySystem  = "shein.category_selector.select_category_system"
	KSheinCategorySelectorSelectCategoryUser    = "shein.category_selector.select_category_user"
	KSheinCategorySelectorExtractCoreItemSystem = "shein.category_selector.extract_core_item_system"
	KSheinCategorySelectorSemanticValidation    = "shein.category_selector.semantic_validation"

	// ── shein/content_optimizer.yaml ───────────────────────────────────────
	KSheinContentOptimizerOptimizeTitleDescriptionSystem = "shein.content_optimizer.optimize_title_description_system"
	KSheinContentOptimizerOptimizeTitleDescriptionUser   = "shein.content_optimizer.optimize_title_description_user"

	// ── shein/display_attribute.yaml ───────────────────────────────────────
	KSheinDisplayAttributeFieldSelection      = "shein.display_attribute.field_selection"
	KSheinDisplayAttributeValueMapping        = "shein.display_attribute.value_mapping"
	KSheinDisplayAttributeValueMappingBatch   = "shein.display_attribute.value_mapping_batch"
	KSheinDisplayAttributeMissingText         = "shein.display_attribute.missing_text"
	KSheinDisplayAttributeMissingValue        = "shein.display_attribute.missing_value"
	KSheinDisplayAttributeFieldSelectionBatch = "shein.display_attribute.field_selection_batch"
	KSheinDisplayAttributeBatchInference      = "shein.display_attribute.batch_inference"
	KSheinDisplayAttributeRequiredRepair      = "shein.display_attribute.required_repair"

	// ── shein/sale_attribute.yaml ──────────────────────────────────────────
	KSheinSaleAttributeMapping               = "shein.sale_attribute.mapping"
	KSheinSaleAttributeSourceDimension       = "shein.sale_attribute.source_dimension"
	KSheinSaleAttributeValueBatchMapping     = "shein.sale_attribute.value_batch_mapping"
	KSheinSaleAttributePromptValueExtraction = "shein.sale_attribute.prompt_value_extraction"

	// ── shein/translation.yaml ─────────────────────────────────────────────
	KSheinTranslationBatchOptimizeSystem = "shein.translation.batch_optimize_system"

	// ── temu/attribute_mapping.yaml ────────────────────────────────────────
	KTemuAttributeMappingSystem = "temu.attribute_mapping.system"

	// ── temu/content_rewriter.yaml ─────────────────────────────────────────
	KTemuContentRewriterSystem = "temu.content_rewriter.system"

	// ── temu/sku_variant_mapping.yaml ──────────────────────────────────────
	KTemuSkuVariantMappingSystem = "temu.sku_variant_mapping.system"

	// ── temu/vision_detector.yaml ──────────────────────────────────────────
	KTemuVisionDetectorDetect = "temu.vision_detector.detect"

	// ── productenrich/llm_scorer.yaml ──────────────────────────────────────
	KProductEnrichLlmScorerTextScoring  = "productenrich.llm_scorer.text_scoring"
	KProductEnrichLlmScorerImageScoring = "productenrich.llm_scorer.image_scoring"

	// ── productenrich/understanding.yaml ───────────────────────────────────
	KProductEnrichUnderstandingAnalyzeImage   = "productenrich.understanding.analyze_image"
	KProductEnrichUnderstandingExtractText    = "productenrich.understanding.extract_text"
	KProductEnrichUnderstandingFuseMultimodal = "productenrich.understanding.fuse_multimodal"

	// ── productenrich/generation.yaml ──────────────────────────────────────
	KProductEnrichGenerationProductJSON       = "productenrich.generation.product_json"
	KProductEnrichGenerationSpecs             = "productenrich.generation.specs"
	KProductEnrichGenerationVariants          = "productenrich.generation.variants"
	KProductEnrichGenerationExtractDimensions = "productenrich.generation.extract_dimensions"
	KProductEnrichGenerationExtractWeight     = "productenrich.generation.extract_weight"

	// ── productimage/generation.yaml ───────────────────────────────────────
	KProductImageSubjectExtract                     = "productimage.subject.extract"
	KProductImageWhiteBackgroundDefault             = "productimage.white_background.default"
	KProductImageSceneDefault                       = "productimage.scene.default"
	KProductImageSceneShoes                         = "productimage.scene.shoes"
	KProductImageSceneJewelry                       = "productimage.scene.jewelry"
	KProductImageSceneBags                          = "productimage.scene.bags"
	KProductImageReviewDefault                      = "productimage.review.default"
	KProductImageStudioGenerationPodDesign          = "productimage.studio_generation.pod_design"
	KProductImageStudioGenerationAmazonProductImage = "productimage.studio_generation.amazon_product_image"
)
