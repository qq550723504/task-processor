package productimage

import (
	"encoding/json"
	"sort"
	"strings"
)

type sellingPointContentEntry struct {
	Slot       string `json:"slot"`
	Text       string `json:"text"`
	ContentType string `json:"content_type,omitempty"`
	SourceKey   string `json:"source_key,omitempty"`
	SourceType  string `json:"source_type,omitempty"`
}

type sellingPointContentPlan struct {
	Copy          []sellingPointContentEntry `json:"copy,omitempty"`
	Badges        []sellingPointContentEntry `json:"badges,omitempty"`
	Measurements  []sellingPointContentEntry `json:"measurements,omitempty"`
	DetailAnchors []sellingPointContentEntry `json:"detail_anchors,omitempty"`
}

type sellingPointContentCandidate struct {
	Text        string
	ContentType string
	SourceKey   string
	SourceType  string
}

func buildSellingPointContentPlan(profile sceneProfile, productContext *ProductContext) *sellingPointContentPlan {
	slotPlan := buildSellingPointSlotPlan(profile)
	if slotPlan == nil {
		return nil
	}
	title := firstNonEmpty(productContextValue(productContext, "title"), productContextValue(productContext, "scraped_title"), productContextValue(productContext, "product_type"))
	attrPairs := sortedProductContextAttributes(productContext)
	measurementPairs, detailPairs, badgePairs := classifyProductContextAttributes(attrPairs)

	plan := &sellingPointContentPlan{
		Copy:          assignSellingPointEntries(slotPlan.CopySlots, buildCopyCandidates(title, productContext, detailPairs), slotPlan.MaxCopyLines),
		Badges:        assignSellingPointEntries(slotPlan.BadgeSlots, buildBadgeCandidates(productContext, badgePairs), slotPlan.MaxBadges),
		Measurements:  assignSellingPointEntries(slotPlan.MeasurementSlots, measurementPairs, len(slotPlan.MeasurementSlots)),
		DetailAnchors: assignSellingPointEntries(slotPlan.DetailAnchors, detailPairs, len(slotPlan.DetailAnchors)),
	}
	if len(plan.Copy) == 0 && len(plan.Badges) == 0 && len(plan.Measurements) == 0 && len(plan.DetailAnchors) == 0 {
		return nil
	}
	return plan
}

func assignSellingPointEntries(slots []string, values []sellingPointContentCandidate, limit int) []sellingPointContentEntry {
	if len(slots) == 0 || len(values) == 0 {
		return nil
	}
	if limit <= 0 || limit > len(values) {
		limit = len(values)
	}
	if limit > len(slots) {
		limit = len(slots)
	}
	out := make([]sellingPointContentEntry, 0, limit)
	for idx := 0; idx < limit; idx++ {
		text := strings.TrimSpace(values[idx].Text)
		if text == "" {
			continue
		}
		out = append(out, sellingPointContentEntry{
			Slot:        strings.TrimSpace(slots[idx]),
			Text:        text,
			ContentType: strings.TrimSpace(values[idx].ContentType),
			SourceKey:   strings.TrimSpace(values[idx].SourceKey),
			SourceType:  strings.TrimSpace(values[idx].SourceType),
		})
	}
	return out
}

func buildCopyCandidates(title string, productContext *ProductContext, detailPairs []sellingPointContentCandidate) []sellingPointContentCandidate {
	candidates := make([]sellingPointContentCandidate, 0, 3)
	if strings.TrimSpace(title) != "" {
		candidates = append(candidates, sellingPointContentCandidate{
			Text:        title,
			ContentType: "headline",
			SourceKey:   "title",
			SourceType:  "product_context",
		})
	}
	if productType := productContextValue(productContext, "product_type"); productType != "" && !strings.EqualFold(productType, title) {
		candidates = append(candidates, sellingPointContentCandidate{
			Text:        productType,
			ContentType: "supporting_copy",
			SourceKey:   "product_type",
			SourceType:  "product_context",
		})
	}
	for _, detail := range detailPairs {
		if strings.TrimSpace(detail.Text) == "" {
			continue
		}
		candidates = append(candidates, detail)
		if len(candidates) >= 3 {
			break
		}
	}
	return uniqueSellingPointContentCandidates(candidates)
}

