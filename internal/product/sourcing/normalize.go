package sourcing

import "errors"

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
