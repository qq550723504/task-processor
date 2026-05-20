package shein

import (
	"context"
	"strconv"
	"strings"

	"task-processor/internal/catalog/canonical"
	sheinapi "task-processor/internal/shein/api"
	sheincategory "task-processor/internal/shein/api/category"
)

type categoryResolver struct {
	api              CategoryAPI
	suggestFallback  categorySuggestFallback
	treeFallback     categoryTreeFallback
	semanticVerifier categorySemanticVerifier
}

func NewCategoryResolver(api CategoryAPI) CategoryResolver { return &categoryResolver{api: api} }

func NewCategoryResolverWithFallbacks(api CategoryAPI, suggestFallback categorySuggestFallback, treeFallback categoryTreeFallback) CategoryResolver {
	return NewCategoryResolverWithSemanticVerifier(api, suggestFallback, treeFallback, nil)
}

func NewCategoryResolverWithSemanticVerifier(api CategoryAPI, suggestFallback categorySuggestFallback, treeFallback categoryTreeFallback, semanticVerifier categorySemanticVerifier) CategoryResolver {
	return &categoryResolver{
		api:              api,
		suggestFallback:  suggestFallback,
		treeFallback:     treeFallback,
		semanticVerifier: semanticVerifier,
	}
}

func NewCategoryResolverWithTreeFallback(api CategoryAPI, treeFallback categoryTreeFallback) CategoryResolver {
	return NewCategoryResolverWithFallbacks(api, nil, treeFallback)
}

