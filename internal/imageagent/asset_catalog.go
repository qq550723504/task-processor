package imageagent

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"

	"task-processor/internal/integration/httpimage"
)

func NormalizeAssetCatalog(catalog AssetCatalog) (AssetCatalog, error) {
	productContext, err := NormalizeProductContextRef(catalog.ProductContext)
	if err != nil {
		return AssetCatalog{}, err
	}
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
		if err := ValidateProvenanceAssetID(asset.ID); err != nil {
			return AssetCatalog{}, fmt.Errorf("%w: authorized asset ID is invalid", err)
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
	normalized := AssetCatalog{Manifest: catalog.Manifest, Assets: make([]AuthorizedAsset, 0, len(ids)), ProductContext: productContext}
	for _, id := range ids {
		normalized.Assets = append(normalized.Assets, byID[id])
	}
	if normalized.Manifest.Version <= 0 {
		normalized.Manifest.Version = 1
	}
	computedHash := CatalogSnapshotHash(normalized.Assets, normalized.ProductContext)
	if normalized.Manifest.Hash != "" && normalized.Manifest.Hash != computedHash {
		return AssetCatalog{}, fmt.Errorf("authorized asset catalog manifest hash does not match canonical assets")
	}
	normalized.Manifest.Hash = computedHash
	return normalized, nil
}

// NormalizeProductContextRef canonicalizes the immutable, owner-verified
// product context that is safe to pass to provider adapters. A completely
// empty value remains valid for historical catalog snapshots.
func NormalizeProductContextRef(value ProductContextRef) (ProductContextRef, error) {
	value.ProductID = strings.TrimSpace(value.ProductID)
	value.Title = strings.TrimSpace(value.Title)
	value.ProductType = strings.TrimSpace(value.ProductType)
	normalizedAttributes := make(map[string]string, len(value.Attributes))
	for rawKey, rawItem := range value.Attributes {
		key, item := strings.TrimSpace(rawKey), strings.TrimSpace(rawItem)
		if key == "" || item == "" {
			continue
		}
		if existing, duplicate := normalizedAttributes[key]; duplicate && existing != item {
			return ProductContextRef{}, fmt.Errorf("product context attribute %q is defined more than once", key)
		}
		normalizedAttributes[key] = item
	}
	value.Attributes = normalizedAttributes
	if len(normalizedAttributes) == 0 {
		value.Attributes = nil
	}
	if ProductContextRefIsZero(value) {
		return ProductContextRef{}, nil
	}
	if value.ProductID == "" {
		return ProductContextRef{}, fmt.Errorf("product context ID is required")
	}
	return value, nil
}

func ProductContextRefIsZero(value ProductContextRef) bool {
	return strings.TrimSpace(value.ProductID) == "" && strings.TrimSpace(value.Title) == "" && strings.TrimSpace(value.ProductType) == "" && len(value.Attributes) == 0
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

// ValidateSubmittedPlanAgainstCatalog adds ingress-only execution prerequisites
// without changing replay validation for historical workflow snapshots.
func ValidateSubmittedPlanAgainstCatalog(plan Plan, catalog AssetCatalog) error {
	if err := ValidatePlanAgainstCatalog(plan, catalog); err != nil {
		return err
	}
	allowedSources := make(map[string]AuthorizedAsset, len(catalog.Assets))
	for _, asset := range catalog.Assets {
		if asset.Type == AuthorizedAssetSource {
			allowedSources[strings.TrimSpace(asset.ID)] = asset
		}
	}
	for _, slot := range plan.Slots {
		if slot.Role != SlotRoleSize {
			continue
		}
		for _, rawID := range slot.SourceAssetIDs {
			id := strings.TrimSpace(rawID)
			if id == "" {
				continue
			}
			source := allowedSources[id]
			if source.Width <= 0 || source.Height <= 0 {
				return fmt.Errorf("size slot %q requires first source asset %q to have reliable dimensions", slot.ID, id)
			}
			break
		}
	}
	return nil
}

func ValidateSafeImageURL(raw string) (string, error) {
	validated, err := httpimage.ValidatePublicHTTPSURL(raw)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(validated)
	if err != nil {
		return "", err
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

// CatalogSnapshotHash extends the legacy asset-only manifest identity when a
// product context snapshot is present. Empty historical contexts deliberately
// retain catalog-v1 hashes for replay and persisted-row compatibility.
func CatalogSnapshotHash(assets []AuthorizedAsset, productContext ProductContextRef) string {
	if ProductContextRefIsZero(productContext) {
		return CatalogHash(assets)
	}
	encoded, _ := json.Marshal(struct {
		AssetsHash     string
		ProductContext ProductContextRef
	}{AssetsHash: CatalogHash(assets), ProductContext: productContext})
	return fmt.Sprintf("catalog-v2:%x", sha256Sum(encoded))
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
