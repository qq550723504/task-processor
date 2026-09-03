package listingkit

import (
	"strconv"
	"strings"

	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	"task-processor/internal/product/catalog/canonical"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sdspod "task-processor/internal/sds/adapter/product_source"
)

func (s *service) refreshSheinDerivedState(task *Task, req *ApplyRevisionRequest) error {
	if s == nil || task == nil || task.Result == nil || task.Result.Shein == nil || req == nil || req.Shein == nil {
		return nil
	}
	if !shouldRefreshSheinDerivedState(req.Shein) {
		return nil
	}
	if task.Result.CanonicalProduct == nil {
		return nil
	}
	task.Result.Shein = sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	if task.Request != nil && task.Request.Options != nil {
		sdsOptions := task.Request.Options.SDS
		if task.Result.SDSDesignResult != nil {
			applySDSSyncMetadataToCanonical(
				task.Result.CanonicalProduct,
				task.Result.SDSDesignResult,
				sdsOptions,
			)
		} else {
			sdspod.ApplyCanonical(task.Result.CanonicalProduct, sdspod.CanonicalMetadata{
				StyleName: sdsStyleName(sdsOptions),
			})
		}
	}

	buildReq := buildSheinPublishRequestForTask(task, task.Request)

	// 检查是否需要重新进行类目解析(用于手动刷新且没有有效 category_id 的场景)
	needReResolveCategory := false
	if req.Shein.CategoryResolution != nil {
		source := strings.TrimSpace(*req.Shein.CategoryResolution.Source)
		categoryID := req.Shein.CategoryResolution.CategoryID
		if source == "manual_refresh" && (categoryID == nil || *categoryID <= 0) {
			needReResolveCategory = true
		}
	}

	// 如果需要重新解析类目,先调用类目解析器
	categoryResolver := resolveSheinCategoryResolver(s)
	attributeResolver := resolveSheinAttributeResolver(s)
	saleAttributeResolver := resolveSheinSaleAttributeResolver(s)
	sizeHeaderResolver := resolveSheinSizeHeaderResolver(s)
	pricingPolicy := resolveSheinPricingPolicy(s)
	if needReResolveCategory {
		if err := validateSheinResolver("category", categoryResolver != nil); err != nil {
			return err
		}
		task.Result.Shein.CategoryResolution = categoryResolver.Resolve(buildReq, task.Result.CanonicalProduct, task.Result.Shein)
		if err := validateSheinResolution("category", task.Result.Shein.CategoryResolution != nil); err != nil {
			return err
		}
		sheinpub.ApplyCategoryResolution(task.Result.Shein, task.Result.Shein.CategoryResolution)
	}

	// 设置目标类目提示(优先使用解析后的 category_id)
	if task.Result.Shein.CategoryID > 0 {
		buildReq.TargetCategoryHint = strconv.Itoa(task.Result.Shein.CategoryID)
	}
	if req.Shein.RegenerateAttributes {
		if err := validateSheinResolver("attribute", attributeResolver != nil); err != nil {
			return err
		}
		pkg := prepareSheinAttributeDerivedState(task)
		resolution := resolveFreshSheinAttributeResolution(attributeResolver, buildReq, task.Result.CanonicalProduct, pkg)
		if err := validateSheinResolution("attribute", resolution != nil); err != nil {
			return err
		}
		if cache, ok := attributeResolver.(sheinpub.AttributeResolutionCache); ok {
			if err := cache.ClearAttributeResolution(buildReq, task.Result.CanonicalProduct, pkg); err != nil {
				return err
			}
		}
		return s.applySheinAttributeDerivedState(task, buildReq, pkg, resolution)
	}
	if err := validateSheinResolver("attribute", attributeResolver != nil); err != nil {
		return err
	}
	if err := validateSheinResolver("sale-attribute", saleAttributeResolver != nil); err != nil {
		return err
	}
	sheinpub.RefreshDerivedState(
		buildReq,
		task.Result.CanonicalProduct,
		task.Result.Shein,
		categoryResolver,
		attributeResolver,
		saleAttributeResolver,
		sizeHeaderResolver,
		pricingPolicy,
	)
	if err := validateSheinResolution("attribute", task.Result.Shein.AttributeResolution != nil); err != nil {
		return err
	}
	if err := validateSheinResolution("sale-attribute", task.Result.Shein.SaleAttributeResolution != nil); err != nil {
		return err
	}
	cookieNote := strings.TrimSpace(s.resolveSheinCookieAvailabilityNote(buildReq.Context, task))
	if cookieNote == "" {
		sheinworkspace.StripCookieUnavailableReviewNotes(task.Result.Shein)
	}
	applySheinSaleAttributeReviewOverride(task.Result.Shein, req.Shein.SaleAttributeResolution)
	normalizeSheinCategoryRefreshSaleAttributeState(task.Result.Shein)
	sheinpub.NormalizeListingCopy(buildReq.Context, task.Result.Shein, task.Result.CanonicalProduct, buildReq.Language)
	syncSheinDraftFromPackage(task.Result.Shein)
	preview := sheinpub.BuildPreviewProduct(task.Result.Shein)
	sheinpub.SetPreviewPayload(task.Result.Shein, preview)
	if cookieNote != "" {
		refreshSheinReviewState(task.Result.Shein, cookieNote)
		return nil
	}
	refreshSheinReviewState(task.Result.Shein)
	return nil
}

