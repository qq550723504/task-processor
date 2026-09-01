package sourcing

import "task-processor/internal/product/catalog"

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
		Sources: []catalog.SourceRecord{{
			Type:   normalized.Identity.SourceType,
			Detail: normalized.RawReference.ReferenceID,
		}},
	}, nil
}
