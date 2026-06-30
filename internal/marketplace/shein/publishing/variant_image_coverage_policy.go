package publishing

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

// VariantImageCoverageState provides coverage evidence needed for variant image blocking.
type VariantImageCoverageState struct {
	RequiredGroupCount          int
	DistinctImageCount          int
	AvailableVariantImageGroups int
	SDSError                    string
}

// VariantImageGroupInput is the neutral SKC shape needed for coverage grouping.
type VariantImageGroupInput struct {
	SKUColorCandidates []string
	SKCCandidates      []string
}

// VariantImageMainImageInput is the neutral SKC shape needed for distinct image counting.
type VariantImageMainImageInput struct {
	SKCMainImage string
	SKUMainImage []string
}

// EnforceVariantImageCoverage checks whether SKC images have enough variant coverage.
func EnforceVariantImageCoverage(state VariantImageCoverageState) (string, bool) {
	if state.RequiredGroupCount <= 1 {
		return "", false
	}
	if state.DistinctImageCount >= state.RequiredGroupCount {
		return "", false
	}
	if state.AvailableVariantImageGroups >= state.RequiredGroupCount {
		return "", false
	}
	warning := variantImageCoverageBlockedMessage
	if sdsErr := strings.TrimSpace(state.SDSError); sdsErr != "" {
		warning = warning + "；" + sdsErr
	}
	return warning, true
}

// VariantImageGroupCount returns the number of distinct SKC image groups required by a package.
func VariantImageGroupCount(keys []string) int {
	groups := map[string]struct{}{}
	unnamed := 0
	for _, key := range keys {
		if key = NormalizeVariantImageKey(key); key != "" {
			groups[key] = struct{}{}
			continue
		}
		unnamed++
	}
	return len(groups) + unnamed
}

// VariantImageGroupKey returns the stable group key for an SKC's variant image coverage.
func VariantImageGroupKey(input VariantImageGroupInput) string {
	for _, candidate := range input.SKUColorCandidates {
		if key := NormalizeVariantImageKey(candidate); key != "" {
			return key
		}
	}
	for _, candidate := range input.SKCCandidates {
		if key := NormalizeVariantImageKey(candidate); key != "" {
			return key
		}
	}
	return ""
}

// DistinctVariantImageMainImageCount returns the number of distinct assigned main images.
func DistinctVariantImageMainImageCount(urls []string) int {
	seen := map[string]struct{}{}
	for _, url := range urls {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}
		seen[url] = struct{}{}
	}
	return len(seen)
}

// VariantImageMainImageURL returns an SKC's main image, falling back to SKU main images.
func VariantImageMainImageURL(input VariantImageMainImageInput) string {
	if value := strings.TrimSpace(input.SKCMainImage); value != "" {
		return value
	}
	for _, value := range input.SKUMainImage {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

// SetVariantImageCoverageMetadata writes or clears variant image coverage metadata.
func SetVariantImageCoverageMetadata(metadata map[string]string, warning string, blocked bool) map[string]string {
	if metadata == nil {
		if !blocked {
			return nil
		}
		metadata = map[string]string{}
	}
	if blocked {
		metadata[VariantImageCoverageStatusKey] = variantImageCoverageStatusBlocked
		metadata[VariantImageCoverageMessageKey] = strings.TrimSpace(warning)
		return metadata
	}
	delete(metadata, VariantImageCoverageStatusKey)
	delete(metadata, VariantImageCoverageMessageKey)
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

// VariantImageCoverageStatus returns the current blocked variant image coverage message.
func VariantImageCoverageStatus(metadata map[string]string) (string, bool) {
	if metadata == nil {
		return "", false
	}
	if strings.TrimSpace(metadata[VariantImageCoverageStatusKey]) != variantImageCoverageStatusBlocked {
		return "", false
	}
	message := strings.TrimSpace(metadata[VariantImageCoverageMessageKey])
	if message == "" {
		message = "变体图片覆盖不完整，请为每个颜色规格补齐独立商品图后再提交"
	}
	return message, true
}
