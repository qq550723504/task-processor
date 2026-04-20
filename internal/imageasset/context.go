package imageasset

import (
	"sort"
	"strings"

	"task-processor/internal/asset"
	"task-processor/internal/catalog"
)

// Context represents a product-oriented image asset view assembled from
// canonical catalog facts and asset inventory records.
type Context struct {
	Product ProductFacts `json:"product"`
	Sources []SourceRef  `json:"sources,omitempty"`
	Assets  []AssetRef   `json:"assets,omitempty"`
	Main    *AssetRef    `json:"main,omitempty"`
}

// ProductFacts holds lightweight product identity fields used by image flows.
type ProductFacts struct {
	Title        string   `json:"title,omitempty"`
	Brand        string   `json:"brand,omitempty"`
	CategoryPath []string `json:"category_path,omitempty"`
}

// SourceRef describes a source image discovered from catalog facts.
type SourceRef struct {
	URL  string `json:"url,omitempty"`
	Role string `json:"role,omitempty"`
}

// AssetRef is a UI/service friendly representation of one inventory record.
type AssetRef struct {
	ID       string            `json:"id,omitempty"`
	Kind     asset.Kind        `json:"kind,omitempty"`
	Origin   asset.Origin      `json:"origin,omitempty"`
	URL      string            `json:"url,omitempty"`
	Role     string            `json:"role,omitempty"`
	Selected bool              `json:"selected"`
	Main     bool              `json:"main"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// BuildContext assembles a product image context from catalog and asset layers.
func BuildContext(product *catalog.Product, inventory *asset.Inventory) Context {
	ctx := Context{}
	if product != nil {
		ctx.Product = ProductFacts{
			Title:        strings.TrimSpace(product.Title),
			Brand:        strings.TrimSpace(product.Brand),
			CategoryPath: append([]string(nil), product.CategoryPath...),
		}
		ctx.Sources = collectSources(product)
	}
	if inventory == nil {
		return ctx
	}

	selected := map[string]struct{}{}
	mainID := ""
	if inventory.Summary != nil {
		mainID = strings.TrimSpace(inventory.Summary.MainAssetID)
		for _, id := range inventory.Summary.SelectedAssetIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			selected[id] = struct{}{}
		}
	}

	for _, record := range inventory.Records {
		id := strings.TrimSpace(record.ID)
		_, isSelected := selected[id]
		isMain := mainID != "" && id == mainID
		ref := AssetRef{
			ID:       id,
			Kind:     record.Kind,
			Origin:   record.Origin,
			URL:      strings.TrimSpace(record.URL),
			Role:     strings.TrimSpace(record.Role),
			Selected: isSelected,
			Main:     isMain,
			Metadata: cloneMetadata(record.Metadata),
		}
		if ctx.Main == nil && isMain {
			mainCopy := ref
			ctx.Main = &mainCopy
		}
		ctx.Assets = append(ctx.Assets, ref)
	}

	if ctx.Main == nil {
		for i := range ctx.Assets {
			if ctx.Assets[i].Kind == asset.KindMainImage {
				ctx.Assets[i].Main = true
				mainCopy := ctx.Assets[i]
				ctx.Main = &mainCopy
				break
			}
		}
	}

	return ctx
}

func collectSources(product *catalog.Product) []SourceRef {
	seen := map[string]struct{}{}
	result := make([]SourceRef, 0, len(product.Images))

	appendSource := func(url string, role string) {
		url = strings.TrimSpace(url)
		if url == "" {
			return
		}
		if _, ok := seen[url]; ok {
			return
		}
		seen[url] = struct{}{}
		result = append(result, SourceRef{URL: url, Role: strings.TrimSpace(role)})
	}

	for _, img := range product.Images {
		appendSource(img.URL, img.Role)
	}
	for _, variant := range product.Variants {
		for _, img := range variant.Images {
			appendSource(img.URL, img.Role)
		}
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Role != result[j].Role {
			return result[i].Role < result[j].Role
		}
		return result[i].URL < result[j].URL
	})
	return result
}

func cloneMetadata(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}