func (s *service) refreshSheinAttributeDerivedState(task *Task, buildReq *sheinpub.BuildRequest) error {
	if s == nil || task == nil || task.Result == nil || task.Result.Shein == nil || task.Result.CanonicalProduct == nil {
		return nil
	}
	attributeResolver := resolveSheinAttributeResolver(s)
	if err := validateSheinResolver("attribute", attributeResolver != nil); err != nil {
		return err
	}
	pkg := prepareSheinAttributeDerivedState(task)
	resolution := attributeResolver.Resolve(buildReq, task.Result.CanonicalProduct, pkg)
	if err := validateSheinResolution("attribute", resolution != nil); err != nil {
		return err
	}
	return s.applySheinAttributeDerivedState(task, buildReq, pkg, resolution)
}

func prepareSheinAttributeDerivedState(task *Task) *sheinpub.Package {
	pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	pkg.ProductAttributes = common.BuildAttributes(task.Result.CanonicalProduct.Attributes)
	sheinpub.EnsureDraftPayload(pkg)
	return pkg
}

func resolveFreshSheinAttributeResolution(resolver sheinpub.AttributeResolver, buildReq *sheinpub.BuildRequest, product *canonical.Product, pkg *sheinpub.Package) *sheinpub.AttributeResolution {
	if freshResolver, ok := resolver.(sheinpub.FreshAttributeResolver); ok {
		return freshResolver.ResolveFreshAttributeResolution(buildReq, product, pkg)
	}
	return resolver.Resolve(buildReq, product, pkg)
}

func (s *service) applySheinAttributeDerivedState(task *Task, buildReq *sheinpub.BuildRequest, pkg *sheinpub.Package, resolution *sheinpub.AttributeResolution) error {
	pkg.AttributeResolution = resolution
	sheinpub.ApplyAttributeResolution(pkg, pkg.AttributeResolution)
	cookieNote := strings.TrimSpace(s.resolveSheinCookieAvailabilityNote(buildReq.Context, task))
	if cookieNote == "" {
		sheinworkspace.StripCookieUnavailableReviewNotes(pkg)
	}
	sheinpub.NormalizeListingCopy(buildReq.Context, pkg, task.Result.CanonicalProduct, buildReq.Language)
	syncSheinDraftFromPackage(pkg)
	preview := sheinpub.BuildPreviewProduct(pkg)
	sheinpub.SetPreviewPayload(pkg, preview)
	if cookieNote != "" {
		refreshSheinReviewState(pkg, cookieNote)
		return nil
	}
	refreshSheinReviewState(pkg)
	return nil
}

func applySheinSaleAttributeReviewOverride(pkg *sheinpub.Package, patch *SheinSaleAttributeResolutionPatch) {
	if pkg == nil || patch == nil ||
		(patch.RecommendCategoryReview == nil && patch.CategoryReviewReason == nil) {
		return
	}
	if pkg.SaleAttributeResolution == nil {
		pkg.SaleAttributeResolution = &sheinpub.SaleAttributeResolution{}
	}
	confirmedCategoryReview := patch.RecommendCategoryReview != nil && !*patch.RecommendCategoryReview
	if patch.RecommendCategoryReview != nil {
		pkg.SaleAttributeResolution.RecommendCategoryReview = *patch.RecommendCategoryReview
		if !*patch.RecommendCategoryReview && pkg.CategoryResolution != nil {
			pkg.CategoryResolution.SuggestedCategory = nil
		}
	}
	if patch.CategoryReviewReason != nil {
		if confirmedCategoryReview {
			pkg.SaleAttributeResolution.CategoryReviewReason = ""
		} else {
			pkg.SaleAttributeResolution.CategoryReviewReason = *patch.CategoryReviewReason
		}
	} else if confirmedCategoryReview {
		pkg.SaleAttributeResolution.CategoryReviewReason = ""
	}
}

func shouldRefreshSheinDerivedState(req *SheinRevisionInput) bool {
	if req == nil {
		return false
	}
	if req.RegenerateAttributes {
		return true
	}
	if req.RegenerateSaleAttributes {
		return true
	}
	if req.CategoryResolution == nil {
		return false
	}
	if req.AttributeResolution != nil {
		return false
	}
	if req.RequestDraft != nil || len(req.SKCPatches) > 0 || req.ResolvedAttributes != nil {
		return false
	}
	return true
}

func normalizeSheinCategoryRefreshSaleAttributeState(pkg *sheinpub.Package) {
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		return
	}
	normalizeSheinSaleAttributeState(pkg)
	if !containsReviewNote(pkg.SaleAttributeResolution.ReviewNotes, "类目变更后已重新生成销售属性，但当前仍缺少真实 sale attribute value 映射，请重新确认规格。") {
		pkg.SaleAttributeResolution.ReviewNotes = uniqueStrings(append(
			[]string(nil),
			append(
				pkg.SaleAttributeResolution.ReviewNotes,
				"类目变更后已重新生成销售属性，但当前仍缺少真实 sale attribute value 映射，请重新确认规格。",
			)...,
		))
	}
}

func containsReviewNote(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
