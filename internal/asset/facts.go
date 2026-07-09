package asset

// Facts is the platform-neutral asset handoff shape used after source
// normalization and before ListingKit or marketplace-specific asset use.
type Facts struct {
	SourceKey      string
	SourceType     string
	SourcePlatform string
	SourceID       string
	Items          []ItemFacts
	Warnings       []FactWarning
}

// ItemFacts carries one source-normalized asset candidate.
type ItemFacts struct {
	SourceID  string
	URL       string
	MediaType string
	Role      string
	Checksum  string
}

// FactWarning records missing or suspicious source facts that downstream owners
// must see instead of receiving hidden defaults.
type FactWarning struct {
	Code    string
	Message string
	Field   string
}

// HasAssets reports whether any source asset candidate is available.
func (f Facts) HasAssets() bool {
	return len(f.Items) > 0
}
