package shein

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/catalog/canonical"
	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type saleAttributeResolver struct {
	api         AttributeAPI
	llm         TextGenerator
	deniedStore ResolutionCacheStore
}

// saleAttributeTemplateContext is the normalized template bundle used after the
// first SHEIN sale-attribute template group has been loaded.
type saleAttributeTemplateContext struct {
	attributes []sheinattribute.AttributeInfo
	index      *templateIndex
}

// saleAttributeCandidateState carries the mutable selection state while the
// resolver moves through template-first matching, LLM repair, and fallback.
type saleAttributeCandidateState struct {
	candidates                 []saleAttributeCandidate
	primaryCandidate           *saleAttributeCandidate
	secondaryCandidate         *saleAttributeCandidate
	blockUnsafePrimaryFallback bool
}

func NewSaleAttributeResolver(api AttributeAPI, llm TextGenerator) SaleAttributeResolver {
	return &saleAttributeResolver{api: api, llm: llm}
}

func NewSaleAttributeResolverWithDeniedStore(api AttributeAPI, llm TextGenerator, store ResolutionCacheStore) SaleAttributeResolver {
	return &saleAttributeResolver{api: api, llm: llm, deniedStore: store}
}

// Resolve keeps the orchestration layer short:
// 1) initialize request state
// 2) load template context
// 3) resolve candidate state
// 4) materialize the public resolution result
func (r *saleAttributeResolver) Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *SaleAttributeResolution {
	log := sheinLogger("shein/sale_attribute")
	resolution, sourceDimensions := r.initializeResolution(canonical, pkg)
	if early := r.validateResolutionInputs(resolution); early != nil {
		return early
	}
	templateCtx, early := r.loadTemplateContext(log, resolution)
	if early != nil {
		return early
	}
	if len(sourceDimensions) == 0 {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "缺少源销售属性维度，当前无法构建 SHEIN SKC/SKU 映射")
		return resolution
	}

	state := r.resolveCandidateState(resolution, sourceDimensions, templateCtx)
	r.applyResolvedCandidates(req, resolution, templateCtx.index, pkg, state.candidates, state.primaryCandidate, state.secondaryCandidate)
	if !resolution.RecommendCategoryReview {
		if recommend, reason := buildCategoryFamilyConflictSummary(canonical, pkg); recommend {
			resolution.RecommendCategoryReview = true
			resolution.CategoryReviewReason = reason
			resolution.ReviewNotes = append(resolution.ReviewNotes, buildCategoryFamilyConflictReviewNotes(canonical, pkg)...)
		}
	}
	log.WithFields(logrus.Fields{
		"category_id":            resolution.CategoryID,
		"status":                 resolution.Status,
		"primary_attribute_id":   resolution.PrimaryAttributeID,
		"secondary_attribute_id": resolution.SecondaryAttributeID,
		"candidate_count":        len(resolution.Candidates),
	}).Info("resolved SHEIN sale attributes")
	return resolution
}

// resolveCandidateState contains the branch-heavy middle section so Resolve can
// stay focused on stage ordering instead of candidate mutation details.
func (r *saleAttributeResolver) resolveCandidateState(
	resolution *SaleAttributeResolution,
	sourceDimensions []SourceVariantDimension,
	templateCtx *saleAttributeTemplateContext,
) saleAttributeCandidateState {
	state := saleAttributeCandidateState{
		candidates: buildSaleAttributeCandidates(sourceDimensions, templateCtx.attributes),
	}
	state.primaryCandidate, state.secondaryCandidate = resolvePrimarySecondaryCandidates(state.candidates, templateCtx.attributes)
	r.applyLLMSelectionRepair(resolution, sourceDimensions, templateCtx, &state)
	r.applyPrimaryTemplateRepair(resolution, sourceDimensions, templateCtx, &state)
	r.enforcePrimaryTemplateConstraints(resolution, templateCtx, &state)
	r.applySourceDimensionFallback(resolution, sourceDimensions, &state)
	if state.primaryCandidate == nil && len(state.candidates) > 0 {
		resolution.ReviewNotes = append(resolution.ReviewNotes, buildNoFitCandidateReviewNotes(state.candidates)...)
	}
	return state
}

