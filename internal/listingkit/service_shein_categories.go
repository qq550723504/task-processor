package listingkit

import (
	"context"
	"fmt"
	"sort"
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
	sheinclient "task-processor/internal/shein/client"
)

const maxSheinCategorySearchResults = 20

func (s *service) SearchSheinCategories(ctx context.Context, taskID string, query string) (*SheinCategorySearchResult, error) {
	return s.sheinAdminOrDefault().SearchSheinCategories(ctx, taskID, query)
}

func (s *service) buildSheinAttributeAPI(ctx context.Context, task *Task) (sheinpub.AttributeAPI, error) {
	apiClient, storeID, err := s.newSheinAPIClient(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("%w for attribute resolution", err)
	}
	if !apiClient.HasCookies() {
		if err := apiClient.ForceRefreshCookies(); err != nil {
			return nil, fmt.Errorf("shein store cookies are unavailable for attribute resolution: %w", err)
		}
	}
	if !apiClient.HasCookies() {
		return nil, fmt.Errorf("shein store cookies are unavailable for attribute resolution")
	}

	baseAPI := sheinclient.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		storeID,
		apiClient.GetHTTPClient(),
	)
	baseAPI.SetAuthRefreshFunc(apiClient.ForceRefreshCookies)
	return sheinattribute.NewClient(baseAPI), nil
}

func (s *service) buildSheinCategoryAPI(ctx context.Context, task *Task) (sheincategory.CategoryAPI, error) {
	apiClient, storeID, err := s.newSheinAPIClient(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("%w for category resolution", err)
	}
	if !apiClient.HasCookies() {
		if err := apiClient.ForceRefreshCookies(); err != nil {
			return nil, fmt.Errorf("shein store cookies are unavailable for category resolution: %w", err)
		}
	}
	if !apiClient.HasCookies() {
		return nil, fmt.Errorf("shein store cookies are unavailable for category resolution")
	}

	baseAPI := sheinclient.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		storeID,
		apiClient.GetHTTPClient(),
	)
	baseAPI.SetAuthRefreshFunc(apiClient.ForceRefreshCookies)
	return sheincategory.NewClient(baseAPI), nil
}

func (s *service) resolveSheinStoreID(ctx context.Context, task *Task) (int64, error) {
	if task != nil && task.Request != nil && task.Request.SheinStoreID > 0 {
		return task.Request.SheinStoreID, nil
	}
	if selection, err := s.resolveSheinStoreSelection(ctx, task); err == nil && selection != nil && selection.Profile != nil && selection.Profile.StoreID > 0 {
		return selection.Profile.StoreID, nil
	}

	s.sheinSettingsMu.RLock()
	defer s.sheinSettingsMu.RUnlock()
	return s.sheinSettings.DefaultStoreID, nil
}

func (s *service) resolveSheinStoreProfile(ctx context.Context, task *Task) (*ListingKitStoreProfile, error) {
	selection, err := s.resolveSheinStoreSelection(ctx, task)
	if err != nil {
		return nil, err
	}
	if selection == nil {
		return nil, fmt.Errorf("store profile is unavailable")
	}
	return cloneStoreProfile(selection.Profile), nil
}

type sheinStoreSelection struct {
	Profile          *ListingKitStoreProfile
	Strategy         string
	Reason           string
	MatchedRuleKinds []string
	ManualOverride   bool
	Fallback         bool
}

