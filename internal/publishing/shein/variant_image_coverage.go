package shein

import "strings"

const (
	// VariantImageCoverageStatusKey stores the variant image coverage status in package metadata.
	VariantImageCoverageStatusKey = "variant_image_coverage_status"
	// VariantImageCoverageMessageKey stores the variant image coverage warning in package metadata.
	VariantImageCoverageMessageKey = "variant_image_coverage_message"
)

const (
	variantImageCoverageBlockedMessage = "变体图片覆盖不完整：当前颜色规格多于可用变体图，已阻止将同一张图复用到所有 SKC，请补齐每个颜色的商品图后再提交"
	variantImageCoverageStatusBlocked  = "blocked"
)

// VariantImageCoverageInput provides external coverage evidence collected outside the publishing package.
type VariantImageCoverageInput struct {
	AvailableVariantImageGroups int
	SDSError                    string
}

// EnforceVariantImageCoverage checks whether SKC images have enough variant coverage.
func EnforceVariantImageCoverage(pkg *Package, input VariantImageCoverageInput) (string, bool) {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return "", false
	}
	requiredGroupCount := VariantImageGroupCount(pkg)
	if requiredGroupCount <= 1 {
		return "", false
	}
	distinctImageCount := DistinctSKCMainImageCount(pkg)
	if distinctImageCount >= requiredGroupCount {
		return "", false
	}
	if input.AvailableVariantImageGroups >= requiredGroupCount {
		return "", false
	}
	warning := variantImageCoverageBlockedMessage
	if sdsErr := strings.TrimSpace(input.SDSError); sdsErr != "" {
		warning = warning + "；" + sdsErr
	}
	return warning, true
}

// VariantImageGroupCount returns the number of distinct SKC image groups required by a package.
func VariantImageGroupCount(pkg *Package) int {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return 0
	}
	groups := map[string]struct{}{}
	unnamed := 0
	for _, skc := range pkg.DraftPayload.SKCList {
		if key := VariantImageGroupKey(skc); key != "" {
			groups[key] = struct{}{}
			continue
		}
		unnamed++
	}
	return len(groups) + unnamed
}

// VariantImageGroupKey returns the stable group key for an SKC's variant image coverage.
func VariantImageGroupKey(skc SKCRequestDraft) string {
	for _, sku := range skc.SKUList {
		for _, candidate := range []string{
			sku.Attributes["Color"],
			sku.Attributes["color"],
			sku.Attributes["variant_color"],
		} {
			if key := NormalizeVariantImageKey(candidate); key != "" {
				return key
			}
		}
	}
	for _, candidate := range []string{
		skc.SaleName,
		skc.SkcName,
		ResolvedSaleAttributeValue(skc.SaleAttribute),
	} {
		if key := NormalizeVariantImageKey(candidate); key != "" {
			return key
		}
	}
	return ""
}

// SetVariantImageCoverageMetadata writes or clears variant image coverage metadata.
func SetVariantImageCoverageMetadata(pkg *Package, warning string, blocked bool) {
	if pkg == nil {
		return
	}
	if pkg.Metadata == nil {
		if !blocked {
			return
		}
		pkg.Metadata = map[string]string{}
	}
	if blocked {
		pkg.Metadata[VariantImageCoverageStatusKey] = variantImageCoverageStatusBlocked
		pkg.Metadata[VariantImageCoverageMessageKey] = strings.TrimSpace(warning)
		return
	}
	delete(pkg.Metadata, VariantImageCoverageStatusKey)
	delete(pkg.Metadata, VariantImageCoverageMessageKey)
	if len(pkg.Metadata) == 0 {
		pkg.Metadata = nil
	}
}

// VariantImageCoverageStatus returns the current blocked variant image coverage message.
func VariantImageCoverageStatus(pkg *Package) (string, bool) {
	if pkg == nil || pkg.Metadata == nil {
		return "", false
	}
	if strings.TrimSpace(pkg.Metadata[VariantImageCoverageStatusKey]) != variantImageCoverageStatusBlocked {
		return "", false
	}
	message := strings.TrimSpace(pkg.Metadata[VariantImageCoverageMessageKey])
	if message == "" {
		message = "变体图片覆盖不完整，请为每个颜色规格补齐独立商品图后再提交"
	}
	return message, true
}

// DistinctSKCMainImageCount returns the number of distinct main images currently assigned to SKCs.
func DistinctSKCMainImageCount(pkg *Package) int {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return 0
	}
	seen := map[string]struct{}{}
	for _, skc := range pkg.DraftPayload.SKCList {
		url := strings.TrimSpace(SKCMainImageURL(skc))
		if url == "" {
			continue
		}
		seen[url] = struct{}{}
	}
	return len(seen)
}

// SKCMainImageURL returns an SKC's main image, falling back to SKU main images.
func SKCMainImageURL(skc SKCRequestDraft) string {
	if skc.ImageInfo != nil && strings.TrimSpace(skc.ImageInfo.MainImage) != "" {
		return strings.TrimSpace(skc.ImageInfo.MainImage)
	}
	for _, sku := range skc.SKUList {
		if strings.TrimSpace(sku.MainImage) != "" {
			return strings.TrimSpace(sku.MainImage)
		}
	}
	return ""
}
