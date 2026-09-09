package sourcing

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

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
	MissingFacts        []MissingFact
	Trace               SourceTrace
}

// Normalize returns a copy with normalized identity and warning metadata.
func (e SourceEnvelope) Normalize() SourceEnvelope {
	e.Identity = NormalizeSourceIdentity(e.Identity)
	e.RawReference.Metadata = cloneSourceMetadata(e.RawReference.Metadata)
	e.ProductCandidate.CategoryPath = append([]string(nil), e.ProductCandidate.CategoryPath...)
	e.ProductCandidate.Attributes = cloneSourceMetadata(e.ProductCandidate.Attributes)
	if len(e.ProductCandidate.Variants) == 0 {
		e.ProductCandidate.Variants = nil
	} else {
		variants := make([]ProductVariantCandidate, len(e.ProductCandidate.Variants))
		for i := range e.ProductCandidate.Variants {
			variants[i] = e.ProductCandidate.Variants[i]
			variants[i].Attributes = cloneSourceMetadata(e.ProductCandidate.Variants[i].Attributes)
		}
		e.ProductCandidate.Variants = variants
	}
	e.AssetCandidates = append([]AssetCandidate(nil), e.AssetCandidates...)
	e.SupplierOrCostFacts.Facts = cloneSourceMetadata(e.SupplierOrCostFacts.Facts)
	e.Trace.Notes = append([]string(nil), e.Trace.Notes...)
	if len(e.MissingFacts) == 0 {
		e.MissingFacts = nil
	} else {
		missing := make([]MissingFact, len(e.MissingFacts))
		for i := range e.MissingFacts {
			missing[i] = e.MissingFacts[i].Normalize()
		}
		e.MissingFacts = missing
	}
	if len(e.Warnings) == 0 {
		e.Warnings = nil
		return e
	}
	warnings := make([]SourceWarning, len(e.Warnings))
	for i := range e.Warnings {
		warnings[i] = e.Warnings[i].Normalize()
	}
	e.Warnings = warnings
	return e
}

// MissingFact preserves an explicit absence discovered by the admitted source
// producer. It is evidence; it must never be replaced with a guessed default.
type MissingFact struct {
	Field  string
	Reason string
}

// Normalize returns bounded-shape metadata without inventing a missing value.
func (f MissingFact) Normalize() MissingFact {
	f.Field = strings.TrimSpace(f.Field)
	f.Reason = strings.TrimSpace(f.Reason)
	return f
}

// RawSourceReference points back to raw source evidence without forcing product
// sourcing to own crawler/browser/runtime execution details.
type RawSourceReference struct {
	ReferenceType string
	ReferenceID   string
	URL           string
	SnapshotID    string
	Checksum      string
	CapturedAt    time.Time
	Metadata      map[string]string
}

// RawSnapshotChecksum returns a stable content checksum for non-empty raw
// source evidence. Whitespace is evidence content and is therefore hashed
// exactly as supplied; whitespace-only values are treated as absent.
func RawSnapshotChecksum(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ProductCandidate carries platform-neutral product facts that can later be
// mapped into catalog facts. Keep target-marketplace category and publishing
// payload decisions out of this shape.
type ProductCandidate struct {
	Title        string
	Description  string
	Brand        string
	CategoryPath []string
	Attributes   map[string]string
	Variants     []ProductVariantCandidate
}

// ProductVariantCandidate carries neutral variant facts from the source.
type ProductVariantCandidate struct {
	SourceID   string
	Title      string
	SKU        string
	Attributes map[string]string
	Currency   string
	Price      float64
	Stock      int
}

// AssetCandidate carries neutral image/design facts from the source.
type AssetCandidate struct {
	SourceID  string
	URL       string
	MediaType string
	Role      string
	Checksum  string
	Width     int
	Height    int
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

func cloneSourceMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	out := make(map[string]string, len(metadata))
	for key, value := range metadata {
		out[key] = value
	}
	return out
}