func (r *categoryResolver) Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *CategoryResolution {
	ctx := context.Background()
	if req != nil && req.Context != nil {
		ctx = req.Context
	}
	suggestQuery := buildCategorySuggestionQuery(req, canonical, pkg)
	treeQuery := buildCategoryQuery(req, canonical, pkg)
	resolution := &CategoryResolution{
		Status:      "unresolved",
		Source:      "fallback",
		QueryText:   firstNonEmptyCategoryQuery(suggestQuery, treeQuery),
		MatchedPath: append([]string(nil), resolveCategoryPath(canonical, pkg)...),
	}
	if hintedID := parseFirstPositiveInt(req.TargetCategoryHint); hintedID > 0 {
		resolution.Source = "target_category_hint"
		resolution.CategoryID = hintedID
		if r.api == nil {
			resolution.Status = "partial"
			resolution.ReviewNotes = append(resolution.ReviewNotes, "缺少 SHEIN CategoryAPI，当前只能保留 hint 中的 category_id")
			return resolution
		}
		if info, err := r.api.GetCategory(hintedID); err == nil && info != nil {
			hydrated := hydrateCategoryResolution(info, resolution.Source, resolution.QueryText)
			hydrated.SemanticValidation = r.semanticValidation(ctx, canonical, pkg, hydrated.MatchedPath)
			if semanticRejectsCategory(hydrated.SemanticValidation) {
				hydrated.ReviewNotes = append(hydrated.ReviewNotes, buildSemanticCategoryReviewNote(hydrated.SemanticValidation))
			}
			return hydrated
		}
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "target_category_hint 已解析，但未能拉取完整类目层级")
		return resolution
	}
	if r.api != nil && r.suggestFallback != nil && strings.TrimSpace(suggestQuery) != "" {
		selectedID, suggestErr := r.suggestFallback.SelectCategoryID(ctx, buildCategorySuggestInput(req, canonical, pkg), r.api)
		if suggestErr == nil && selectedID > 0 {
			if info, infoErr := r.api.GetCategory(selectedID); infoErr == nil && info != nil {
				hydrated := hydrateCategoryResolution(info, "suggest_category_by_text", suggestQuery)
				hydrated.SemanticValidation = r.semanticValidation(ctx, canonical, pkg, hydrated.MatchedPath)
				if r.acceptsAutomatedCategory(hydrated.SemanticValidation) {
					return hydrated
				}
				resolution.Status = "partial"
				resolution.Source = "suggest_category_by_text"
				resolution.QueryText = suggestQuery
				resolution.SemanticValidation = hydrated.SemanticValidation
				resolution.SuggestedCategory = categorySuggestionFromResolution(hydrated, semanticCategoryReviewNote(hydrated.SemanticValidation))
				resolution.ReviewNotes = append(resolution.ReviewNotes, semanticCategoryReviewNote(hydrated.SemanticValidation))
			} else {
				if tree, treeErr := r.api.GetCategoryTree(); treeErr == nil {
					if hydrated := hydrateCategoryResolutionFromTree(tree, selectedID, "suggest_category_by_text", suggestQuery); hydrated != nil {
						hydrated.SemanticValidation = r.semanticValidation(ctx, canonical, pkg, hydrated.MatchedPath)
						if r.acceptsAutomatedCategory(hydrated.SemanticValidation) {
							return hydrated
						}
						hydrated.Status = "partial"
						hydrated.SuggestedCategory = categorySuggestionFromResolution(hydrated, semanticCategoryReviewNote(hydrated.SemanticValidation))
						hydrated.CategoryID = 0
						hydrated.CategoryIDList = nil
						hydrated.ProductTypeID = 0
						hydrated.TopCategoryID = 0
						hydrated.ReviewNotes = append(hydrated.ReviewNotes, semanticCategoryReviewNote(hydrated.SemanticValidation))
						return hydrated
					}
				}
				resolution.Source = "suggest_category_by_text"
				resolution.Status = "partial"
				resolution.QueryText = suggestQuery
				resolution.CategoryID = selectedID
				resolution.ReviewNotes = append(resolution.ReviewNotes, "SuggestCategoryByText 已命中，但未能拉取完整类目详情")
				return resolution
			}
		} else if suggestErr != nil {
			resolution.Status = "partial"
			resolution.QueryText = suggestQuery
			resolution.ReviewNotes = append(resolution.ReviewNotes, formatCategoryResolutionAPIError(suggestErr))
		}
	}
	if r.api != nil && r.treeFallback != nil && strings.TrimSpace(treeQuery) != "" {
		tree, treeErr := r.api.GetCategoryTree()
		if treeErr != nil {
			resolution.Status = "partial"
			resolution.ReviewNotes = append(resolution.ReviewNotes, formatCategoryTreeResolutionAPIError(treeErr))
			return resolution
		}
		selectedID, selectErr := r.treeFallback.SelectCategoryID(ctx, treeQuery, tree)
		if selectErr != nil {
			resolution.Status = "partial"
			resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 类目树候选重选失败: "+strings.TrimSpace(selectErr.Error()))
			return resolution
		}
		if selectedID > 0 {
			if info, infoErr := r.api.GetCategory(selectedID); infoErr == nil && info != nil {
				hydrated := hydrateCategoryResolution(info, "ai_category_tree", treeQuery)
				hydrated.SemanticValidation = r.semanticValidation(ctx, canonical, pkg, hydrated.MatchedPath)
				if r.acceptsAutomatedCategory(hydrated.SemanticValidation) {
					return hydrated
				}
				resolution.Status = "partial"
				resolution.QueryText = treeQuery
				resolution.SemanticValidation = hydrated.SemanticValidation
				resolution.SuggestedCategory = categorySuggestionFromResolution(hydrated, semanticCategoryReviewNote(hydrated.SemanticValidation))
				resolution.ReviewNotes = append(resolution.ReviewNotes, semanticCategoryReviewNote(hydrated.SemanticValidation))
			}
			if hydrated := hydrateCategoryResolutionFromTree(tree, selectedID, "ai_category_tree", treeQuery); hydrated != nil {
				hydrated.SemanticValidation = r.semanticValidation(ctx, canonical, pkg, hydrated.MatchedPath)
				if r.acceptsAutomatedCategory(hydrated.SemanticValidation) {
					return hydrated
				}
				hydrated.Status = "partial"
				hydrated.SuggestedCategory = categorySuggestionFromResolution(hydrated, semanticCategoryReviewNote(hydrated.SemanticValidation))
				hydrated.CategoryID = 0
				hydrated.CategoryIDList = nil
				hydrated.ProductTypeID = 0
				hydrated.TopCategoryID = 0
				hydrated.ReviewNotes = append(hydrated.ReviewNotes, semanticCategoryReviewNote(hydrated.SemanticValidation))
				return hydrated
			}
			resolution.Source = "ai_category_tree"
			resolution.Status = "partial"
			resolution.QueryText = treeQuery
			resolution.CategoryID = selectedID
			resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 分类树重选已命中，但未能拉取完整类目详情")
			return resolution
		}
	}
	if len(resolution.MatchedPath) > 0 {
		resolution.ReviewNotes = append(resolution.ReviewNotes, "当前仅保留 SHEIN 类目路径名称，尚未解析到真实 category_id")
	} else {
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 类目解析未命中，请补充 target_category_hint 或接入类目接口")
	}
	return resolution
}

