package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

type sheinStudioSupplierSKURename struct {
	Old string
	New string
}

func normalizeSheinStudioSubmitSupplierSKUs(task *Task, pkg *sheinpub.Package, submitRequestID string) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if task == nil || task.Request == nil || task.Request.Options == nil || pkg == nil || pkg.DraftPayload == nil {
		return false
	}
	sds := task.Request.Options.SDS
	if sds == nil {
		return false
	}
	taskDiscriminator := combineStudioSubmitDiscriminators(
		studioSubmitTaskDiscriminator(task.ID),
		studioSubmitRequestDiscriminator(submitRequestID),
	)
	styleID := resolveStudioSubmitStyleSuffix(task)
	if styleID == "" && strings.TrimSpace(sds.ProductSKU) == "" && strings.TrimSpace(sds.VariantSKU) == "" && len(sds.Variants) == 0 {
		return false
	}

	seen := map[string]int{}
	renames := make([]sheinStudioSupplierSKURename, 0)
	changed := false
	globalIndex := 0

	for skcIndex := range pkg.DraftPayload.SKCList {
		draftSKC := &pkg.DraftPayload.SKCList[skcIndex]
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
			if pkg.PreviewPayload != nil && skcIndex < len(pkg.PreviewPayload.SKCList) && skuIndex < len(pkg.PreviewPayload.SKCList[skcIndex].SKUS) {
				pkg.PreviewPayload.SKCList[skcIndex].SKUS[skuIndex].SupplierSKU = newSKU
				sheinpub.SetPreviewPayload(pkg, pkg.PreviewPayload)
			}
			globalIndex++
		}
	}

	if changed {
		applySheinStudioSupplierSKURenames(pkg, renames)
	}
	reconciled := reconcileSheinStudioPricingReferences(pkg)
	return changed || reconciled
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

	if pkg.FinalSubmissionDraft != nil {
		pkg.FinalSubmissionDraft.ManualPriceOverrides = remapSheinPriceOverrides(pkg.FinalSubmissionDraft.ManualPriceOverrides, renameMap)
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

func reconcileSheinStudioPricingReferences(pkg *sheinpub.Package) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return false
	}
	currentSKUs := collectSheinRequestDraftSupplierSKUs(pkg.DraftPayload)
	if len(currentSKUs) == 0 {
		return false
	}
	currentSKUSet := make(map[string]struct{}, len(currentSKUs))
	aliasMap := make(map[string][]string, len(currentSKUs))
	for _, sku := range currentSKUs {
		currentSKUSet[sku] = struct{}{}
		aliasKey := sheinStudioPricingSKUAlias(sku)
		if aliasKey == "" {
			continue
		}
		duplicate := false
		for _, existing := range aliasMap[aliasKey] {
			if strings.EqualFold(existing, sku) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			aliasMap[aliasKey] = append(aliasMap[aliasKey], sku)
		}
	}

	changed := false
	if pkg.FinalSubmissionDraft != nil {
		remapped, updated := reconcileSheinPriceOverrideAliases(pkg.FinalSubmissionDraft.ManualPriceOverrides, currentSKUSet, aliasMap)
		if updated {
			pkg.FinalSubmissionDraft.ManualPriceOverrides = remapped
			changed = true
		}
	}
	if pkg.Pricing != nil {
		remapped, updated := reconcileSheinPriceOverrideAliases(pkg.Pricing.ManualOverrides, currentSKUSet, aliasMap)
		if updated {
			pkg.Pricing.ManualOverrides = remapped
			changed = true
		}
		occurrence := map[string]int{}
		for index := range pkg.Pricing.SKUPrices {
			oldSKU := strings.TrimSpace(pkg.Pricing.SKUPrices[index].SupplierSKU)
			if oldSKU == "" {
				continue
			}
			if _, ok := currentSKUSet[oldSKU]; ok {
				continue
			}
			replacements := aliasMap[sheinStudioPricingSKUAlias(oldSKU)]
			if len(replacements) == 0 {
				continue
			}
			replaceIndex := occurrence[oldSKU]
			if replaceIndex >= len(replacements) {
				replaceIndex = len(replacements) - 1
			}
			if !strings.EqualFold(oldSKU, replacements[replaceIndex]) {
				pkg.Pricing.SKUPrices[index].SupplierSKU = replacements[replaceIndex]
				changed = true
			}
			occurrence[oldSKU] = replaceIndex + 1
		}
	}
	return changed
}