func (s *service) resolveSheinStoreSelection(ctx context.Context, task *Task) (*sheinStoreSelection, error) {
	if snapshot := sheinStoreResolutionSnapshotFromTask(task); snapshot != nil && snapshot.StoreID > 0 {
		return selectionFromSnapshot(snapshot), nil
	}
	if s == nil || s.storeProfileRepo == nil {
		return nil, fmt.Errorf("store profile repository is not configured")
	}
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		tenantID = tenantIDInt64FromTask(task)
	}
	if tenantID <= 0 {
		return nil, fmt.Errorf("tenant id is unavailable")
	}
	items, err := s.storeProfileRepo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("store profile is unavailable")
	}
	enabled := make([]ListingKitStoreProfile, 0, len(items))
	for idx := range items {
		if items[idx].Enabled {
			enabled = append(enabled, items[idx])
		}
	}
	if len(enabled) == 0 {
		return nil, fmt.Errorf("store profile is unavailable")
	}
	if task != nil && task.Request != nil && task.Request.SheinStoreID > 0 {
		for idx := range enabled {
			if enabled[idx].StoreID == task.Request.SheinStoreID {
				return &sheinStoreSelection{
					Profile:        cloneStoreProfile(&enabled[idx]),
					Strategy:       "manual",
					Reason:         "任务显式指定了 SHEIN 店铺。",
					ManualOverride: true,
				}, nil
			}
		}
	}
	settings, err := s.routingSettingsRepo.GetByTenant(ctx, tenantID)
	var fallback *ListingKitStoreProfile
	for idx := range enabled {
		if enabled[idx].IsFallback && fallback == nil {
			fallback = cloneStoreProfile(&enabled[idx])
		}
	}
	if err == nil && settings != nil && settings.FallbackStoreID > 0 {
		for idx := range enabled {
			if enabled[idx].StoreID == settings.FallbackStoreID {
				fallback = cloneStoreProfile(&enabled[idx])
				break
			}
		}
	}
	if settings != nil && settings.SelectionStrategy != "manual" {
		if matched := matchStoreProfileForTask(enabled, task, settings.SelectionStrategy); matched != nil {
			return &sheinStoreSelection{
				Profile:          cloneStoreProfile(matched.profile),
				Strategy:         settings.SelectionStrategy,
				Reason:           routeSelectionReason(settings.SelectionStrategy, matched.kinds),
				MatchedRuleKinds: append([]string(nil), matched.kinds...),
			}, nil
		}
		if settings.AllowFallback && fallback != nil {
			return &sheinStoreSelection{
				Profile:  cloneStoreProfile(fallback),
				Strategy: settings.SelectionStrategy,
				Reason:   "没有命中路由规则，已回退到 fallback 店铺。",
				Fallback: true,
			}, nil
		}
	}
	if settings != nil && settings.AllowFallback && settings.FallbackStoreID > 0 && fallback != nil {
		return &sheinStoreSelection{
			Profile:  cloneStoreProfile(fallback),
			Strategy: firstNonEmpty(strings.TrimSpace(settings.SelectionStrategy), "manual"),
			Reason:   "当前使用配置的 fallback 店铺作为兜底。",
			Fallback: true,
		}, nil
	}
	if len(enabled) > 0 {
		strategy := "priority"
		reason := "当前未显式指定店铺，使用已启用店铺里的最高优先级项。"
		if settings != nil && settings.SelectionStrategy == "manual" {
			strategy = "manual"
			reason = "当前未显式指定店铺，按优先级选择默认店铺。"
		}
		return &sheinStoreSelection{
			Profile:  cloneStoreProfile(&enabled[0]),
			Strategy: strategy,
			Reason:   reason,
		}, nil
	}
	return nil, fmt.Errorf("store profile is unavailable")
}

func sheinStoreResolutionSnapshotFromTask(task *Task) *SheinStoreResolutionSnapshot {
	if task == nil || task.SheinStoreResolutionSnapshot == nil || task.SheinStoreResolutionSnapshot.StoreID <= 0 {
		return nil
	}
	return task.SheinStoreResolutionSnapshot
}

