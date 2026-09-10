package sourcing

import (
	"errors"
	"strings"
)

// ErrSourceIdentityRequired means an envelope lacks a strict source identity.
var ErrSourceIdentityRequired = errors.New("source identity is required")

// Normalize returns a provider-neutral envelope with normalized identity and
// warning codes while preserving its raw evidence, lineage, and source metadata.
func Normalize(in SourceEnvelope) (SourceEnvelope, error) {
	out := in.Normalize()
	if !out.Identity.Valid() {
		return SourceEnvelope{}, ErrSourceIdentityRequired
	}
	return out, nil
}

// NormalizePublicationEnvelope enforces the current source-neutral identity
// contract at the durable publication boundary. Legacy product-fetch identity
// fields may still serve older in-memory paths, but they cannot be persisted as
// source evidence or used to derive its identity.
func NormalizePublicationEnvelope(in SourceEnvelope) (SourceEnvelope, error) {
	identity := in.Identity
	if strings.TrimSpace(identity.SourceType) == "" ||
		strings.TrimSpace(identity.SourcePlatform) == "" ||
		strings.TrimSpace(identity.SourceID) == "" ||
		strings.TrimSpace(identity.Platform) != "" ||
		strings.TrimSpace(identity.Region) != "" ||
		strings.TrimSpace(identity.ProductID) != "" ||
		identity.StoreID != 0 {
		return SourceEnvelope{}, ErrInvalidSourcePublication
	}

	out, err := Normalize(in)
	if err != nil {
		return SourceEnvelope{}, err
	}
	// Generic normalization backfills compatibility aliases for legacy callers.
	// The new durable publication contract intentionally strips those aliases.
	out.Identity.Platform = ""
	out.Identity.Region = ""
	out.Identity.ProductID = ""
	out.Identity.StoreID = 0
	return out, nil
}
