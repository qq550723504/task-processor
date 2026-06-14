package workspace

import (
	"strconv"
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type FinalReviewSKU struct {
	SupplierCode string  `json:"supplier_code,omitempty"`
	SupplierSKU  string  `json:"supplier_sku,omitempty"`
	Color        string  `json:"color,omitempty"`
	Size         string  `json:"size,omitempty"`
	Price        float64 `json:"price,omitempty"`
	Currency     string  `json:"currency,omitempty"`
	Stock        int     `json:"stock,omitempty"`
	Weight       float64 `json:"weight,omitempty"`
}

type FinalReviewImage struct {
	URL     string `json:"url,omitempty"`
	Role    string `json:"role,omitempty"`
	Sort    int    `json:"sort,omitempty"`
	Final   bool   `json:"final"`
	Main    bool   `json:"main,omitempty"`
	Swatch  bool   `json:"swatch,omitempty"`
	SizeMap bool   `json:"size_map,omitempty"`
}

func BuildPreviewReviewSummary(pkg *sheinpub.Package) (bool, []string) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return false, nil
	}
	needsReview := len(pkg.ReviewNotes) > 0
	summary := uniqueStrings(append([]string(nil), pkg.ReviewNotes...))
	if pkg.Inspection != nil {
		needsReview = needsReview || pkg.Inspection.NeedsReview
		summary = uniqueStrings(append(summary, pkg.Inspection.Summary...))
	}
	return needsReview, summary
}

func BuildFinalReviewSKUs(draft *sheinpub.RequestDraft) []FinalReviewSKU {
	if draft == nil {
		return nil
	}
	out := make([]FinalReviewSKU, 0)
	for _, skc := range draft.SKCList {
		for _, sku := range skc.SKUList {
			out = append(out, BuildFinalReviewSKU(skc.SupplierCode, sku))
		}
	}
	return out
}

func BuildFinalReviewSKU(supplierCode string, sku sheinpub.SKUDraft) FinalReviewSKU {
	item := FinalReviewSKU{
		SupplierCode: supplierCode,
		SupplierSKU:  sku.SupplierSKU,
		Price:        parseMoney(sku.BasePrice),
		Currency:     sku.Currency,
		Stock:        sku.StockCount,
		Weight:       sku.Weight,
	}
	for _, attr := range sku.SaleAttributes {
		switch normalizeFinalReviewAttributeName(attr.Name) {
		case "color":
			item.Color = attr.Value
		case "size":
			item.Size = attr.Value
		}
	}
	return item
}

func BuildFinalReviewImages(draft *sheinpub.RequestDraft, finalDraft *sheinpub.FinalDraft, product *sheinproduct.Product) []FinalReviewImage {
	if draft == nil || draft.ImageInfo == nil {
		return nil
	}
	sizeMapURLs := sheinproduct.CollectSizeMapImageURLs(product)
	out := make([]FinalReviewImage, 0)
	seen := make(map[string]int)
	add := func(url, role string, sort int, main bool) {
		url = strings.TrimSpace(url)
		if url == "" {
			return
		}
		role, main = resolveFinalReviewImageRole(url, role, main, finalDraft, sizeMapURLs)
		if existingIndex, ok := seen[url]; ok {
			mergeFinalReviewImage(&out[existingIndex], role, main)
			return
		}
		seen[url] = len(out)
		out = append(out, FinalReviewImage{
			URL:     url,
			Role:    role,
			Sort:    sort,
			Final:   true,
			Main:    main || role == "main",
			Swatch:  isFinalReviewSwatchRole(role),
			SizeMap: role == "size_map",
		})
	}
	add(draft.ImageInfo.MainImage, "main", 1, true)
	for i, image := range draft.ImageInfo.Gallery {
		add(image, "gallery", i+2, false)
	}
	if draft.ImageInfo.WhiteBg != "" {
		add(draft.ImageInfo.WhiteBg, "white_bg", len(out)+1, false)
	}
	for _, skc := range draft.SKCList {
		if skc.ImageInfo != nil {
			add(skc.ImageInfo.MainImage, "skc", len(out)+1, false)
		}
	}
	return out
}

func resolveFinalReviewImageRole(url, role string, main bool, finalDraft *sheinpub.FinalDraft, sizeMapURLs map[string]struct{}) (string, bool) {
	if finalDraft != nil {
		if override := strings.TrimSpace(finalDraft.ImageRoleOverrides[url]); override != "" {
			role = override
		}
		if strings.TrimSpace(finalDraft.MainImageURL) == url && role != "skc" && role != "swatch" && role != "size_map" {
			main = true
			role = "main"
		}
	}
	if _, ok := sizeMapURLs[url]; ok && role == "gallery" {
		role = "size_map"
	}
	return role, main
}

func isFinalReviewSwatchRole(role string) bool {
	return role == "swatch" || role == "skc"
}

func mergeFinalReviewImage(existing *FinalReviewImage, role string, main bool) {
	if existing == nil {
		return
	}
	switch {
	case main || role == "main":
		existing.Role = "main"
		existing.Main = true
		existing.SizeMap = false
		existing.Swatch = false
	case role == "size_map" && existing.Role != "main":
		existing.Role = "size_map"
		existing.SizeMap = true
		existing.Swatch = false
	case isFinalReviewSwatchRole(role) && existing.Role != "main" && existing.Role != "size_map":
		existing.Role = role
		existing.Swatch = true
	}
}

func normalizeFinalReviewAttributeName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "color", "颜色":
		return "color"
	case "size", "尺码", "尺寸":
		return "size"
	default:
		return ""
	}
}

func parseMoney(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}
