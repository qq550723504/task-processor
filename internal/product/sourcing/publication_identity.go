package sourcing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// PublicationIdentity binds a source product to an immutable Catalog publication.
// An explicit run is an idempotency key, not a payload checksum: Catalog rejects
// changed payloads for the same key. Otherwise all durable snapshot facts,
// including lineage and warnings, determine the content publication.
func PublicationIdentity(envelope SourceEnvelope) (productKey, publicationID string, err error) {
	snapshot, err := ToSnapshot(envelope)
	if err != nil {
		return "", "", err
	}
	identity := NormalizeSourceIdentity(envelope.Identity)
	// A source version identifies a revision, not another product.
	identity.SourceVersion = ""
	productKey = boundedIdentity(identity.SourceKey(), "source-key-hash:")
	if run := strings.TrimSpace(envelope.Trace.SourceRunID); run != "" {
		return productKey, boundedIdentity("source-run:"+run, "source-run-hash:"), nil
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return "", "", err
	}
	digest := sha256.Sum256(payload)
	return productKey, "source-snapshot:" + hex.EncodeToString(digest[:]), nil
}

func boundedIdentity(value, prefix string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 128 {
		return value
	}
	digest := sha256.Sum256([]byte(value))
	return prefix + hex.EncodeToString(digest[:])
}
