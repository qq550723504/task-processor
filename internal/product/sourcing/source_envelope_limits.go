package sourcing

const (
	// MaxSourceEnvelopeStringBytes and MaxSourceEnvelopeCollectionItems reuse
	// the current server-side product input bounds. They are enforced before
	// normalization or JSON materialization.
	MaxSourceEnvelopeStringBytes     = 8 << 10
	MaxSourceEnvelopeCollectionItems = 256
	MaxSourceEnvelopeAggregateItems  = 1024
)

type sourceEnvelopeBudget struct {
	stringBytes int
	items       int
}

// validateSourceEnvelopePreflight walks the caller-owned shape without copying
// strings, slices, or maps. The encoded limits remain authoritative after
// normalization; this preflight prevents unbounded work before those limits can
// be measured.
func validateSourceEnvelopePreflight(envelope SourceEnvelope) error {
	budget := sourceEnvelopeBudget{}
	if !budget.addStrings(
		envelope.Identity.SourceType,
		envelope.Identity.SourcePlatform,
		envelope.Identity.SourceID,
		envelope.Identity.SourceURL,
		envelope.Identity.SourceVersion,
		envelope.Identity.SourceFingerprint,
		envelope.Identity.Platform,
		envelope.Identity.Region,
		envelope.RawReference.ReferenceType,
		envelope.RawReference.ReferenceID,
		envelope.RawReference.URL,
		envelope.RawReference.SnapshotID,
		envelope.RawReference.Checksum,
		envelope.ProductCandidate.Title,
		envelope.ProductCandidate.Description,
		envelope.ProductCandidate.Brand,
		envelope.SupplierOrCostFacts.SupplierID,
		envelope.SupplierOrCostFacts.SupplierName,
		envelope.SupplierOrCostFacts.Currency,
		envelope.SupplierOrCostFacts.Cost,
		envelope.SupplierOrCostFacts.Price,
		envelope.Trace.SourceRunID,
		envelope.Trace.RequestID,
	) {
		return ErrSourcePublicationTooLarge
	}
	if !budget.addMap(envelope.RawReference.Metadata) ||
		!budget.addSlice(envelope.ProductCandidate.CategoryPath) ||
		!budget.addMap(envelope.ProductCandidate.Attributes) ||
		!budget.addItems(len(envelope.ProductCandidate.Variants)) {
		return ErrSourcePublicationTooLarge
	}
	for _, variant := range envelope.ProductCandidate.Variants {
		if !budget.addStrings(variant.SourceID, variant.Title, variant.SKU, variant.Currency) ||
			!budget.addMap(variant.Attributes) {
			return ErrSourcePublicationTooLarge
		}
	}
	if !budget.addItems(len(envelope.AssetCandidates)) {
		return ErrSourcePublicationTooLarge
	}
	for _, asset := range envelope.AssetCandidates {
		if !budget.addStrings(asset.SourceID, asset.URL, asset.MediaType, asset.Role, asset.Checksum) {
			return ErrSourcePublicationTooLarge
		}
	}
	if !budget.addMap(envelope.SupplierOrCostFacts.Facts) ||
		!budget.addSlice(envelope.Trace.Notes) ||
		!budget.addItems(len(envelope.MissingFacts)) {
		return ErrSourcePublicationTooLarge
	}
	for _, fact := range envelope.MissingFacts {
		if !budget.addStrings(fact.Field, fact.Reason) {
			return ErrSourcePublicationTooLarge
		}
	}
	if !budget.addItems(len(envelope.Warnings)) {
		return ErrSourcePublicationTooLarge
	}
	for _, warning := range envelope.Warnings {
		if !budget.addStrings(warning.Code, warning.Message, warning.Field) {
			return ErrSourcePublicationTooLarge
		}
	}
	return nil
}

func (b *sourceEnvelopeBudget) addItems(count int) bool {
	if count < 0 || count > MaxSourceEnvelopeCollectionItems || count > MaxSourceEnvelopeAggregateItems-b.items {
		return false
	}
	b.items += count
	return true
}

func (b *sourceEnvelopeBudget) addStrings(values ...string) bool {
	for _, value := range values {
		if len(value) > MaxSourceEnvelopeStringBytes || len(value) > MaxEncodedEnvelopeBytes-b.stringBytes {
			return false
		}
		b.stringBytes += len(value)
	}
	return true
}

func (b *sourceEnvelopeBudget) addSlice(values []string) bool {
	if !b.addItems(len(values)) {
		return false
	}
	for _, value := range values {
		if !b.addStrings(value) {
			return false
		}
	}
	return true
}

func (b *sourceEnvelopeBudget) addMap(values map[string]string) bool {
	if !b.addItems(len(values)) {
		return false
	}
	for key, value := range values {
		if !b.addStrings(key, value) {
			return false
		}
	}
	return true
}