func hydrateCategoryResolutionFromTree(tree *sheincategory.CategoryTreeResponse, categoryID int, source, query string) *CategoryResolution {
	if tree == nil || categoryID <= 0 {
		return nil
	}
	path, ok := findCategoryTreePath(tree.Data, categoryID, nil)
	if !ok || len(path) == 0 {
		return nil
	}
	last := path[len(path)-1]
	matchedPath := make([]string, 0, len(path))
	idList := make([]int, 0, len(path))
	for _, node := range path {
		if strings.TrimSpace(node.CategoryName) != "" {
			matchedPath = append(matchedPath, node.CategoryName)
		}
		if node.CategoryID > 0 {
			idList = append(idList, node.CategoryID)
		}
	}
	resolution := &CategoryResolution{
		Status:         "resolved",
		Source:         source,
		QueryText:      query,
		MatchedPath:    matchedPath,
		CategoryID:     last.CategoryID,
		CategoryIDList: idList,
		ProductTypeID:  last.ProductTypeID,
	}
	if len(path) > 0 {
		resolution.TopCategoryID = path[0].CategoryID
	}
	if len(matchedPath) == 0 || len(idList) == 0 {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "类目树已命中，但类目路径信息不完整")
	}
	return resolution
}

func findCategoryTreePath(nodes []sheincategory.CategoryTreeNode, categoryID int, parents []sheincategory.CategoryTreeNode) ([]sheincategory.CategoryTreeNode, bool) {
	for _, node := range nodes {
		nextPath := append(append([]sheincategory.CategoryTreeNode(nil), parents...), node)
		if node.CategoryID == categoryID {
			return nextPath, true
		}
		if path, ok := findCategoryTreePath(node.Children, categoryID, nextPath); ok {
			return path, true
		}
	}
	return nil, false
}

func firstNonEmptyCategoryQuery(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func formatCategoryResolutionAPIError(err error) string {
	if authErr, ok := sheinapi.IsAuthenticationExpired(err); ok {
		return "SHEIN 类目在线解析失败: " + authErr.Error()
	}
	return "SHEIN 类目在线解析失败: " + strings.TrimSpace(err.Error())
}

func formatCategoryTreeResolutionAPIError(err error) string {
	if authErr, ok := sheinapi.IsAuthenticationExpired(err); ok {
		return "SHEIN 类目树加载失败: " + authErr.Error()
	}
	return "SHEIN 类目树加载失败: " + strings.TrimSpace(err.Error())
}

func hydrateCategoryResolution(info *sheincategory.CategoryInfo, source, query string) *CategoryResolution {
	if info == nil {
		return &CategoryResolution{Status: "unresolved", Source: source, QueryText: query}
	}
	matchedPath := buildMatchedPath(info)
	resolution := &CategoryResolution{
		Status:         "resolved",
		Source:         source,
		QueryText:      query,
		MatchedPath:    matchedPath,
		CategoryID:     info.CategoryID,
		CategoryIDList: buildCategoryIDList(info),
		ProductTypeID:  info.ProductTypeID,
		TopCategoryID:  info.LevelOneCategoryID,
	}
	if len(matchedPath) == 0 {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "类目详情已返回，但类目路径信息不完整")
	}
	return resolution
}

func (r *categoryResolver) semanticValidation(ctx context.Context, canonical *canonical.Product, pkg *Package, categoryPath []string) *CategorySemanticValidation {
	if validation := validateChildrenCategoryCompatibility(canonical, pkg, categoryPath); validation != nil {
		return validation
	}
	if r == nil || r.semanticVerifier == nil || len(categoryPath) == 0 {
		return nil
	}
	return r.semanticVerifier.ValidateProductCategory(ctx, canonical, pkg, categoryPath)
}

