package sourcing

import (
	"sort"
	"strconv"
	"strings"

	"task-processor/internal/product/catalog"
)

// ToSnapshot converts a valid source envelope into canonical product facts.
// Asset candidates remain on the envelope for an authorized image/asset adapter.
func ToSnapshot(in SourceEnvelope) (catalog.ProductSnapshot, error) {
	normalized, err := Normalize(in)
	if err != nil {
		return catalog.ProductSnapshot{}, err
	}
	return catalog.ProductSnapshot{
		Title:        normalized.ProductCandidate.Title,
		Brand:        normalized.ProductCandidate.Brand,
		CategoryPath: snapshotCategoryPath(normalized.ProductCandidate.CategoryPath),
		Description:  normalized.ProductCandidate.Description,
		Attributes:   snapshotAttributes(normalized.ProductCandidate.Attributes),
		Variants:     snapshotVariants(normalized),
		Images:       snapshotImages(normalized),
		Review:       reviewFromSourceWarnings(normalized.Warnings),
		Warnings:     snapshotWarnings(normalized.Warnings),
		Sources: []catalog.SourceRecord{{
			Type: normalized.Identity.SourceType, Detail: normalized.RawReference.ReferenceID,
			Platform: normalized.Identity.SourcePlatform, SourceID: normalized.Identity.SourceID,
			SourceVersion: normalized.Identity.SourceVersion,
			ReferenceType: normalized.RawReference.ReferenceType, URL: normalized.RawReference.URL,
			SnapshotID: normalized.RawReference.SnapshotID, Checksum: normalized.RawReference.Checksum,
			CapturedAt: normalized.RawReference.CapturedAt, Metadata: normalized.RawReference.Metadata,
			SourceRunID: normalized.Trace.SourceRunID, RequestID: normalized.Trace.RequestID,
			Notes: normalized.Trace.Notes,
		}},
	}, nil
}

func snapshotCategoryPath(path []string) []string {
	if len(path) == 0 {
		return nil
	}
	result := make([]string, 0, len(path))
	for _, segment := range path {
		if segment = strings.TrimSpace(segment); segment != "" {
			result = append(result, segment)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func snapshotImages(envelope SourceEnvelope) []catalog.Image {
	if len(envelope.AssetCandidates) == 0 {
		return nil
	}
	images := make([]catalog.Image, 0, len(envelope.AssetCandidates))
	for _, candidate := range envelope.AssetCandidates {
		url := strings.TrimSpace(candidate.URL)
		mediaType := strings.ToLower(strings.TrimSpace(candidate.MediaType))
		if url == "" || mediaType != "" && mediaType != "image" && mediaType != "image/*" {
			continue
		}
		images = append(images, catalog.Image{
			URL: url, Role: strings.TrimSpace(candidate.Role), Width: candidate.Width, Height: candidate.Height,
			Trace: catalog.Trace{Sources: []catalog.SourceRecord{{
				Type: envelope.Identity.SourceType, Platform: envelope.Identity.SourcePlatform,
				SourceID: candidate.SourceID, URL: url, Checksum: strings.TrimSpace(candidate.Checksum),
				SourceRunID: envelope.Trace.SourceRunID, RequestID: envelope.Trace.RequestID,
			}}},
		})
	}
	if len(images) == 0 {
		return nil
	}
	return images
}

func snapshotWarnings(warnings []SourceWarning) []catalog.Warning {
	if len(warnings) == 0 {
		return nil
	}
	result := make([]catalog.Warning, len(warnings))
	for index, warning := range warnings {
		result[index] = catalog.Warning{Code: warning.Code, Field: warning.Field, Message: warning.Message}
	}
	return result
}

func reviewFromSourceWarnings(warnings []SourceWarning) *catalog.ReviewState {
	if len(warnings) == 0 {
		return nil
	}
	reasons := make([]string, 0, len(warnings))
	seen := make(map[string]struct{}, len(warnings))
	for _, warning := range warnings {
		reason := strings.TrimSpace(warning.Message)
		if reason == "" {
			reason = strings.TrimSpace(warning.Code)
		}
		if reason == "" {
			continue
		}
		if _, exists := seen[reason]; exists {
			continue
		}
		seen[reason] = struct{}{}
		reasons = append(reasons, reason)
	}
	if len(reasons) == 0 {
		return nil
	}
	return &catalog.ReviewState{NeedsReview: true, Reasons: reasons}
}

func snapshotVariants(envelope SourceEnvelope) []catalog.Variant {
	candidates := envelope.ProductCandidate.Variants
	if len(candidates) == 0 {
		return nil
	}
	cost, hasCost := parseSupplierCost(envelope.SupplierOrCostFacts)
	variants := make([]catalog.Variant, len(candidates))
	for index, candidate := range candidates {
		var price *catalog.Price
		if candidate.Price > 0 || hasCost {
			currency := strings.TrimSpace(candidate.Currency)
			if currency == "" {
				currency = strings.TrimSpace(envelope.SupplierOrCostFacts.Currency)
			}
			price = &catalog.Price{
				Currency: currency, Amount: candidate.Price,
				CostPrice: cost,
			}
		}
		variants[index] = catalog.Variant{
			SourceID:   candidate.SourceID,
			Title:      candidate.Title,
			SKU:        candidate.SKU,
			Attributes: snapshotAttributes(candidate.Attributes),
			Price:      price,
			Stock:      candidate.Stock,
		}
	}
	return variants
}

func parseSupplierCost(facts SupplierOrCostFacts) (float64, bool) {
	raw := strings.TrimSpace(facts.Cost)
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func snapshotAttributes(values map[string]string) []catalog.Attribute {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	attributes := make([]catalog.Attribute, 0, len(keys))
	for _, key := range keys {
		attributes = append(attributes, catalog.Attribute{Name: key, Value: values[key]})
	}
	return attributes
}
