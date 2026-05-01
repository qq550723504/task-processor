package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

type sheinStudioSupplierSKURename struct {
	Old string
	New string
}

func normalizeSheinStudioSubmitSupplierSKUs(task *Task, pkg *sheinpub.Package) bool {
	if task == nil || task.Request == nil || task.Request.Options == nil || pkg == nil || pkg.RequestDraft == nil {
		return false
	}
	sds := task.Request.Options.SDS
	if sds == nil {
		return false
	}
	taskDiscriminator := studioSubmitTaskDiscriminator(task.ID)
	styleID := firstNonEmptyString(
		sheinStudioStyleID(task.Request.Options.SheinStudio),
		sds.StyleID,
	)
	if styleID == "" && strings.TrimSpace(sds.ProductSKU) == "" && strings.TrimSpace(sds.VariantSKU) == "" && len(sds.Variants) == 0 {
		return false
	}

	seen := map[string]int{}
	renames := make([]sheinStudioSupplierSKURename, 0)
	changed := false
	globalIndex := 0

	for skcIndex := range pkg.RequestDraft.SKCList {
		draftSKC := &pkg.RequestDraft.SKCList[skcIndex]
		for skuIndex := range draftSKC.SKUList {
			draftSKU := &draftSKC.SKUList[skuIndex]
			oldSKU := strings.TrimSpace(draftSKU.SupplierSKU)
			match, matchedIndex := matchStudioSubmitVariantOption(sds, draftSKC, draftSKU, globalIndex)
			baseSKU := resolveStudioSubmitBaseSKU(sds, draftSKU, match, oldSKU)
			requireVariantDiscriminator := studioSubmitRequiresVariantDiscriminator(sds, baseSKU) || taskDiscriminator != ""
			discriminator := resolveStudioSubmitVariantDiscriminator(sds, draftSKU, match, matchedIndex, globalIndex, taskDiscriminator)
			newSKU := buildStudioVariantSKU(baseSKU, styleID, discriminator, requireVariantDiscriminator, seen)
			if strings.TrimSpace(newSKU) == "" {
				globalIndex++
				continue
			}
			if oldSKU == "" || !strings.EqualFold(oldSKU, newSKU) {
				draftSKU.SupplierSKU = newSKU
				changed = true
			}
			renames = append(renames, sheinStudioSupplierSKURename{Old: oldSKU, New: newSKU})

			if skcIndex < len(pkg.SkcList) && skuIndex < len(pkg.SkcList[skcIndex].SKUs) {
				pkg.SkcList[skcIndex].SKUs[skuIndex].SKU = newSKU
			}
			if pkg.PreviewProduct != nil && skcIndex < len(pkg.PreviewProduct.SKCList) && skuIndex < len(pkg.PreviewProduct.SKCList[skcIndex].SKUS) {
				pkg.PreviewProduct.SKCList[skcIndex].SKUS[skuIndex].SupplierSKU = newSKU
			}
			globalIndex++
		}
	}

	if !changed {
		return false
	}
	applySheinStudioSupplierSKURenames(pkg, renames)
	return true
}

func matchStudioSubmitVariantOption(sds *SDSSyncOptions, draftSKC *SheinSKCRequestDraft, draftSKU *sheinpub.SKUDraft, globalIndex int) (*SDSSyncVariantOption, int) {
	if sds == nil || len(sds.Variants) == 0 {
		return nil, -1
	}

	sourceSKU := strings.TrimSpace(draftSKU.Attributes["source_sds_sku"])
	color := firstNonEmptyString(
		draftSKU.Attributes["Color"],
		draftSKU.Attributes["color"],
		sheinDraftSKCSaleAttributeValue(draftSKC),
	)
	size := firstNonEmptyString(
		draftSKU.Attributes["Size"],
		draftSKU.Attributes["size"],
	)

	if sourceSKU != "" {
		for i := range sds.Variants {
			if strings.EqualFold(strings.TrimSpace(sds.Variants[i].VariantSKU), sourceSKU) {
				return &sds.Variants[i], i
			}
		}
	}

	if color != "" || size != "" {
		for i := range sds.Variants {
			if studioSubmitVariantMatches(&sds.Variants[i], color, size) {
				return &sds.Variants[i], i
			}
		}
	}

	colorMatches := make([]int, 0, len(sds.Variants))
	if color != "" {
		for i := range sds.Variants {
			if strings.EqualFold(strings.TrimSpace(sds.Variants[i].Color), strings.TrimSpace(color)) {
				colorMatches = append(colorMatches, i)
			}
		}
		if len(colorMatches) == 1 {
			return &sds.Variants[colorMatches[0]], colorMatches[0]
		}
	}

	if globalIndex >= 0 && globalIndex < len(sds.Variants) {
		return &sds.Variants[globalIndex], globalIndex
	}
	return nil, -1
}

func studioSubmitVariantMatches(item *SDSSyncVariantOption, color, size string) bool {
	if item == nil {
		return false
	}
	if color != "" && !strings.EqualFold(strings.TrimSpace(item.Color), strings.TrimSpace(color)) {
		return false
	}
	if size != "" && !strings.EqualFold(strings.TrimSpace(item.Size), strings.TrimSpace(size)) {
		return false
	}
	return color != "" || size != ""
}

func resolveStudioSubmitBaseSKU(sds *SDSSyncOptions, draftSKU *sheinpub.SKUDraft, match *SDSSyncVariantOption, oldSKU string) string {
	if match != nil {
		return firstNonEmptyString(match.VariantSKU, sds.VariantSKU, sds.ProductSKU)
	}
	return firstNonEmptyString(
		draftSKU.Attributes["source_sds_sku"],
		sds.VariantSKU,
		sds.ProductSKU,
		inferStudioSubmitBaseSKUFromOld(oldSKU, sds.StyleID),
	)
}

