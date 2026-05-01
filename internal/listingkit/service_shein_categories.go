package listingkit

import (
	"context"
	"fmt"
	"sort"
	"strings"

	sheincategory "task-processor/internal/shein/api/category"
	sheinclient "task-processor/internal/shein/client"
)

const maxSheinCategorySearchResults = 20

func (s *service) SearchSheinCategories(ctx context.Context, taskID string, query string) (*SheinCategorySearchResult, error) {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return nil, ErrInvalidSheinCategorySearchQuery
	}

	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	storeID := s.resolveSheinStoreID(task)
	if storeID <= 0 {
		return nil, fmt.Errorf("shein store id is unavailable for category search")
	}
	if s.sheinManagementClient == nil {
		return nil, fmt.Errorf("shein management client is unavailable for category search")
	}

	apiClient := sheinclient.NewAPIClient(storeID, s.sheinManagementClient)
	if !apiClient.HasCookies() {
		return nil, fmt.Errorf("shein store cookies are unavailable for category search")
	}

	baseAPI := sheinclient.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		storeID,
		apiClient.GetHTTPClient(),
	)
	categoryAPI := sheincategory.NewClient(baseAPI)
	tree, err := categoryAPI.GetCategoryTree()
	if err != nil {
		return nil, err
	}

	return &SheinCategorySearchResult{
		TaskID: taskID,
		Query:  trimmedQuery,
		Items:  searchSheinCategoryCandidates(tree.Data, trimmedQuery),
	}, nil
}

func (s *service) resolveSheinStoreID(task *Task) int64 {
	if task != nil && task.Request != nil && task.Request.SheinStoreID > 0 {
		return task.Request.SheinStoreID
	}

	s.sheinSettingsMu.RLock()
	defer s.sheinSettingsMu.RUnlock()
	return s.sheinSettings.DefaultStoreID
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