// applyLLMSelectionRepair asks the LLM for an explicit mapping only when the
// template-first result is weak or conflicts with the primary template rule.
func (r *saleAttributeResolver) applyLLMSelectionRepair(
	resolution *SaleAttributeResolution,
	sourceDimensions []SourceVariantDimension,
	templateCtx *saleAttributeTemplateContext,
	state *saleAttributeCandidateState,
) {
	if !shouldSelectSaleAttributeMappingWithLLM(state.primaryCandidate, sourceDimensions, templateCtx.attributes) {
		return
	}
	mappingDimensions := saleAttributeMappingSourceDimensions(sourceDimensions)
	selection, err := selectSaleAttributeMappingWithLLM(r.llm, mappingDimensions, templateCtx.attributes)
	if err != nil || selection == nil {
		return
	}
	llmPrimary, llmSecondary, augmentedCandidates := matchSelectedCandidates(state.candidates, selection, sourceDimensions, templateCtx.attributes)
	if hasMarkedPrimarySaleTemplate(templateCtx.attributes) && (llmPrimary == nil || selectedCandidateMissesPrimarySaleTemplate(llmPrimary, templateCtx.attributes)) {
		r.applyLLMPrimaryRetry(resolution, sourceDimensions, templateCtx, state, llmPrimary, augmentedCandidates, mappingDimensions)
		return
	}
	if selectedCandidateMissesPrimarySaleTemplate(llmPrimary, templateCtx.attributes) {
		state.candidates = augmentedCandidates
		resolution.ReviewNotes = append(resolution.ReviewNotes, buildPrimarySaleTemplateMismatchNote(llmPrimary, templateCtx.attributes))
		state.blockUnsafePrimaryFallback = true
		state.primaryCandidate = nil
		state.secondaryCandidate = nil
		return
	}
	if shouldUseLLMSaleAttributeMapping(state.primaryCandidate, llmPrimary, templateCtx.attributes) {
		state.candidates = augmentedCandidates
		state.primaryCandidate, state.secondaryCandidate = llmPrimary, llmSecondary
		resolution.Source = "llm_sale_attribute_mapping"
		resolution.ReviewNotes = append(resolution.ReviewNotes, selection.Reasons...)
	}
}

// applyLLMPrimaryRetry is the stricter repair path used when the first LLM
// answer misses the primary template marked by AttributeLabel == 1.
func (r *saleAttributeResolver) applyLLMPrimaryRetry(
	resolution *SaleAttributeResolution,
	sourceDimensions []SourceVariantDimension,
	templateCtx *saleAttributeTemplateContext,
	state *saleAttributeCandidateState,
	llmPrimary *saleAttributeCandidate,
	augmentedCandidates []saleAttributeCandidate,
	mappingDimensions []SourceVariantDimension,
) {
	feedback := buildPrimarySaleTemplateMismatchNote(llmPrimary, templateCtx.attributes) + "。请重新选择 primary_source_dimension,但 primary_attribute_id 必须等于 SHEIN primary_label=true 的首个模板 attribute_id。"
	retrySelection, retryErr := selectSaleAttributeMappingWithLLMFeedback(r.llm, mappingDimensions, templateCtx.attributes, feedback)
	if retryErr != nil || retrySelection == nil {
		state.candidates = augmentedCandidates
		resolution.ReviewNotes = append(resolution.ReviewNotes, buildPrimarySaleTemplateMismatchNote(llmPrimary, templateCtx.attributes))
		state.blockUnsafePrimaryFallback = true
		state.primaryCandidate = nil
		state.secondaryCandidate = nil
		return
	}
	retryPrimary, retrySecondary, retryCandidates := matchSelectedCandidates(augmentedCandidates, retrySelection, sourceDimensions, templateCtx.attributes)
	if retryPrimary != nil &&
		!selectedCandidateMissesPrimarySaleTemplate(retryPrimary, templateCtx.attributes) &&
		shouldUseLLMSaleAttributeMapping(state.primaryCandidate, retryPrimary, templateCtx.attributes) {
		state.candidates = retryCandidates
		state.primaryCandidate, state.secondaryCandidate = retryPrimary, retrySecondary
		resolution.Source = "llm_sale_attribute_mapping"
		resolution.ReviewNotes = append(resolution.ReviewNotes, retrySelection.Reasons...)
		return
	}
	state.candidates = retryCandidates
	resolution.ReviewNotes = append(resolution.ReviewNotes, buildPrimarySaleTemplateMismatchNote(retryPrimary, templateCtx.attributes))
	state.blockUnsafePrimaryFallback = true
	state.primaryCandidate = nil
	state.secondaryCandidate = nil
}

