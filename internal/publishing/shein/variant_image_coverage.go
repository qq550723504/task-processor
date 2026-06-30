package shein

import sheinmarketpub "task-processor/internal/marketplace/shein/publishing"

const (
	// VariantImageCoverageStatusKey stores the variant image coverage status in package metadata.
	VariantImageCoverageStatusKey = sheinmarketpub.VariantImageCoverageStatusKey
	// VariantImageCoverageMessageKey stores the variant image coverage warning in package metadata.
	VariantImageCoverageMessageKey = sheinmarketpub.VariantImageCoverageMessageKey
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
	return sheinmarketpub.EnforceVariantImageCoverage(sheinmarketpub.VariantImageCoverageState{
		RequiredGroupCount:          VariantImageGroupCount(pkg),
		DistinctImageCount:          DistinctSKCMainImageCount(pkg),
		AvailableVariantImageGroups: input.AvailableVariantImageGroups,
		SDSError:                    input.SDSError,
	})
}

// VariantImageGroupCount returns the number of distinct SKC image groups required by a package.
func VariantImageGroupCount(pkg *Package) int {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return 0
	}
	keys := make([]string, 0, len(pkg.DraftPayload.SKCList))
	for _, skc := range pkg.DraftPayload.SKCList {
		keys = append(keys, VariantImageGroupKey(skc))
	}
	return sheinmarketpub.VariantImageGroupCount(keys)
}

// VariantImageGroupKey returns the stable group key for an SKC's variant image coverage.
func VariantImageGroupKey(skc SKCRequestDraft) string {
	input := sheinmarketpub.VariantImageGroupInput{
		SKCCandidates: []string{
			skc.SaleName,
			skc.SkcName,
			ResolvedSaleAttributeValue(skc.SaleAttribute),
		},
	}
	for _, sku := range skc.SKUList {
		input.SKUColorCandidates = append(input.SKUColorCandidates,
			sku.Attributes["Color"],
			sku.Attributes["color"],
			sku.Attributes["variant_color"],
		)
	}
	return sheinmarketpub.VariantImageGroupKey(input)
}

// SetVariantImageCoverageMetadata writes or clears variant image coverage metadata.
func SetVariantImageCoverageMetadata(pkg *Package, warning string, blocked bool) {
	if pkg == nil {
		return
	}
	pkg.Metadata = sheinmarketpub.SetVariantImageCoverageMetadata(pkg.Metadata, warning, blocked)
}

// VariantImageCoverageStatus returns the current blocked variant image coverage message.
func VariantImageCoverageStatus(pkg *Package) (string, bool) {
	if pkg == nil {
		return "", false
	}
	return sheinmarketpub.VariantImageCoverageStatus(pkg.Metadata)
}

// DistinctSKCMainImageCount returns the number of distinct main images currently assigned to SKCs.
func DistinctSKCMainImageCount(pkg *Package) int {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return 0
	}
	urls := make([]string, 0, len(pkg.DraftPayload.SKCList))
	for _, skc := range pkg.DraftPayload.SKCList {
		urls = append(urls, SKCMainImageURL(skc))
	}
	return sheinmarketpub.DistinctVariantImageMainImageCount(urls)
}

// SKCMainImageURL returns an SKC's main image, falling back to SKU main images.
func SKCMainImageURL(skc SKCRequestDraft) string {
	input := sheinmarketpub.VariantImageMainImageInput{}
	if skc.ImageInfo != nil {
		input.SKCMainImage = skc.ImageInfo.MainImage
	}
	for _, sku := range skc.SKUList {
		input.SKUMainImage = append(input.SKUMainImage, sku.MainImage)
	}
	return sheinmarketpub.VariantImageMainImageURL(input)
}