func semanticRejectsCategory(validation *CategorySemanticValidation) bool {
	return validation != nil && strings.EqualFold(strings.TrimSpace(validation.Verdict), "incompatible")
}

func semanticAcceptsCategory(validation *CategorySemanticValidation) bool {
	return validation != nil && strings.EqualFold(strings.TrimSpace(validation.Verdict), "compatible")
}

func (r *categoryResolver) acceptsAutomatedCategory(validation *CategorySemanticValidation) bool {
	return !semanticRejectsCategory(validation)
}

func semanticCategoryReviewNote(validation *CategorySemanticValidation) string {
	if validation == nil {
		return "SHEIN 类目语义校验未完成，候选类目需人工确认"
	}
	if semanticRejectsCategory(validation) {
		return buildSemanticCategoryReviewNote(validation)
	}
	reason := strings.TrimSpace(validation.Reason)
	if reason == "" {
		reason = "AI 未能确认当前类目路径与商品语义完全匹配"
	}
	if len(validation.ComparedPath) == 0 {
		return reason
	}
	return reason + "（候选类目: " + strings.Join(validation.ComparedPath, " > ") + "）"
}

func categorySuggestionFromResolution(resolution *CategoryResolution, reason string) *CategorySuggestion {
	if resolution == nil {
		return nil
	}
	return &CategorySuggestion{
		Source:         resolution.Source,
		Reason:         strings.TrimSpace(reason),
		MatchedPath:    append([]string(nil), resolution.MatchedPath...),
		CategoryID:     resolution.CategoryID,
		CategoryIDList: append([]int(nil), resolution.CategoryIDList...),
		ProductTypeID:  resolution.ProductTypeID,
		TopCategoryID:  resolution.TopCategoryID,
	}
}

func buildSemanticCategoryReviewNote(validation *CategorySemanticValidation) string {
	if validation == nil {
		return ""
	}
	reason := strings.TrimSpace(validation.Reason)
	if reason == "" {
		reason = "AI 判断当前类目路径与商品语义不一致"
	}
	if len(validation.ComparedPath) == 0 {
		return reason
	}
	return reason + "（候选类目: " + strings.Join(validation.ComparedPath, " > ") + "）"
}

func buildMatchedPath(info *sheincategory.CategoryInfo) []string {
	if info == nil {
		return nil
	}
	path := make([]string, 0, 4)
	for _, name := range []string{info.LevelOneCategoryName, info.LevelTwoCategoryName, info.LevelThreeCategoryName} {
		if strings.TrimSpace(name) != "" {
			path = append(path, name)
		}
	}
	if info.LevelFourCategoryName != nil && strings.TrimSpace(*info.LevelFourCategoryName) != "" {
		path = append(path, *info.LevelFourCategoryName)
	}
	return path
}

func buildCategoryIDList(info *sheincategory.CategoryInfo) []int {
	if info == nil {
		return nil
	}
	idList := make([]int, 0, 4)
	for _, id := range []int{info.LevelOneCategoryID, info.LevelTwoCategoryID, info.LevelThreeCategoryID} {
		if id > 0 {
			idList = append(idList, id)
		}
	}
	if info.LevelFourCategoryID != nil && *info.LevelFourCategoryID > 0 {
		idList = append(idList, *info.LevelFourCategoryID)
	}
	return idList
}

func resolveCategoryPath(canonical *canonical.Product, pkg *Package) []string {
	if pkg != nil && len(pkg.CategoryPath) > 0 {
		return pkg.CategoryPath
	}
	if canonical != nil && len(canonical.CategoryPath) > 0 {
		return canonical.CategoryPath
	}
	return nil
}

func parseFirstPositiveInt(raw string) int {
	for _, token := range strings.FieldsFunc(raw, func(r rune) bool { return r < '0' || r > '9' }) {
		if token == "" {
			continue
		}
		if parsed, err := strconv.Atoi(token); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 0
}
