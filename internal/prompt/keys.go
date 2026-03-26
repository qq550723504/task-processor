package prompt

// Prompt key 常量，与 prompts/ 目录下 YAML 文件的展平路径对应。
// 命名规则：K + 大驼峰路径，例如 shein.category_selector.select_category_system → KSheinCategorySelectorSelectCategorySystem
const (
	// ── shein/attribute_selector.yaml ──────────────────────────────────────
	KSheinAttributeSelectorSystem = "shein.attribute_selector.system"

	// ── shein/category_selector.yaml ───────────────────────────────────────
	KSheinCategorySelectorSelectCategorySystem  = "shein.category_selector.select_category_system"
	KSheinCategorySelectorSelectCategoryUser    = "shein.category_selector.select_category_user"
	KSheinCategorySelectorExtractCoreItemSystem = "shein.category_selector.extract_core_item_system"

	// ── shein/content_optimizer.yaml ───────────────────────────────────────
	KSheinContentOptimizerOptimizeTitleDescriptionSystem = "shein.content_optimizer.optimize_title_description_system"
	KSheinContentOptimizerOptimizeTitleDescriptionUser   = "shein.content_optimizer.optimize_title_description_user"

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
)
