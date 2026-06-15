package listingkit

import (
	"strings"

	"task-processor/internal/asset"
	common "task-processor/internal/publishing/common"
)

func buildAssetURLLookup(bundle *asset.Bundle) map[string]string {
	if bundle == nil || len(bundle.Assets) == 0 {
		return nil
	}
	lookup := make(map[string]string, len(bundle.Assets)*4)
	for _, item := range bundle.Assets {
		publishedURL := preferredAssetURL(item)
		if publishedURL == "" {
			continue
		}
		indexAssetURLLookup(lookup, item.ID, publishedURL)
		indexAssetURLLookup(lookup, item.URL, publishedURL)
		if item.Metadata != nil {
			indexAssetURLLookup(lookup, item.Metadata["published_url"], publishedURL)
			indexAssetURLLookup(lookup, item.Metadata["published_path"], publishedURL)
			indexAssetURLLookup(lookup, item.Metadata["local_path"], publishedURL)
		}
	}
	if len(lookup) == 0 {
		return nil
	}
	return lookup
}

func indexAssetURLLookup(lookup map[string]string, key, value string) {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return
	}
	if existing := strings.TrimSpace(lookup[key]); existing != "" && !shouldReplaceAssetURL(existing, value) {
		return
	}
	lookup[key] = value
}

func publishedAssetURLForBundleSlot(slot *common.BundleSlot, assetURLByID map[string]string, assetBundle *asset.Bundle) string {
	if slot == nil || len(assetURLByID) == 0 {
		if slot == nil {
			return ""
		}
	}
	fallbackURL := ""
	candidates := []string{slot.URL, slot.RecipeID}
	for _, candidate := range candidates {
		if url := assetURLByID[strings.TrimSpace(candidate)]; url != "" {
			if isPublishedAssetURL(url) {
				return url
			}
			if fallbackURL == "" {
				fallbackURL = url
			}
		}
	}
	if assetBundle == nil {
		return fallbackURL
	}
	for _, item := range assetBundle.Assets {
		url := preferredAssetURL(item)
		if !isPublishedAssetURL(url) {
			continue
		}
		if matchesPublishedBundleAssetForSlot(item, slot) {
			return url
		}
	}
	return fallbackURL
}

func matchesPublishedBundleAssetForSlot(item asset.Asset, slot *common.BundleSlot) bool {
	if slot == nil {
		return false
	}
	if strings.TrimSpace(item.RecipeID) != "" && strings.EqualFold(strings.TrimSpace(item.RecipeID), strings.TrimSpace(slot.RecipeID)) {
		return true
	}
	if !shareSourceAssetID(item.SourceAssetIDs, slot.SourceAssetIDs) {
		return false
	}
	if hasLabel(item.Labels, slot.Key) || strings.EqualFold(strings.TrimSpace(item.Role), strings.TrimSpace(slot.Purpose)) || strings.EqualFold(strings.TrimSpace(item.Role), strings.TrimSpace(slot.Key)) {
		return true
	}
	return false
}

func shareSourceAssetID(left, right []string) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(left))
	for _, item := range left {
		key := strings.TrimSpace(strings.ToLower(item))
		if key == "" {
			continue
		}
		seen[key] = struct{}{}
	}
	for _, item := range right {
		key := strings.TrimSpace(strings.ToLower(item))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			return true
		}
	}
	return false
}

func hasLabel(labels []string, target string) bool {
	target = strings.TrimSpace(strings.ToLower(target))
	if target == "" {
		return false
	}
	for _, label := range labels {
		if strings.TrimSpace(strings.ToLower(label)) == target {
			return true
		}
	}
	return false
}

func preferredAssetURL(item asset.Asset) string {
	if item.Metadata != nil {
		if published := strings.TrimSpace(item.Metadata["published_url"]); published != "" {
			return published
		}
	}
	return strings.TrimSpace(item.URL)
}

func shouldReplaceAssetURL(existing, candidate string) bool {
	existingRemote := isPublishedAssetURL(existing)
	candidateRemote := isPublishedAssetURL(candidate)
	switch {
	case existingRemote && !candidateRemote:
		return false
	case !existingRemote && candidateRemote:
		return true
	default:
		return existing == ""
	}
}

func isPublishedAssetURL(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}
