package design

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"time"

	sdstemplate "task-processor/internal/sds/template"
)

const (
	renderedImagePollInterval    = 5 * time.Second
	maxRenderedImagePollAttempts = 24
)

func (s *Service) fetchRenderedImageURLs(ctx context.Context, input PrepareSyncDesignInput, result *PrepareSyncDesignResult) []string {
	byProduct := s.fetchRenderedImageURLsByProduct(ctx, input, result)
	variantID := resolvedDesignVariantID(input, result)
	if urls := byProduct[variantID]; len(urls) > 0 {
		if result != nil {
			result.RenderedImageURLsByProduct = byProduct
		}
		return urls
	}
	if len(byProduct) > 0 && result != nil {
		result.RenderedImageURLsByProduct = byProduct
	}
	return nil
}

func (s *Service) fetchRenderedImageURLsByProduct(ctx context.Context, input PrepareSyncDesignInput, result *PrepareSyncDesignResult) map[int64][]string {
	parentProductID := input.ParentProductID
	if parentProductID <= 0 && result != nil && result.Page != nil {
		parentProductID = result.Page.Product.ParentID
		if parentProductID <= 0 {
			parentProductID = result.Page.MerchantProductParentID
		}
	}
	if parentProductID <= 0 {
		return nil
	}

	variantID := resolvedDesignVariantID(input, result)
	expectedCount := expectedRenderedImageCount(result)
	targetIDs := renderedTargetVariantIDs(input, variantID)
	var best map[int64][]string
	bestObservations := make(map[int64]RenderedImageObservation, len(targetIDs))
	sensitiveWordRepairAttempted := false
	expectedMaterialName := finishedProductMaterialName(result)
	finalize := func(current map[int64][]string) map[int64][]string {
		if result != nil {
			if len(current) > 0 {
				result.RenderedImageURLsByProduct = current
			}
			if len(bestObservations) > 0 {
				result.RenderedImageObservations = cloneRenderedImageObservations(bestObservations)
				result.RenderedSensitiveWords = s.fetchRenderedSensitiveWords(ctx, bestObservations)
			}
		}
		return current
	}
	for attempt := 0; attempt < maxRenderedImagePollAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return finalize(best)
			case <-time.After(renderedImagePollInterval):
			}
		}
		urlsByProduct, observations, sensitiveWords := s.fetchFinishedProductImageURLsByProduct(ctx, input, result, parentProductID, targetIDs)
		for productID, observation := range observations {
			bestObservations[productID] = observation
		}
		if len(sensitiveWords) > 0 {
			if result != nil {
				result.RenderedSensitiveWords = sensitiveWords
			}
			if !sensitiveWordRepairAttempted && s.repairSensitiveDesignProductExportNames(ctx, input, result, sensitiveWords) {
				sensitiveWordRepairAttempted = true
				continue
			}
		}
		if len(urlsByProduct) > 0 {
			best = preferredRenderedImageURLsByProduct(best, urlsByProduct)
			if renderedImageURLsByProductReady(urlsByProduct, targetIDs, expectedCount) {
				return finalize(urlsByProduct)
			}
		}
		if expectedMaterialName != "" {
			continue
		}
		detail, err := s.GetProductDetail(ctx, parentProductID)
		if err != nil {
			continue
		}
		urls := renderedImageURLsFromProduct(detail, variantID)
		if staleRenderedImageURLs(urls, result) {
			continue
		}
		if len(urls) > 0 {
			if best == nil {
				best = map[int64][]string{}
			}
			best[variantID] = preferredRenderedImageURLs(best[variantID], urls)
			if renderedImageURLsReady(urls, expectedCount) {
				return finalize(best)
			}
		}
	}
	return finalize(best)
}

func (s *Service) fetchFinishedProductImageURLs(ctx context.Context, input PrepareSyncDesignInput, result *PrepareSyncDesignResult, variantID int64, parentProductID int64) []string {
	if urls, _, _ := s.fetchFinishedProductImageURLsByProduct(ctx, input, result, parentProductID, []int64{variantID}); len(urls) > 0 {
		return urls[variantID]
	}
	return nil
}

