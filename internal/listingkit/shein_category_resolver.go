package listingkit

import (
	"strconv"
	"strings"

	"task-processor/internal/productenrich"
	sheincategory "task-processor/internal/shein/api/category"
)

type sheinCategoryResolver struct {
	api SheinCategoryAPI
}

func NewSheinCategoryResolver(api SheinCategoryAPI) SheinCategoryResolver {
	return &sheinCategoryResolver{api: api}
}

func (r *sheinCategoryResolver) Resolve(req *GenerateRequest, canonical *productenrich.CanonicalProduct, pkg *SheinPackage) *SheinCategoryResolution {
	resolution := &SheinCategoryResolution{
		Status:      "unresolved",
		Source:      "fallback",
		QueryText:   buildSheinCategoryQuery(req, canonical, pkg),
		MatchedPath: append([]string(nil), resolveSheinCategoryPath(canonical, pkg)...),
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
			return hydrateSheinCategoryResolution(info, resolution.Source, resolution.QueryText)
		}
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "target_category_hint 已解析，但未能拉取完整类目层级")
		return resolution
	}

	if r.api != nil && strings.TrimSpace(resolution.QueryText) != "" {
		resp, err := r.api.SuggestCategoryByText(resolution.QueryText)
		if err == nil && resp != nil && len(resp.Data) > 0 {
			if suggestedID, convErr := strconv.Atoi(strings.TrimSpace(resp.Data[0].CategoryID)); convErr == nil && suggestedID > 0 {
				if info, infoErr := r.api.GetCategory(suggestedID); infoErr == nil && info != nil {
					return hydrateSheinCategoryResolution(info, "suggest_category", resolution.QueryText)
				}
				resolution.Source = "suggest_category"
				resolution.Status = "partial"
				resolution.CategoryID = suggestedID
				resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 推荐类目已命中，但未能拉取完整类目详情")
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

func hydrateSheinCategoryResolution(info *sheincategory.CategoryInfo, source, query string) *SheinCategoryResolution {
	if info == nil {
		return &SheinCategoryResolution{Status: "unresolved", Source: source, QueryText: query}
	}
	matchedPath := buildSheinMatchedPath(info)
	resolution := &SheinCategoryResolution{
		Status:         "resolved",
		Source:         source,
		QueryText:      query,
		MatchedPath:    matchedPath,
		CategoryID:     info.CategoryID,
		CategoryIDList: buildSheinCategoryIDList(info),
		ProductTypeID:  info.ProductTypeID,
		TopCategoryID:  info.LevelOneCategoryID,
	}
	if len(matchedPath) == 0 {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "类目详情已返回，但类目路径信息不完整")
	}
	return resolution
}

func buildSheinMatchedPath(info *sheincategory.CategoryInfo) []string {
	if info == nil {
		return nil
	}
	path := make([]string, 0, 4)
	for _, name := range []string{
		info.LevelOneCategoryName,
		info.LevelTwoCategoryName,
		info.LevelThreeCategoryName,
	} {
		if strings.TrimSpace(name) != "" {
			path = append(path, name)
		}
	}
	if info.LevelFourCategoryName != nil && strings.TrimSpace(*info.LevelFourCategoryName) != "" {
		path = append(path, *info.LevelFourCategoryName)
	}
	return path
}

func buildSheinCategoryIDList(info *sheincategory.CategoryInfo) []int {
	if info == nil {
		return nil
	}
	idList := make([]int, 0, 4)
	for _, id := range []int{
		info.LevelOneCategoryID,
		info.LevelTwoCategoryID,
		info.LevelThreeCategoryID,
	} {
		if id > 0 {
			idList = append(idList, id)
		}
	}
	if info.LevelFourCategoryID != nil && *info.LevelFourCategoryID > 0 {
		idList = append(idList, *info.LevelFourCategoryID)
	}
	return idList
}

func buildSheinCategoryQuery(req *GenerateRequest, canonical *productenrich.CanonicalProduct, pkg *SheinPackage) string {
	candidates := []string{
		req.TargetCategoryHint,
		canonical.Title,
		firstNonEmpty(pkg.CategoryName, lastCategory(canonical.CategoryPath)),
		canonical.Description,
		req.Text,
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func resolveSheinCategoryPath(canonical *productenrich.CanonicalProduct, pkg *SheinPackage) []string {
	if pkg != nil && len(pkg.CategoryPath) > 0 {
		return pkg.CategoryPath
	}
	if canonical != nil && len(canonical.CategoryPath) > 0 {
		return canonical.CategoryPath
	}
	return nil
}

func parseFirstPositiveInt(raw string) int {
	for _, token := range strings.FieldsFunc(raw, func(r rune) bool {
		return r < '0' || r > '9'
	}) {
		if token == "" {
			continue
		}
		if parsed, err := strconv.Atoi(token); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 0
}
