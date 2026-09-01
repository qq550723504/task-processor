package catalog

// ProductFacts is the platform-neutral product handoff shape used after source
// normalization and before target-marketplace adaptation. It should not carry
// marketplace publishing payloads or crawler runtime state.
type ProductFacts struct {
	SourceKey      string
	SourceType     string
	SourcePlatform string
	SourceID       string
	SourceURL      string
	Title          string
	Description    string
	Brand          string
	Attributes     map[string]string
	Variants       []VariantFacts
	Warnings       []FactWarning
}

// VariantFacts carries source-normalized, platform-neutral variant facts.
type VariantFacts struct {
	SourceID   string
	Title      string
	SKU        string
	Attributes map[string]string
}

// FactWarning records missing or suspicious source facts that downstream owners
// must see instead of receiving hidden defaults.
type FactWarning struct {
	Code    string
	Message string
	Field   string
}

// HasIdentity reports whether the product facts can be traced back to a source.
func (f ProductFacts) HasIdentity() bool {
	return f.SourceKey != "" || (f.SourcePlatform != "" && f.SourceID != "")
}