func collectSheinRequestDraftSupplierSKUs(draft *sheinpub.RequestDraft) []string {
	if draft == nil {
		return nil
	}
	skus := make([]string, 0)
	for _, skc := range draft.SKCList {
		for _, sku := range skc.SKUList {
			if value := strings.TrimSpace(sku.SupplierSKU); value != "" {
				skus = append(skus, value)
			}
		}
	}
	return skus
}

func reconcileSheinPriceOverrideAliases(
	input map[string]float64,
	currentSKUSet map[string]struct{},
	aliasMap map[string][]string,
) (map[string]float64, bool) {
	if len(input) == 0 {
		return input, false
	}
	output := make(map[string]float64, len(input))
	changed := false
	for key, value := range input {
		trimmed := strings.TrimSpace(key)
		if _, ok := currentSKUSet[trimmed]; ok {
			output[trimmed] = value
			continue
		}
		replacements := aliasMap[sheinStudioPricingSKUAlias(trimmed)]
		if len(replacements) == 0 {
			output[key] = value
			continue
		}
		changed = true
		for _, replacement := range replacements {
			output[replacement] = value
		}
	}
	return output, changed
}

func sheinStudioPricingSKUAlias(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "-")
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if looksLikeStudioSubmitRequestToken(part) || looksLikeStudioSubmitTaskToken(part) {
			continue
		}
		filtered = append(filtered, part)
	}
	return strings.Join(filtered, "-")
}

func looksLikeStudioSubmitRequestToken(token string) bool {
	token = strings.TrimSpace(strings.ToUpper(token))
	if len(token) < 6 || len(token) > 9 || !strings.HasPrefix(token, "R") {
		return false
	}
	for _, r := range token[1:] {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

func looksLikeStudioSubmitTaskToken(token string) bool {
	token = strings.TrimSpace(strings.ToUpper(token))
	if len(token) != 9 || !strings.HasPrefix(token, "T") {
		return false
	}
	for _, r := range token[1:] {
		switch {
		case r >= '0' && r <= '9', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
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

func resolveStudioSubmitStyleSuffix(task *Task) string {
	if task == nil || task.Request == nil || task.Request.Options == nil {
		return ""
	}
	if value := firstNonEmptyString(
		sheinStudioStyleID(task.Request.Options.SheinStudio),
		task.Request.Options.SDS.StyleID,
	); strings.TrimSpace(value) != "" {
		return value
	}
	return deriveStudioSubmitStyleSuffix(
		task.Request.Text,
		task.Request.Options.SDS.ProductEnglishName,
		task.Request.Options.SDS.ProductName,
	)
}

func deriveStudioSubmitStyleSuffix(values ...string) string {
	stopwords := map[string]bool{
		"THE": true, "AND": true, "FOR": true, "WITH": true, "FROM": true,
		"FRESH": true, "SDS": true, "TASK": true, "PUBLIC": true, "IMAGE": true,
		"RETRY": true, "TEST": true, "DEFAULT": true, "DESIGN": true,
	}
	tokens := make([]string, 0, 8)
	for _, value := range values {
		for _, token := range tokenizeStudioStyleSuffixWords(value) {
			if stopwords[token] {
				continue
			}
			tokens = append(tokens, token)
		}
	}
	if len(tokens) == 0 {
		return ""
	}
	shortToken := ""
	longToken := ""
	for _, token := range tokens {
		if shortToken == "" && len(token) >= 2 && len(token) <= 3 {
			shortToken = token
		}
		if len(token) > len(longToken) {
			longToken = token
		}
	}
	if shortToken != "" && longToken != "" && !strings.EqualFold(shortToken, longToken) {
		return normalizeStyleIDSuffix(shortToken + longToken)
	}
	var builder strings.Builder
	for _, token := range tokens {
		builder.WriteString(token)
		if builder.Len() >= 8 {
			break
		}
	}
	return normalizeStyleIDSuffix(builder.String())
}

func tokenizeStudioStyleSuffixWords(value string) []string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return nil
	}
	tokens := make([]string, 0, 8)
	var current strings.Builder
	flush := func() {
		if current.Len() == 0 {
			return
		}
		tokens = append(tokens, current.String())
		current.Reset()
	}
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			current.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	return tokens
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

func studioSubmitRequestDiscriminator(requestID string) string {
	requestID = strings.TrimSpace(strings.ToUpper(requestID))
	if requestID == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("R")
	for _, r := range requestID {
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

func combineStudioSubmitDiscriminators(values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, "-")
}