// applyPrimaryTemplateRepair rebuilds a primary candidate directly from the
// chosen source dimension when source selection was usable but mapping was not.
func (r *saleAttributeResolver) applyPrimaryTemplateRepair(
	resolution *SaleAttributeResolution,
	sourceDimensions []SourceVariantDimension,
	templateCtx *saleAttributeTemplateContext,
	state *saleAttributeCandidateState,
) {
	if resolution.Source != "llm_source_dimensions" ||
		!hasMarkedPrimarySaleTemplate(templateCtx.attributes) ||
		(state.primaryCandidate != nil && !selectedCandidateMissesPrimarySaleTemplate(state.primaryCandidate, templateCtx.attributes)) {
		return
	}
	repairedPrimary, ok := buildPrimaryLabelCandidateFromSourceSelection(resolution.PrimarySourceDimension, sourceDimensions, templateCtx.attributes)
	if !ok {
		return
	}
	state.candidates = append(state.candidates, repairedPrimary)
	if orderedPrimary, orderedSecondary := selectTemplateOrderedCandidates(state.candidates, templateCtx.attributes); orderedPrimary != nil {
		state.primaryCandidate = orderedPrimary
		state.secondaryCandidate = orderedSecondary
		resolution.Source = "llm_sale_attribute_mapping"
	}
}

// enforcePrimaryTemplateConstraints prevents later fallback rules from silently
// replacing the required primary template with a scored alternative candidate.
func (r *saleAttributeResolver) enforcePrimaryTemplateConstraints(
	resolution *SaleAttributeResolution,
	templateCtx *saleAttributeTemplateContext,
	state *saleAttributeCandidateState,
) {
	if hasMarkedPrimarySaleTemplate(templateCtx.attributes) && selectedCandidateMissesPrimarySaleTemplate(state.primaryCandidate, templateCtx.attributes) {
		resolution.ReviewNotes = append(resolution.ReviewNotes, buildPrimarySaleTemplateMismatchNote(state.primaryCandidate, templateCtx.attributes))
		state.blockUnsafePrimaryFallback = true
		state.primaryCandidate = nil
		state.secondaryCandidate = nil
	}
	if hasMarkedPrimarySaleTemplate(templateCtx.attributes) && state.primaryCandidate == nil && len(state.candidates) > 0 {
		resolution.ReviewNotes = append(resolution.ReviewNotes, buildPrimarySaleTemplateMismatchNote(nil, templateCtx.attributes))
		state.blockUnsafePrimaryFallback = true
	}
}

// applySourceDimensionFallback is the final non-LLM fallback path after all
// primary-template guards and repair attempts have run.
func (r *saleAttributeResolver) applySourceDimensionFallback(
	resolution *SaleAttributeResolution,
	sourceDimensions []SourceVariantDimension,
	state *saleAttributeCandidateState,
) {
	if shouldBlockPromptDerivedAIStylePrimary(state.primaryCandidate, sourceDimensions) && resolution.Source != "llm_sale_attribute_mapping" {
		state.blockUnsafePrimaryFallback = true
		state.primaryCandidate = nil
		state.secondaryCandidate = nil
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN Style Type 不能使用用户设计提示词作为销售属性来源，需使用稳定销售维度重新映射")
	}
	if state.primaryCandidate == nil && !state.blockUnsafePrimaryFallback {
		state.primaryCandidate, state.secondaryCandidate = selectCandidatesBySourceDimensions(
			state.candidates,
			resolution.PrimarySourceDimension,
			resolution.SecondarySourceDimension,
		)
	}
}

// initializeResolution extracts source dimensions and records the initial
// source-dimension selection result before any template lookup begins.
func (r *saleAttributeResolver) initializeResolution(canonical *canonical.Product, pkg *Package) (*SaleAttributeResolution, []SourceVariantDimension) {
	resolution := &SaleAttributeResolution{Status: "unresolved", Source: "fallback", CategoryID: categoryID(pkg)}
	sourceDimensions := saleAttributeSourceDimensions(buildSourceVariantDimensions(canonical, common.BuildVariants(canonical)))
	resolution.SourceDimensions = append([]SourceVariantDimension(nil), sourceDimensions...)
	if sourceSelection := selectSourceDimensions(sourceDimensions, r.llm); sourceSelection != nil {
		resolution.PrimarySourceDimension = sourceSelection.PrimarySourceDimension
		resolution.SecondarySourceDimension = sourceSelection.SecondarySourceDimension
		resolution.SelectionSummary = append(resolution.SelectionSummary, sourceSelection.Reasons...)
		if r.llm != nil {
			resolution.Source = "llm_source_dimensions"
		} else {
			resolution.Source = "source_dimensions_fallback"
		}
	}
	return resolution, sourceDimensions
}

// validateResolutionInputs handles hard-stop checks that do not require
// template loading or candidate construction.
func (r *saleAttributeResolver) validateResolutionInputs(resolution *SaleAttributeResolution) *SaleAttributeResolution {
	if resolution.CategoryID == 0 {
		resolution.ReviewNotes = append(resolution.ReviewNotes, "缺少 SHEIN category_id，无法解析销售属性")
		return resolution
	}
	if r.api == nil {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "缺少 SHEIN AttributeAPI，当前无法加载销售属性模板")
		return resolution
	}
	return nil
}