func selectionFromSnapshot(snapshot *SheinStoreResolutionSnapshot) *sheinStoreSelection {
	if snapshot == nil || snapshot.StoreID <= 0 {
		return nil
	}
	return &sheinStoreSelection{
		Profile: &ListingKitStoreProfile{
			ID:                snapshot.MatchedProfileID,
			StoreID:           snapshot.StoreID,
			Enabled:           true,
			Site:              snapshot.Site,
			WarehouseCode:     snapshot.WarehouseCode,
			DefaultStock:      snapshot.DefaultStock,
			DefaultSubmitMode: snapshot.DefaultSubmitMode,
			Pricing:           snapshot.Pricing,
		},
		Strategy:         snapshot.Strategy,
		Reason:           snapshot.Reason,
		MatchedRuleKinds: append([]string(nil), snapshot.MatchedRuleKinds...),
		ManualOverride:   snapshot.ManualOverride,
		Fallback:         snapshot.Fallback,
	}
}

type matchedStoreProfile struct {
	profile *ListingKitStoreProfile
	kinds   []string
}

func matchStoreProfileForTask(
	items []ListingKitStoreProfile,
	task *Task,
	strategy string,
) *matchedStoreProfile {
	if len(items) == 0 || task == nil || task.Request == nil {
		return nil
	}
	type scoredProfile struct {
		profile ListingKitStoreProfile
		score   int
		kinds   []string
	}
	scored := make([]scoredProfile, 0, len(items))
	for idx := range items {
		score, kinds := storeProfileMatchScore(&items[idx], task, strategy)
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredProfile{profile: items[idx], score: score, kinds: kinds})
	}
	if len(scored) == 0 {
		return nil
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		if scored[i].profile.Priority != scored[j].profile.Priority {
			return scored[i].profile.Priority < scored[j].profile.Priority
		}
		return scored[i].profile.ID < scored[j].profile.ID
	})
	return &matchedStoreProfile{
		profile: cloneStoreProfile(&scored[0].profile),
		kinds:   append([]string(nil), scored[0].kinds...),
	}
}

func storeProfileMatchScore(profile *ListingKitStoreProfile, task *Task, strategy string) (int, []string) {
	if profile == nil || task == nil || task.Request == nil {
		return 0, nil
	}
	request := task.Request
	country := strings.ToLower(strings.TrimSpace(request.Country))
	categoryText := strings.ToLower(strings.TrimSpace(strings.Join(extractTaskCategoryHints(request), " ")))
	total := 0
	matchedRules := 0
	kinds := make([]string, 0, 2)
	for _, rule := range profile.MatchRules {
		kind := strings.ToLower(strings.TrimSpace(rule.Kind))
		switch kind {
		case "country":
			if country != "" && ruleValuesContain(rule.Values, country) {
				total += 100
				matchedRules++
				kinds = append(kinds, "country")
			}
		case "category":
			if categoryText != "" && ruleValuesMatchCategory(rule.Values, categoryText) {
				total += 40
				matchedRules++
				kinds = append(kinds, "category")
			}
		}
	}
	if matchedRules > 0 {
		return total + matchedRules, uniqueStrings(kinds)
	}
	if strategy != "country" {
		return 0, nil
	}
	if country == "" {
		return 0, nil
	}
	if strings.EqualFold(strings.TrimSpace(profile.Site), country) {
		return 60, []string{"country"}
	}
	if profile.Store != nil && strings.EqualFold(strings.TrimSpace(profile.Store.Region), country) {
		return 60, []string{"country"}
	}
	return 0, nil
}

func extractTaskCategoryHints(request *GenerateRequest) []string {
	if request == nil {
		return nil
	}
	hints := make([]string, 0, 4)
	if trimmed := strings.TrimSpace(request.TargetCategoryHint); trimmed != "" {
		hints = append(hints, trimmed)
	}
	if request.Options != nil && request.Options.SDS != nil && len(request.Options.SDS.CategoryPath) > 0 {
		hints = append(hints, strings.Join(request.Options.SDS.CategoryPath, " "))
	}
	return hints
}

func ruleValuesContain(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), want) {
			return true
		}
	}
	return false
}

