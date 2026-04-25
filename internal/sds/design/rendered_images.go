package design

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"time"

	sdstemplate "task-processor/internal/sds/template"
)

func (s *Service) fetchRenderedImageURLs(ctx context.Context, input PrepareSyncDesignInput, result *PrepareSyncDesignResult) []string {
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

	var variantID int64
	if result != nil && result.Page != nil {
		variantID = result.Page.Product.ID
	}
	if variantID <= 0 {
		variantID = input.VariantID
	}

	expectedCount := expectedRenderedImageCount(result)
	var bestURLs []string
	for attempt := 0; attempt < 8; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return bestURLs
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
		if urls := s.fetchFinishedProductImageURLs(ctx, input, result, variantID, parentProductID); len(urls) > 0 {
			bestURLs = preferredRenderedImageURLs(bestURLs, urls)
			if renderedImageURLsReady(urls, expectedCount) {
				return urls
			}
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
			bestURLs = preferredRenderedImageURLs(bestURLs, urls)
			if renderedImageURLsReady(urls, expectedCount) {
				return urls
			}
		}
	}
	return bestURLs
}

func (s *Service) fetchFinishedProductImageURLs(ctx context.Context, input PrepareSyncDesignInput, result *PrepareSyncDesignResult, variantID int64, parentProductID int64) []string {
	if variantID <= 0 {
		return nil
	}
	list, err := s.ListDesignProducts(ctx, ListDesignProductsRequest{
		ProductID:       variantID,
		ParentProductID: parentProductID,
		DesignType:      input.DesignType,
		Page:            1,
		Size:            10,
	})
	if err != nil || list == nil || len(list.Items) == 0 {
		return nil
	}

	expectedMaterialName := finishedProductMaterialName(result)
	return selectFinishedProductImageURLs(list.Items, variantID, expectedMaterialName)
}

func selectFinishedProductImageURLs(items []DesignProductListItem, variantID int64, expectedMaterialName string) []string {
	candidates := append([]DesignProductListItem(nil), items...)
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].FinishTime > candidates[j].FinishTime
	})
	if expectedMaterialName != "" {
		for _, item := range candidates {
			if urls := finishedProductItemImageURLs(item, variantID, expectedMaterialName); len(urls) > 0 {
				return urls
			}
		}
		return nil
	}
	for _, item := range candidates {
		if urls := finishedProductItemImageURLs(item, variantID, ""); len(urls) > 0 {
			return urls
		}
	}
	return nil
}

func finishedProductItemImageURLs(item DesignProductListItem, variantID int64, expectedMaterialName string) []string {
	if item.ProductID != 0 && item.ProductID != variantID {
		return nil
	}
	if !item.BuildFinish || len(item.ImageURLs) == 0 {
		return nil
	}
	if expectedMaterialName != "" && strings.TrimSpace(item.MaterialImageName) != "" && strings.TrimSpace(item.MaterialImageName) != expectedMaterialName {
		return nil
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

func preferredRenderedImageURLs(current []string, candidate []string) []string {
	if len(candidate) > len(current) {
		return candidate
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