// loadTemplateContext loads the first template group and normalizes it into the
// structure used by downstream sale-attribute selection logic.
func (r *saleAttributeResolver) loadTemplateContext(log *logrus.Entry, resolution *SaleAttributeResolution) (*saleAttributeTemplateContext, *SaleAttributeResolution) {
	log.WithFields(logrus.Fields{
		"category_id":         resolution.CategoryID,
		"source_dimensions":   len(resolution.SourceDimensions),
		"primary_dimension":   resolution.PrimarySourceDimension,
		"secondary_dimension": resolution.SecondarySourceDimension,
	}).Info("loading SHEIN sale attribute templates")
	templates, err := r.api.GetAttributeTemplates(resolution.CategoryID)
	if err != nil {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 销售属性模板加载失败: "+err.Error())
		log.WithError(err).WithField("category_id", resolution.CategoryID).Warn("failed to load SHEIN sale attribute templates")
		return nil, resolution
	}
	if templates == nil || len(templates.Data) == 0 {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 销售属性模板为空")
		log.WithField("category_id", resolution.CategoryID).Warn("SHEIN sale attribute templates are empty")
		return nil, resolution
	}
	template := templates.Data[0]
	attributes := orderSaleScopeAttributes(filterSaleScopeAttributes(template.AttributeInfos), template.AttributeID)
	resolution.TemplateOptions = buildSaleAttributeTemplateOptions(attributes)
	index := newTemplateIndex(attributes)
	log.WithFields(logrus.Fields{
		"category_id":          resolution.CategoryID,
		"template_groups":      len(templates.Data),
		"sale_attribute_count": len(attributes),
	}).Info("loaded SHEIN sale attribute templates")
	if len(index.attributes) == 0 {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "当前类目未识别到可用的销售属性模板")
		return nil, resolution
	}
	return &saleAttributeTemplateContext{attributes: attributes, index: index}, nil
}

// applyResolvedCandidates writes the chosen candidates back into the public
// resolution payload and computes the final resolved/partial status.
func (r *saleAttributeResolver) applyResolvedCandidates(
	req *BuildRequest,
	resolution *SaleAttributeResolution,
	index *templateIndex,
	pkg *Package,
	candidates []saleAttributeCandidate,
	primaryCandidate, secondaryCandidate *saleAttributeCandidate,
) {
	if primaryCandidate != nil {
		resolution.PrimarySourceDimension = primaryCandidate.SourceName
	}
	if secondaryCandidate != nil {
		resolution.SecondarySourceDimension = secondaryCandidate.SourceName
	} else if primaryCandidate != nil && normalizeText(resolution.SecondarySourceDimension) == normalizeText(primaryCandidate.SourceName) {
		resolution.SecondarySourceDimension = ""
	}

	resolution.Candidates = buildSaleAttributeCandidateInfos(candidates, primaryCandidate, secondaryCandidate)
	spuName := ""
	if pkg != nil {
		spuName = firstNonEmpty(pkg.SpuName, pkg.ProductNameEn)
	}
	applySelectedCandidate(index, primaryCandidate, "skc", r.api, resolution.CategoryID, spuName, sheinStoreID(req), r.deniedStore, r.llm, resolution)
	applySelectedCandidate(index, secondaryCandidate, "sku", r.api, resolution.CategoryID, spuName, sheinStoreID(req), r.deniedStore, r.llm, resolution)
	resolution.SelectionSummary = append(resolution.SelectionSummary, buildSelectionSummary(primaryCandidate, secondaryCandidate)...)
	if primaryCandidate != nil && resolution.Source != "llm_sale_attribute_mapping" {
		resolution.Source = "sale_attribute_templates"
	}
	primaryCoverage := resolvedSaleAttributeCoverage(primaryCandidate, resolution.skcValueAssignments)
	secondaryCoverage := resolvedSaleAttributeCoverage(secondaryCandidate, resolution.skuValueAssignments)
	switch {
	case resolution.PrimaryAttributeID > 0 &&
		primaryCoverage.complete &&
		((resolution.SecondaryAttributeID > 0 && secondaryCoverage.complete) || secondaryCandidate == nil):
		resolution.Status = "resolved"
	case resolution.PrimaryAttributeID > 0 || resolution.SecondaryAttributeID > 0:
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 销售属性已部分解析，仍建议人工确认变体规格")
	default:
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "未命中可用的 SHEIN 销售属性映射")
	}
}