func ruleValuesMatchCategory(values []string, categoryText string) bool {
	if categoryText == "" {
		return false
	}
	for _, value := range values {
		needle := strings.ToLower(strings.TrimSpace(value))
		if needle == "" {
			continue
		}
		if strings.Contains(categoryText, needle) {
			return true
		}
	}
	return false
}

func routeSelectionReason(strategy string, kinds []string) string {
	switch strategy {
	case "country":
		return "根据任务国家信息命中了对应店铺。"
	case "priority":
		if len(kinds) > 0 {
			return "根据店铺匹配规则命中后，再按优先级选中了当前店铺。"
		}
		return "根据优先级选中了当前店铺。"
	default:
		if len(kinds) > 0 {
			return "根据店铺匹配规则选中了当前店铺。"
		}
		return "系统自动选中了当前店铺。"
	}
}

type sheinCategorySearchMatch struct {
	candidate SheinCategorySearchCandidate
	score     int
}

func searchSheinCategoryCandidates(nodes []sheincategory.CategoryTreeNode, query string) []SheinCategorySearchCandidate {
	matches := make([]sheinCategorySearchMatch, 0)
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	tokens := strings.Fields(normalizedQuery)

	var walk func(node sheincategory.CategoryTreeNode, pathNames []string, pathIDs []int)
	walk = func(node sheincategory.CategoryTreeNode, pathNames []string, pathIDs []int) {
		currentPathNames := append(append([]string(nil), pathNames...), strings.TrimSpace(node.CategoryName))
		currentPathIDs := append(append([]int(nil), pathIDs...), node.CategoryID)
		if node.LastCategory || len(node.Children) == 0 {
			if score, ok := sheinCategoryMatchScore(currentPathNames, normalizedQuery, tokens); ok {
				topCategoryID := 0
				if len(currentPathIDs) > 0 {
					topCategoryID = currentPathIDs[0]
				}
				matches = append(matches, sheinCategorySearchMatch{
					candidate: SheinCategorySearchCandidate{
						CategoryID:     node.CategoryID,
						CategoryIDList: currentPathIDs,
						CategoryPath:   currentPathNames,
						ProductTypeID:  node.ProductTypeID,
						TopCategoryID:  topCategoryID,
						Source:         "manual_search",
						MatchReason:    "keyword_match",
					},
					score: score,
				})
			}
			return
		}
		for _, child := range node.Children {
			walk(child, currentPathNames, currentPathIDs)
		}
	}

	for _, node := range nodes {
		walk(node, nil, nil)
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		if len(matches[i].candidate.CategoryPath) != len(matches[j].candidate.CategoryPath) {
			return len(matches[i].candidate.CategoryPath) < len(matches[j].candidate.CategoryPath)
		}
		return strings.Join(matches[i].candidate.CategoryPath, " > ") < strings.Join(matches[j].candidate.CategoryPath, " > ")
	})

	if len(matches) > maxSheinCategorySearchResults {
		matches = matches[:maxSheinCategorySearchResults]
	}

	items := make([]SheinCategorySearchCandidate, 0, len(matches))
	for _, match := range matches {
		items = append(items, match.candidate)
	}
	return items
}

func sheinCategoryMatchScore(path []string, normalizedQuery string, tokens []string) (int, bool) {
	if len(path) == 0 {
		return 0, false
	}
	leaf := strings.ToLower(strings.TrimSpace(path[len(path)-1]))
	joined := strings.ToLower(strings.Join(path, " > "))
	if normalizedQuery == "" {
		return 0, false
	}

	score := 0
	switch {
	case leaf == normalizedQuery:
		score += 120
	case strings.HasPrefix(leaf, normalizedQuery):
		score += 90
	case strings.Contains(leaf, normalizedQuery):
		score += 70
	case strings.Contains(joined, normalizedQuery):
		score += 50
	default:
		return 0, false
	}

	for _, token := range tokens {
		if token == "" {
			continue
		}
		if !strings.Contains(joined, token) {
			return 0, false
		}
		score += 5
	}

	return score, true
}
