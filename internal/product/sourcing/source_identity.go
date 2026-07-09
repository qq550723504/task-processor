package sourcing

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

const (
	// SourceTypeCrawler identifies source data collected by crawler/integration adapters.
	SourceTypeCrawler = "crawler"
	// SourceTypePODDesign identifies product/design facts from POD design systems.
	SourceTypePODDesign = "pod_design"
	// SourceTypeWarehouseCatalog identifies product facts from warehouse/source catalogs.
	SourceTypeWarehouseCatalog = "warehouse_catalog"
	// SourceTypeManualImport identifies operator-supplied product/source facts.
	SourceTypeManualImport = "manual_import"
)

// SourceIdentity is the stable identity for a sourced product/design record.
//
// The Source* fields are the product-sourcing MVP model. Platform, Region,
// ProductID, and StoreID are kept for compatibility with existing product fetch
// identity keys while callers migrate toward the source-neutral fields.
type SourceIdentity struct {
	SourceType        string
	SourcePlatform    string
	SourceID          string
	SourceURL         string
	SourceVersion     string
	SourceFingerprint string

	// Legacy product-fetch identity fields.
	Platform  string
	Region    string
	ProductID string
	StoreID   int64
}

// SourceIdentityValidation records whether an identity can be used safely for
// source lineage, dedupe, or retry decisions.
type SourceIdentityValidation struct {
	MissingSourceType     bool
	MissingSourcePlatform bool
	MissingSourceID       bool
	MissingFingerprint    bool
	Fingerprintable       bool
}

// Valid reports whether the identity has enough stable fields for normal source
// lineage. Weak-but-fingerprintable identities are intentionally not Valid; the
// caller must decide whether the weak identity is acceptable for a specific
// import path.
func (v SourceIdentityValidation) Valid() bool {
	return !v.MissingSourceType && !v.MissingSourcePlatform && !v.MissingSourceID
}

// WeakButFingerprintable reports whether the identity is missing a source id but
// still has enough source facts to build a deterministic fingerprint.
func (v SourceIdentityValidation) WeakButFingerprintable() bool {
	return !v.Valid() && v.MissingSourceID && v.Fingerprintable && !v.MissingFingerprint
}

// NormalizeSourceIdentity trims all fields, normalizes source type/platform
// tokens, fills legacy compatibility fields when possible, and derives a stable
// fingerprint for weak identities when source facts are available.
func NormalizeSourceIdentity(id SourceIdentity) SourceIdentity {
	id.SourceType = strings.ToLower(strings.TrimSpace(id.SourceType))
	id.SourcePlatform = strings.ToLower(strings.TrimSpace(id.SourcePlatform))
	id.SourceID = strings.TrimSpace(id.SourceID)
	id.SourceURL = strings.TrimSpace(id.SourceURL)
	id.SourceVersion = strings.TrimSpace(id.SourceVersion)
	id.SourceFingerprint = strings.ToLower(strings.TrimSpace(id.SourceFingerprint))
	id.Platform = strings.ToLower(strings.TrimSpace(id.Platform))
	id.Region = strings.ToLower(strings.TrimSpace(id.Region))
	id.ProductID = strings.TrimSpace(id.ProductID)

	if id.SourceType == "" && id.Platform != "" {
		id.SourceType = SourceTypeCrawler
	}
	if id.SourcePlatform == "" {
		id.SourcePlatform = id.Platform
	}
	if id.Platform == "" {
		id.Platform = id.SourcePlatform
	}
	if id.SourceID == "" {
		id.SourceID = id.ProductID
	}
	if id.ProductID == "" {
		id.ProductID = id.SourceID
	}
	if id.SourceFingerprint == "" && canFingerprintSourceIdentity(id) {
		id.SourceFingerprint = deriveSourceFingerprint(id)
	}
	return id
}

// Validation returns structured validation details for source import decisions.
func (id SourceIdentity) Validation() SourceIdentityValidation {
	id = NormalizeSourceIdentity(id)
	return SourceIdentityValidation{
		MissingSourceType:     id.SourceType == "",
		MissingSourcePlatform: id.SourcePlatform == "",
		MissingSourceID:       id.SourceID == "",
		MissingFingerprint:    id.SourceFingerprint == "",
		Fingerprintable:       canFingerprintSourceIdentity(id),
	}
}

// Key returns a stable string key for existing product-fetch identity comparisons
// and caches. It intentionally preserves the legacy platform:region:product_id
// shape and only appends StoreID when set.
func (id SourceIdentity) Key() string {
	id = NormalizeSourceIdentity(id)
	parts := []string{id.Platform, id.Region, id.ProductID}
	if id.StoreID > 0 {
		parts = append(parts, strconv.FormatInt(id.StoreID, 10))
	}
	return strings.Join(parts, ":")
}

// SourceKey returns a source-neutral identity key for future product-sourcing
// dedupe, lineage, and retry paths.
func (id SourceIdentity) SourceKey() string {
	id = NormalizeSourceIdentity(id)
	parts := []string{id.SourceType, id.SourcePlatform}
	if id.SourceID != "" {
		parts = append(parts, id.SourceID)
	} else {
		parts = append(parts, "fingerprint", id.SourceFingerprint)
	}
	if id.SourceVersion != "" {
		parts = append(parts, "version", id.SourceVersion)
	}
	return strings.Join(parts, ":")
}

func canFingerprintSourceIdentity(id SourceIdentity) bool {
	return strings.TrimSpace(id.SourceType) != "" &&
		strings.TrimSpace(id.SourcePlatform) != "" &&
		(strings.TrimSpace(id.SourceURL) != "" || strings.TrimSpace(id.SourceVersion) != "")
}

func deriveSourceFingerprint(id SourceIdentity) string {
	payload := strings.Join([]string{
		strings.ToLower(strings.TrimSpace(id.SourceType)),
		strings.ToLower(strings.TrimSpace(id.SourcePlatform)),
		strings.TrimSpace(id.SourceURL),
		strings.TrimSpace(id.SourceVersion),
	}, "\x00")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}
