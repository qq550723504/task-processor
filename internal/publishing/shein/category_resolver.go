package shein

import (
	"strconv"
	"strings"

	"task-processor/internal/productenrich"
	common "task-processor/internal/publishing/common"
	sheinapi "task-processor/internal/shein/api"
	sheincategory "task-processor/internal/shein/api/category"
)

type categoryResolver struct {
	api          CategoryAPI
	suggestFallback categorySuggestFallback
	treeFallback categoryTreeFallback
}

func NewCategoryResolver(api CategoryAPI) CategoryResolver { return &categoryResolver{api: api} }

func NewCategoryResolverWithFallbacks(api CategoryAPI, suggestFallback categorySuggestFallback, treeFallback categoryTreeFallback) CategoryResolver {
	return &categoryResolver{
		api:             api,
		suggestFallback: suggestFallback,
		treeFallback:    treeFallback,
	}
}

func NewCategoryResolverWithTreeFallback(api CategoryAPI, treeFallback categoryTreeFallback) CategoryResolver {
	return NewCategoryResolverWithFallbacks(api, nil, treeFallback)
}

func (r *categoryResolver) Resolve(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) *CategoryResolution {
	resolution := &CategoryResolution{
		Status:      "unresolved",
		Source:      "fallback",
		QueryText:   buildCategoryQuery(req, canonical, pkg),
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
			return hydrateCategoryResolution(info, resolution.Source, resolution.QueryText)
		}
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "target_category_hint 已解析，但未能拉取完整类目层级")
		return resolution
	}
	if r.api != nil && strings.TrimSpace(resolution.QueryText) != "" {
		resp, err := r.api.SuggestCategoryByText(resolution.QueryText)
		if err != nil {
			resolution.Status = "partial"
			resolution.ReviewNotes = append(resolution.ReviewNotes, formatCategoryResolutionAPIError(err))
			if r.treeFallback == nil {
				return resolution
			}
		} else if resp != nil && len(resp.Data) > 0 {
			if suggestedID, convErr := strconv.Atoi(strings.TrimSpace(resp.Data[0].CategoryID)); convErr == nil && suggestedID > 0 {
				if info, infoErr := r.api.GetCategory(suggestedID); infoErr == nil && info != nil {
					return hydrateCategoryResolution(info, "suggest_category", resolution.QueryText)
				}
				resolution.Source = "suggest_category"
				resolution.Status = "partial"
				resolution.CategoryID = suggestedID
				resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 推荐类目已命中，但未能拉取完整类目详情")
				return resolution
			}
		}
		if r.treeFallback != nil {
			tree, treeErr := r.api.GetCategoryTree()
			if treeErr != nil {
				resolution.Status = "partial"
				resolution.ReviewNotes = append(resolution.ReviewNotes, formatCategoryTreeResolutionAPIError(treeErr))
				return resolution
			}
			selectedID, selectErr := r.treeFallback.SelectCategoryID(resolution.QueryText, tree)
			if selectErr != nil {
				resolution.Status = "partial"
				resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 类目树候选重选失败: "+strings.TrimSpace(selectErr.Error()))
				return resolution
			}
			if selectedID > 0 {
				if info, infoErr := r.api.GetCategory(selectedID); infoErr == nil && info != nil {
					return hydrateCategoryResolution(info, "ai_category_tree", resolution.QueryText)
				}
				resolution.Source = "ai_category_tree"
				resolution.Status = "partial"
				resolution.CategoryID = selectedID
				resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 分类树重选已命中，但未能拉取完整类目详情")
				return resolution
			}
		}
	}
	if len(resolution.MatchedPath) > 0 {
		resolution.ReviewNotes = append(resolution.ReviewNotes, "当前仅保留 SHEIN 类目路径名称，尚未解析到真实 category_id")
	} else {
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 类目解析未命中，请补充 target_category_hint 或接入类目接口")
	}
	return resolution
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

func (r *categoryResolver) SuggestAlternative(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) *CategorySuggestion {
	if r == nil || r.api == nil {
		return nil
	}
	query := buildCategorySuggestionQuery(req, canonical, pkg)
	if strings.TrimSpace(query) == "" {
		return nil
	}
	tryCandidate := func(selectedID int, source, reason string) *CategorySuggestion {
		if selectedID <= 0 {
			return nil
		}
		if pkg != nil && selectedID == pkg.CategoryID {
			return nil
		}
		info, err := r.api.GetCategory(selectedID)
		if err != nil || info == nil {
			return nil
		}
		resolution := hydrateCategoryResolution(info, source, query)
		if resolution == nil || resolution.CategoryID <= 0 {
			return nil
		}
		suggestion := &CategorySuggestion{
			Source:         resolution.Source,
			Reason:         reason,
			MatchedPath:    append([]string(nil), resolution.MatchedPath...),
			CategoryID:     resolution.CategoryID,
			CategoryIDList: append([]int(nil), resolution.CategoryIDList...),
			ProductTypeID:  resolution.ProductTypeID,
			TopCategoryID:  resolution.TopCategoryID,
		}
		if !shouldAcceptSuggestedCategory(canonical, pkg, suggestion) {
			return nil
		}
		return suggestion
	}

	if r.suggestFallback != nil {
		selectedID, err := r.suggestFallback.SelectCategoryID(query, r.api)
		if err == nil {
			if suggestion := tryCandidate(selectedID, "suggest_category", "当前类目模板不适配销售属性结构，建议优先复核该推荐类目"); suggestion != nil {
				return suggestion
			}
		}
	}

	if r.treeFallback == nil {
		return nil
	}
	tree, err := r.api.GetCategoryTree()
	if err != nil || tree == nil {
		return nil
	}
	selectedID, err := r.treeFallback.SelectCategoryID(query, tree)
	if err != nil {
		return nil
	}
	return tryCandidate(selectedID, "ai_category_tree", "当前类目模板不适配销售属性结构，建议复核该候选类目")
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

func buildCategoryQuery(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) string {
	candidates := []string{req.TargetCategoryHint, canonical.Title, common.FirstNonEmpty(pkg.CategoryName, common.LastCategory(canonical.CategoryPath)), canonical.Description, req.Text}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func buildCategorySuggestionQuery(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) string {
	candidates := []string{
		canonical.Title,
		common.FirstNonEmpty(pkg.CategoryName, common.LastCategory(canonical.CategoryPath)),
		canonical.Description,
		req.Text,
		req.TargetCategoryHint,
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func resolveCategoryPath(canonical *productenrich.CanonicalProduct, pkg *Package) []string {
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