func (s *Service) fetchFinishedProductImageURLsByProduct(ctx context.Context, input PrepareSyncDesignInput, result *PrepareSyncDesignResult, parentProductID int64, targetIDs []int64) (map[int64][]string, map[int64]RenderedImageObservation, map[string][]SensitiveWordHit) {
	if parentProductID <= 0 {
		return nil, nil, nil
	}
	if len(targetIDs) == 0 {
		return nil, nil, nil
	}
	list, err := s.ListDesignProducts(ctx, ListDesignProductsRequest{
		ParentProductID: parentProductID,
		DesignType:      input.DesignType,
		Page:            1,
		Size:            50,
	})
	if err != nil || list == nil || len(list.Items) == 0 {
		return nil, nil, nil
	}

	observations := collectRenderedImageObservations(list.Items, targetIDs)
	sensitiveWords := s.fetchRenderedSensitiveWords(ctx, observations)
	expectedMaterialName := finishedProductMaterialName(result)
	accept := func(urls []string) bool {
		return !s.renderedCandidateLooksBlank(ctx, urls, input.BlankDesignURL)
	}
	if urls := selectFinishedProductImageURLsByProductWithAccept(list.Items, targetIDs, expectedMaterialName, accept); len(urls) > 0 {
		return urls, observations, sensitiveWords
	}
	return selectFinishedProductImageURLsByProductWithAccept(list.Items, targetIDs, expectedMaterialName, nil), observations, sensitiveWords
}

func resolvedDesignVariantID(input PrepareSyncDesignInput, result *PrepareSyncDesignResult) int64 {
	if result != nil && result.Page != nil && result.Page.Product.ID > 0 {
		return result.Page.Product.ID
	}
	return input.VariantID
}

func renderedTargetVariantIDs(input PrepareSyncDesignInput, variantID int64) []int64 {
	values := make([]int64, 0, len(input.RelatedVariantIDs)+1)
	if variantID > 0 {
		values = append(values, variantID)
	}
	values = append(values, input.RelatedVariantIDs...)
	seen := map[int64]struct{}{}
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func selectFinishedProductImageURLs(items []DesignProductListItem, variantID int64, expectedMaterialName string) []string {
	return selectFinishedProductImageURLsWithAccept(items, variantID, expectedMaterialName, nil)
}

func selectFinishedProductImageURLsWithAccept(items []DesignProductListItem, variantID int64, expectedMaterialName string, accept func([]string) bool) []string {
	candidates := append([]DesignProductListItem(nil), items...)
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].FinishTime > candidates[j].FinishTime
	})
	if expectedMaterialName != "" {
		for _, item := range candidates {
			if urls := finishedProductItemImageURLs(item, variantID, expectedMaterialName); len(urls) > 0 {
				if accept != nil && !accept(urls) {
					continue
				}
				return urls
			}
		}
		return nil
	}
	for _, item := range candidates {
		if urls := finishedProductItemImageURLs(item, variantID, ""); len(urls) > 0 {
			if accept != nil && !accept(urls) {
				continue
			}
			return urls
		}
	}
	return nil
}

