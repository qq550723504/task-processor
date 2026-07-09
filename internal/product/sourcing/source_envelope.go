package sourcing

import "strings"

// SourceEnvelope is the neutral handoff shape from source adapters into product,
// catalog, asset, and ListingKit orchestration. It should not carry target
// marketplace publishing payloads or runtime adapter dependencies.
type SourceEnvelope struct {
	Identity            SourceIdentity
	RawReference        RawSourceReference
	ProductCandidate    ProductCandidate
	AssetCandidates     []AssetCandidate
	SupplierOrCostFacts SupplierOrCostFacts
	Warnings            []SourceWarning
	Trace               SourceTrace
}

// Normalize returns a copy with normalized identity and warning metadata.
func (e SourceEnvelope) Normalize() SourceEnvelope {
	e.Identity = NormalizeSourceIdentity(e.Identity)
	for i := range e.Warnings {
		e.Warnings[i] = e.Warnings[i].Normalize()
	}
	return e
}

// RawSourceReference points back to raw source evidence without forcing product
// sourcing to own crawler/browser/runtime execution details.
type RawSourceReference struct {
	ReferenceType string
	ReferenceID   string
	URL           string
	SnapshotID    string
	Checksum      string
}

// ProductCandidate carries platform-neutral product facts that can later be
// mapped into catalog facts. Keep target-marketplace category and publishing
// payload decisions out of this shape.
type ProductCandidate struct {
	Title       string
	Description string
	Brand       string
	Attributes  map[string]string
	Variants    []ProductVariantCandidate
}

// ProductVariantCandidate carries neutral variant facts from the source.
type ProductVariantCandidate struct {
	SourceID   string
	Title      string
	SKU        string
	Attributes map[string]string
}

// AssetCandidate carries neutral image/design facts from the source.
type AssetCandidate struct {
	SourceID  string
	URL       string
	MediaType string
	Role      string
	Checksum  string
}

// SupplierOrCostFacts carries source-side commercial facts only when they are
// platform-neutral enough to be reused by later catalog/listing steps.
type SupplierOrCostFacts struct {
	SupplierID   string
	SupplierName string
	Currency     string
	Cost         string
	Price        string
	Facts        map[string]string
}

// SourceWarning records missing or suspicious source facts without hiding them
// behind defaults.
type SourceWarning struct {
	Code    string
	Message string
	Field   string
}

// Normalize returns a warning with trimmed metadata.
func (w SourceWarning) Normalize() SourceWarning {
	w.Code = strings.ToLower(strings.TrimSpace(w.Code))
	w.Field = strings.TrimSpace(w.Field)
	w.Message = strings.TrimSpace(w.Message)
	return w
}

// SourceTrace carries operator/debug information that can explain source import
// behavior without leaking raw runtime clients into product sourcing.
type SourceTrace struct {
	SourceRunID string
	RequestID   string
	Notes       []string
}