func buildBadgeCandidates(productContext *ProductContext, badgePairs []sellingPointContentCandidate) []sellingPointContentCandidate {
	candidates := make([]sellingPointContentCandidate, 0, 3)
	for _, badge := range badgePairs {
		if strings.TrimSpace(badge.Text) == "" {
			continue
		}
		candidates = append(candidates, badge)
		if len(candidates) >= 3 {
			break
		}
	}
	if len(candidates) == 0 {
		if productType := productContextValue(productContext, "product_type"); productType != "" {
			candidates = append(candidates, sellingPointContentCandidate{
				Text:        productType,
				ContentType: "badge",
				SourceKey:   "product_type",
				SourceType:  "product_context",
			})
		}
	}
	return uniqueSellingPointContentCandidates(candidates)
}

func classifyProductContextAttributes(values []sellingPointContentCandidate) (measurements []sellingPointContentCandidate, details []sellingPointContentCandidate, badges []sellingPointContentCandidate) {
	for _, value := range values {
		lower := strings.ToLower(value.Text)
		switch {
		case strings.Contains(lower, "size"), strings.Contains(lower, "dimension"), strings.Contains(lower, "length"), strings.Contains(lower, "width"), strings.Contains(lower, "height"), strings.Contains(lower, "weight"):
			value.ContentType = "measurement"
			measurements = append(measurements, value)
		case strings.Contains(lower, "material"), strings.Contains(lower, "fabric"), strings.Contains(lower, "feature"), strings.Contains(lower, "style"), strings.Contains(lower, "fit"):
			value.ContentType = "badge"
			badges = append(badges, value)
		default:
			value.ContentType = "detail_anchor"
			details = append(details, value)
		}
	}
	return uniqueSellingPointContentCandidates(measurements), uniqueSellingPointContentCandidates(details), uniqueSellingPointContentCandidates(badges)
}

func sortedProductContextAttributes(productContext *ProductContext) []sellingPointContentCandidate {
	if productContext == nil || len(productContext.Attributes) == 0 {
		return nil
	}
	keys := make([]string, 0, len(productContext.Attributes))
	for key := range productContext.Attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]sellingPointContentCandidate, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(productContext.Attributes[key])
		if value == "" {
			continue
		}
		out = append(out, sellingPointContentCandidate{
			Text:       strings.TrimSpace(key) + ": " + value,
			SourceKey:  strings.TrimSpace(key),
			SourceType: "attribute",
		})
	}
	return out
}

func applySellingPointContentPlanMetadata(metadata map[string]string, profile sceneProfile, productContext *ProductContext) {
	if metadata == nil {
		return
	}
	plan := buildSellingPointContentPlan(profile, productContext)
	if plan == nil {
		return
	}
	setMetadataDefault(metadata, "layout_content.copy", marshalSellingPointContentEntries(plan.Copy))
	setMetadataDefault(metadata, "layout_content.badges", marshalSellingPointContentEntries(plan.Badges))
	setMetadataDefault(metadata, "layout_content.measurements", marshalSellingPointContentEntries(plan.Measurements))
	setMetadataDefault(metadata, "layout_content.detail_anchors", marshalSellingPointContentEntries(plan.DetailAnchors))
	setMetadataDefault(metadata, "content_plan_version", "v1")
}

func marshalSellingPointContentEntries(entries []sellingPointContentEntry) string {
	if len(entries) == 0 {
		return ""
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return ""
	}
	return string(data)
}

func productContextValue(productContext *ProductContext, key string) string {
	if productContext == nil {
		return ""
	}
	switch key {
	case "title":
		return strings.TrimSpace(productContext.Title)
	case "scraped_title":
		return strings.TrimSpace(productContext.ScrapedTitle)
	case "product_type":
		return strings.TrimSpace(productContext.ProductType)
	default:
		return ""
	}
}

func uniqueTrimmedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func uniqueSellingPointContentCandidates(values []sellingPointContentCandidate) []sellingPointContentCandidate {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]sellingPointContentCandidate, 0, len(values))
	for _, value := range values {
		text := strings.TrimSpace(value.Text)
		if text == "" {
			continue
		}
		key := strings.ToLower(text) + "|" + strings.ToLower(strings.TrimSpace(value.SourceKey)) + "|" + strings.ToLower(strings.TrimSpace(value.ContentType))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		value.Text = text
		out = append(out, value)
	}
	return out
}