func selectFinishedProductImageURLsByProductWithAccept(items []DesignProductListItem, targetIDs []int64, expectedMaterialName string, accept func([]string) bool) map[int64][]string {
	targetSet := make(map[int64]struct{}, len(targetIDs))
	for _, id := range targetIDs {
		if id > 0 {
			targetSet[id] = struct{}{}
		}
	}
	if len(targetSet) == 0 {
		return nil
	}
	result := make(map[int64][]string, len(targetSet))
	candidates := append([]DesignProductListItem(nil), items...)
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].FinishTime > candidates[j].FinishTime
	})
	for _, item := range candidates {
		if _, ok := targetSet[item.ProductID]; !ok {
			continue
		}
		if _, exists := result[item.ProductID]; exists {
			continue
		}
		if urls := finishedProductItemImageURLs(item, item.ProductID, expectedMaterialName); len(urls) > 0 {
			if accept != nil && !accept(urls) {
				continue
			}
			result[item.ProductID] = urls
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func collectRenderedImageObservations(items []DesignProductListItem, targetIDs []int64) map[int64]RenderedImageObservation {
	targetSet := make(map[int64]struct{}, len(targetIDs))
	for _, id := range targetIDs {
		if id > 0 {
			targetSet[id] = struct{}{}
		}
	}
	if len(targetSet) == 0 {
		return nil
	}
	candidates := append([]DesignProductListItem(nil), items...)
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].FinishTime > candidates[j].FinishTime
	})
	result := make(map[int64]RenderedImageObservation, len(targetSet))
	for _, item := range candidates {
		if _, ok := targetSet[item.ProductID]; !ok {
			continue
		}
		if _, exists := result[item.ProductID]; exists {
			continue
		}
		result[item.ProductID] = RenderedImageObservation{
			ProductID:         item.ProductID,
			Found:             true,
			BuildFinish:       item.BuildFinish,
			Status:            item.Status,
			MaterialImageName: strings.TrimSpace(item.MaterialImageName),
			TaskID:            strings.TrimSpace(item.TaskID),
			DesignTaskID:      strings.TrimSpace(item.DesignTaskID),
			ItemID:            strings.TrimSpace(item.ID),
			ImageCount:        len(item.ImageURLs),
			ThumbnailCount:    len(item.ThumbnailImageURLs),
		}
	}
	return result
}