func resolveStudioSubmitVariantDiscriminator(sds *SDSSyncOptions, draftSKU *sheinpub.SKUDraft, match *SDSSyncVariantOption, matchedIndex, globalIndex int, taskDiscriminator string) string {
	base := ""
	if match != nil {
		base = studioVariantDiscriminator(*match, matchedIndex)
	} else {
		color := firstNonEmptyString(draftSKU.Attributes["Color"], draftSKU.Attributes["color"])
		size := firstNonEmptyString(draftSKU.Attributes["Size"], draftSKU.Attributes["size"])
		if color != "" || size != "" {
			base = strings.Join([]string{
				strings.TrimSpace(color),
				strings.TrimSpace(size),
				"V" + itoa(globalIndex+1),
			}, "-")
		} else {
			base = studioFallbackVariantDiscriminator(sds)
		}
	}
	if taskDiscriminator == "" {
		return base
	}
	if base == "" {
		return taskDiscriminator
	}
	return strings.Join([]string{
		base,
		taskDiscriminator,
	}, "-")
}

func studioSubmitRequiresVariantDiscriminator(sds *SDSSyncOptions, baseSKU string) bool {
	if sds == nil {
		return false
	}
	if len(sds.Variants) > 0 {
		key := strings.TrimSpace(baseSKU)
		if key == "" {
			key = "__empty__"
		}
		return studioVariantBaseSKUCounts(sds)[key] > 1
	}
	return strings.TrimSpace(sds.VariantSKU) == ""
}

func inferStudioSubmitBaseSKUFromOld(oldSKU, styleID string) string {
	oldSKU = strings.TrimSpace(oldSKU)
	if oldSKU == "" {
		return ""
	}
	styleSuffix := normalizeStyleIDSuffix(styleID)
	if styleSuffix == "" {
		return oldSKU
	}
	suffix := "-" + styleSuffix
	upper := strings.ToUpper(oldSKU)
	if !strings.HasSuffix(upper, suffix) {
		return oldSKU
	}
	base := strings.TrimSpace(oldSKU[:len(oldSKU)-len(suffix)])
	if idx := strings.LastIndex(base, "-"); idx > 0 {
		prefix := strings.TrimSpace(base[:idx])
		discriminator := normalizeStudioVariantDiscriminator(base[idx+1:])
		if prefix != "" && discriminator != "" && strings.HasPrefix(discriminator, "V") {
			return prefix
		}
	}
	return base
}

func applySheinStudioSupplierSKURenames(pkg *sheinpub.Package, renames []sheinStudioSupplierSKURename) {
	if pkg == nil || len(renames) == 0 {
		return
	}
	renameMap := make(map[string][]string, len(renames))
	for _, item := range renames {
		oldKey := strings.TrimSpace(item.Old)
		newKey := strings.TrimSpace(item.New)
		if newKey == "" {
			continue
		}
		existing := renameMap[oldKey]
		duplicate := false
		for _, candidate := range existing {
			if strings.EqualFold(candidate, newKey) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			renameMap[oldKey] = append(renameMap[oldKey], newKey)
		}
	}

	if pkg.FinalDraft != nil {
		pkg.FinalDraft.ManualPriceOverrides = remapSheinPriceOverrides(pkg.FinalDraft.ManualPriceOverrides, renameMap)
	}
	if pkg.Pricing != nil {
		pkg.Pricing.ManualOverrides = remapSheinPriceOverrides(pkg.Pricing.ManualOverrides, renameMap)
		occurrence := map[string]int{}
		for i := range pkg.Pricing.SKUPrices {
			oldKey := strings.TrimSpace(pkg.Pricing.SKUPrices[i].SupplierSKU)
			replacements := renameMap[oldKey]
			switch {
			case len(replacements) == 0:
				continue
			case len(replacements) == 1:
				pkg.Pricing.SKUPrices[i].SupplierSKU = replacements[0]
			default:
				index := occurrence[oldKey]
				if index >= len(replacements) {
					index = len(replacements) - 1
				}
				pkg.Pricing.SKUPrices[i].SupplierSKU = replacements[index]
				occurrence[oldKey] = index + 1
			}
		}
	}
}

func remapSheinPriceOverrides(input map[string]float64, renameMap map[string][]string) map[string]float64 {
	if len(input) == 0 {
		return input
	}
	output := make(map[string]float64, len(input))
	for key, value := range input {
		replacements := renameMap[strings.TrimSpace(key)]
		if len(replacements) == 0 {
			output[key] = value
			continue
		}
		for _, replacement := range replacements {
			output[replacement] = value
		}
	}
	return output
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := [20]byte{}
	index := len(digits)
	v := value
	for v > 0 {
		index--
		digits[index] = byte('0' + v%10)
		v /= 10
	}
	return string(digits[index:])
}

func sheinStudioStyleID(options *SheinStudioOptions) string {
	if options == nil {
		return ""
	}
	return options.StyleID
}

func sheinDraftSKCSaleAttributeValue(draft *SheinSKCRequestDraft) string {
	if draft == nil || draft.SaleAttribute == nil {
		return ""
	}
	return draft.SaleAttribute.Value
}

func studioSubmitTaskDiscriminator(taskID string) string {
	taskID = strings.TrimSpace(strings.ToUpper(taskID))
	if taskID == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("T")
	for _, r := range taskID {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
		if b.Len() >= 9 {
			break
		}
	}
	if b.Len() <= 1 {
		return ""
	}
	return b.String()
}
