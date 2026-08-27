package imageagent

import (
	"crypto/sha256"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"sort"
	"strings"

	"task-processor/internal/pkg/safeimagehttp"
)

func NormalizeAssetCatalog(catalog AssetCatalog) (AssetCatalog, error) {
	byID := make(map[string]AuthorizedAsset, len(catalog.Assets))
	for _, raw := range catalog.Assets {
		asset := raw
		asset.ID = strings.TrimSpace(asset.ID)
		asset.URL = strings.TrimSpace(asset.URL)
		asset.SourceURL = strings.TrimSpace(asset.SourceURL)
		asset.DisplayURL = strings.TrimSpace(asset.DisplayURL)
		asset.Label = strings.TrimSpace(asset.Label)
		if asset.ID == "" {
			return AssetCatalog{}, fmt.Errorf("authorized asset ID is required")
		}
		if asset.Type != AuthorizedAssetSource && asset.Type != AuthorizedAssetStyle {
			return AssetCatalog{}, fmt.Errorf("authorized asset %q has invalid type", asset.ID)
		}
		if asset.URL == "" {
			asset.URL = asset.DisplayURL
		}
		if asset.SourceURL == "" {
			asset.SourceURL = asset.URL
		}
		for name, rawURL := range map[string]string{"url": asset.URL, "source_url": asset.SourceURL, "display_url": asset.DisplayURL} {
			if rawURL == "" {
				continue
			}
			validated, err := ValidateSafeImageURL(rawURL)
			if err != nil {
				return AssetCatalog{}, fmt.Errorf("authorized asset %q %s is unsafe: %w", asset.ID, name, err)
			}
			switch name {
			case "url":
				asset.URL = validated
			case "source_url":
				asset.SourceURL = validated
			case "display_url":
				asset.DisplayURL = validated
			}
		}
		asset.Metadata = cloneStringMap(asset.Metadata)
		if existing, duplicate := byID[asset.ID]; duplicate {
			if !reflect.DeepEqual(existing, asset) {
				return AssetCatalog{}, fmt.Errorf("authorized asset %q is defined more than once", asset.ID)
			}
			continue
		}
		byID[asset.ID] = asset
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	normalized := AssetCatalog{Manifest: catalog.Manifest, Assets: make([]AuthorizedAsset, 0, len(ids))}
	for _, id := range ids {
		normalized.Assets = append(normalized.Assets, byID[id])
	}
	if normalized.Manifest.Version <= 0 {
		normalized.Manifest.Version = 1
	}
	computedHash := CatalogHash(normalized.Assets)
	if normalized.Manifest.Hash != "" && normalized.Manifest.Hash != computedHash {
		return AssetCatalog{}, fmt.Errorf("authorized asset catalog manifest hash does not match canonical assets")
	}
	normalized.Manifest.Hash = computedHash
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
		if _, ok := allowedSources[strings.TrimSpace(id)]; !ok {
			return fmt.Errorf("source asset %q is not authorized", id)
		}
	}
	for _, id := range plan.StyleReferenceIDs {
		if _, ok := allowedStyles[strings.TrimSpace(id)]; !ok {
			return fmt.Errorf("style reference %q is not authorized", id)
		}
	}
	for _, slot := range plan.Slots {
		for _, id := range slot.SourceAssetIDs {
			if _, ok := allowedSources[strings.TrimSpace(id)]; !ok {
				return fmt.Errorf("slot %q source asset %q is not authorized", slot.ID, id)
			}
		}
		for _, id := range slot.StyleReferenceIDs {
			if _, ok := allowedStyles[strings.TrimSpace(id)]; !ok {
				return fmt.Errorf("slot %q style reference %q is not authorized", slot.ID, id)
			}
		}
	}
	return nil
}

func ValidateSafeImageURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("http(s) image URL is required")
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed == nil || !parsed.IsAbs() || parsed.Host == "" {
		return "", fmt.Errorf("absolute http(s) image URL is required")
	}
	if !strings.EqualFold(parsed.Scheme, "https") && !strings.EqualFold(parsed.Scheme, "http") {
		return "", fmt.Errorf("http(s) image URL is required")
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" || strings.EqualFold(host, "localhost") {
		return "", fmt.Errorf("public image host is required")
	}
	if ip := net.ParseIP(host); ip != nil && safeimagehttp.IsPrivateIP(ip) {
		return "", fmt.Errorf("public image host is required")
	}
	parsed.User = nil
	return parsed.String(), nil
}

func CatalogHash(assets []AuthorizedAsset) string {
	// NormalizeAssetCatalog sorts by ID; JSON-like stable concatenation avoids
	// leaking map iteration order into the immutable manifest identity.
	var builder strings.Builder
	for _, asset := range assets {
		fmt.Fprintf(&builder, "%s\x00%s\x00%s\x00%s\x00%s\x00%d\x00%d\n", asset.ID, asset.Type, asset.URL, asset.SourceURL, asset.DisplayURL, asset.Width, asset.Height)
		keys := make([]string, 0, len(asset.Metadata))
		for key := range asset.Metadata {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&builder, "%s=%s\n", key, asset.Metadata[key])
		}
	}
	return fmt.Sprintf("catalog-v1:%x", sha256Sum([]byte(builder.String())))
}

func sha256Sum(value []byte) [32]byte { return sha256.Sum256(value) }

func cloneStringMap(value map[string]string) map[string]string {
	if value == nil {
		return nil
	}
	cloned := make(map[string]string, len(value))
	for key, item := range value {
		cloned[strings.TrimSpace(key)] = strings.TrimSpace(item)
	}
	return cloned
}