func cloneRenderedImageObservations(input map[int64]RenderedImageObservation) map[int64]RenderedImageObservation {
	if len(input) == 0 {
		return nil
	}
	result := make(map[int64]RenderedImageObservation, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func (s *Service) fetchRenderedSensitiveWords(ctx context.Context, observations map[int64]RenderedImageObservation) map[string][]SensitiveWordHit {
	if len(observations) == 0 || s == nil || s.client == nil {
		return nil
	}
	state := s.client.AuthState()
	if state == nil || state.MerchantID <= 0 {
		return nil
	}
	itemIDs := make([]string, 0, len(observations))
	seen := make(map[string]struct{}, len(observations))
	for _, observation := range observations {
		itemID := strings.TrimSpace(observation.ItemID)
		if itemID == "" {
			continue
		}
		if _, ok := seen[itemID]; ok {
			continue
		}
		seen[itemID] = struct{}{}
		itemIDs = append(itemIDs, itemID)
	}
	if len(itemIDs) == 0 {
		return nil
	}
	return s.ListSensitiveWordsByItemIDs(ctx, state.MerchantID, itemIDs)
}

func (s *Service) repairSensitiveDesignProductExportNames(ctx context.Context, input PrepareSyncDesignInput, result *PrepareSyncDesignResult, sensitiveWords map[string][]SensitiveWordHit) bool {
	if len(sensitiveWords) == 0 || s == nil {
		return false
	}
	parentProductID := input.ParentProductID
	if parentProductID <= 0 && result != nil && result.Page != nil {
		parentProductID = result.Page.Product.ParentID
		if parentProductID <= 0 {
			parentProductID = result.Page.MerchantProductParentID
		}
	}
	if parentProductID <= 0 {
		return false
	}
	list, err := s.ListDesignProducts(ctx, ListDesignProductsRequest{
		ParentProductID: parentProductID,
		DesignType:      input.DesignType,
		Page:            1,
		Size:            50,
	})
	if err != nil || list == nil || len(list.Items) == 0 {
		return false
	}
	updates := buildSensitiveDesignProductUpdates(list.Items, sensitiveWords)
	if len(updates) == 0 {
		return false
	}
	if err := s.UpdateDesignProducts(ctx, updates); err != nil {
		return false
	}
	return true
}

func finishedProductItemImageURLs(item DesignProductListItem, variantID int64, expectedMaterialName string) []string {
	if item.ProductID != 0 && item.ProductID != variantID {
		return nil
	}
	if !item.BuildFinish || len(item.ImageURLs) == 0 {
		return nil
	}
	if expectedMaterialName != "" {
		if strings.TrimSpace(item.MaterialImageName) != expectedMaterialName {
			return nil
		}
	}
	return renderedImageURLCandidates(item.ImageURLs)
}

func finishedProductMaterialName(result *PrepareSyncDesignResult) string {
	if result == nil || result.Material == nil || result.Material.Material == nil {
		return ""
	}
	name := strings.TrimSpace(result.Material.Material.Name)
	if name == "" {
		return ""
	}
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return strings.TrimSpace(name)
}

func staleRenderedImageURLs(urls []string, result *PrepareSyncDesignResult) bool {
	if len(urls) == 0 || result == nil || result.Page == nil {
		return false
	}
	for _, value := range urls {
		if renderedImageURLUnavailable(value) {
			return true
		}
	}
	initial := uniqueStrings([]string{
		result.Page.Product.ImgURL,
		result.Page.Product.PSDImgURL,
	})
	if len(initial) == 0 {
		return false
	}
	initialSet := make(map[string]struct{}, len(initial))
	for _, value := range initial {
		initialSet[value] = struct{}{}
	}
	if _, ok := initialSet[strings.TrimSpace(urls[0])]; ok {
		return true
	}
	for _, value := range urls {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := initialSet[value]; !ok {
			return false
		}
	}
	return true
}

func renderedImageURLsFromProduct(detail *sdstemplate.ProductDetail, variantID int64) []string {
	if detail == nil {
		return nil
	}
	if detail.Subproducts != nil {
		for i := range detail.Subproducts.Items {
			item := &detail.Subproducts.Items[i]
			if item.ID != variantID {
				continue
			}
			urls := renderedImageURLsFromSummary(item)
			if len(urls) > 0 {
				return urls
			}
		}
	}
	return renderedImageURLsFromSummary(&detail.ProductSummary)
}

func renderedImageURLsFromSummary(product *sdstemplate.ProductSummary) []string {
	if product == nil {
		return nil
	}
	urls := make([]string, 0, 8)
	if product.DesignPrototype != nil {
		groups := append([]sdstemplate.PrototypeResultGroup(nil), product.DesignPrototype.PrototypeResultGroups...)
		sort.SliceStable(groups, func(i, j int) bool {
			return groups[i].Sort < groups[j].Sort
		})
		for _, group := range groups {
			urls = append(urls, group.ResultImage)
		}
	}
	urls = append(urls, product.ImgURL, product.PSDImgURL)
	return renderedImageURLCandidates(urls)
}

func renderedImageURLCandidates(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if renderedImageURLUnavailable(value) {
			continue
		}
		filtered = append(filtered, value)
	}
	return uniqueStrings(filtered)
}

func expectedRenderedImageCount(result *PrepareSyncDesignResult) int {
	if result == nil || result.Page == nil {
		return 0
	}
	count := 0
	for _, psd := range result.Page.PSDs {
		if psdModelFile(psd) != "" {
			count++
		}
	}
	return count
}

func renderedImageURLsReady(urls []string, expectedCount int) bool {
	if len(urls) == 0 {
		return false
	}
	return expectedCount <= 1 || len(urls) >= expectedCount
}

func renderedImageURLsByProductReady(urlsByProduct map[int64][]string, targetIDs []int64, expectedCount int) bool {
	if len(urlsByProduct) == 0 || len(targetIDs) == 0 {
		return false
	}
	for _, id := range targetIDs {
		if !renderedImageURLsReady(urlsByProduct[id], expectedCount) {
			return false
		}
	}
	return true
}

func preferredRenderedImageURLs(current []string, candidate []string) []string {
	if len(candidate) > len(current) {
		return candidate
	}
	return current
}

func preferredRenderedImageURLsByProduct(current map[int64][]string, candidate map[int64][]string) map[int64][]string {
	if len(candidate) == 0 {
		return current
	}
	if current == nil {
		current = map[int64][]string{}
	}
	for productID, urls := range candidate {
		current[productID] = preferredRenderedImageURLs(current[productID], urls)
	}
	return current
}

func renderedImageURLUnavailable(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return true
	}
	return strings.Contains(value, "shengchengzhong") ||
		strings.Contains(value, "/output/generating") ||
		strings.Contains(value, "/output/loading") ||
		strings.Contains(value, "/output/placeholder") ||
		strings.Contains(value, "cdn.sdspod.com/images/")
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
