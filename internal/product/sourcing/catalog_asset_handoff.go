package sourcing

import (
	"sort"

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
		Title:       normalized.ProductCandidate.Title,
		Brand:       normalized.ProductCandidate.Brand,
		Description: normalized.ProductCandidate.Description,
		Attributes:  snapshotAttributes(normalized.ProductCandidate.Attributes),
		Variants:    snapshotVariants(normalized.ProductCandidate.Variants),
		Sources: []catalog.SourceRecord{{
			Type:   normalized.Identity.SourceType,
			Detail: normalized.RawReference.ReferenceID,
		}},
	}, nil
}

func snapshotVariants(candidates []ProductVariantCandidate) []catalog.Variant {
	if len(candidates) == 0 {
		return nil
	}
	variants := make([]catalog.Variant, len(candidates))
	for index, candidate := range candidates {
		variants[index] = catalog.Variant{
			SourceID:   candidate.SourceID,
			Title:      candidate.Title,
			SKU:        candidate.SKU,
			Attributes: snapshotAttributes(candidate.Attributes),
		}
	}
	return variants
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
