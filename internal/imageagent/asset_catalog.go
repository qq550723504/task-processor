package imageagent

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

func NormalizeAssetCatalog(catalog AssetCatalog) (AssetCatalog, error) {
	byID := make(map[string]AuthorizedAsset, len(catalog.Assets))
	for _, raw := range catalog.Assets {
		asset := raw
		asset.ID = strings.TrimSpace(asset.ID)
		asset.DisplayURL = safeDisplayURL(asset.DisplayURL)
		asset.Label = strings.TrimSpace(asset.Label)
		if asset.ID == "" {
			return AssetCatalog{}, fmt.Errorf("authorized asset ID is required")
		}
		if asset.Type != AuthorizedAssetSource && asset.Type != AuthorizedAssetStyle {
			return AssetCatalog{}, fmt.Errorf("authorized asset %q has invalid type", asset.ID)
		}
		if existing, duplicate := byID[asset.ID]; duplicate {
			if existing != asset {
				return AssetCatalog{}, fmt.Errorf("authorized asset %q is defined more than once", asset.ID)
			}
			continue
		}
		byID[asset.ID] = asset
	}
	ids := make([]string, 0, len(byID))
	for id := range byID { ids = append(ids, id) }
	sort.Strings(ids)
	normalized := AssetCatalog{Assets: make([]AuthorizedAsset, 0, len(ids))}
	for _, id := range ids { normalized.Assets = append(normalized.Assets, byID[id]) }
	return normalized, nil
}

func ValidatePlanAgainstCatalog(plan Plan, catalog AssetCatalog) error {
	allowedSources := map[string]struct{}{}
	allowedStyles := map[string]struct{}{}
	for _, asset := range catalog.Assets {
		switch asset.Type {
		case AuthorizedAssetSource:
			allowedSources[asset.ID] = struct{}{}
		case AuthorizedAssetStyle:
			allowedStyles[asset.ID] = struct{}{}
		}
	}
	for _, id := range plan.SourceAssetIDs {
		if _, ok := allowedSources[strings.TrimSpace(id)]; !ok { return fmt.Errorf("source asset %q is not authorized", id) }
	}
	for _, id := range plan.StyleReferenceIDs {
		if _, ok := allowedStyles[strings.TrimSpace(id)]; !ok { return fmt.Errorf("style reference %q is not authorized", id) }
	}
	for _, slot := range plan.Slots {
		for _, id := range slot.SourceAssetIDs {
			if _, ok := allowedSources[strings.TrimSpace(id)]; !ok { return fmt.Errorf("slot %q source asset %q is not authorized", slot.ID, id) }
		}
		for _, id := range slot.StyleReferenceIDs {
			if _, ok := allowedStyles[strings.TrimSpace(id)]; !ok { return fmt.Errorf("slot %q style reference %q is not authorized", slot.ID, id) }
		}
	}
	return nil
}

func safeDisplayURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" { return "" }
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" { return "" }
	if parsed.Scheme != "https" && parsed.Scheme != "http" { return "" }
	parsed.User = nil
	return parsed.String()
}
