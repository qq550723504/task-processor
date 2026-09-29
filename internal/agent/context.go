package agent

import "encoding/hex"

// ContextSnapshotRef is an opaque, already-materialized context identity. The
// runtime fingerprints/carries it; content and authorization belong to its owner.
type ContextSnapshotRef struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

func (r ContextSnapshotRef) Absent() bool { return r == (ContextSnapshotRef{}) }
func (r ContextSnapshotRef) Valid() bool {
	digest, err := hex.DecodeString(r.Digest)
	return ValidID(r.Kind) && ValidID(r.ID) && err == nil && len(digest) == 32 && hex.EncodeToString(digest) == r.Digest
}
func (r ContextSnapshotRef) ValidOrAbsent() bool { return r.Absent() || r.Valid() }

// ContextCitationRef is returned only after the governed adapter validates the
// citation against the exact context. It contains no protected display content.
type ContextCitationRef struct {
	Snapshot ContextSnapshotRef
	ID       string
}

const MaxContextCitationRefs = 64

func ValidContextCitations(snapshot ContextSnapshotRef, refs []ContextCitationRef) bool {
	if !snapshot.ValidOrAbsent() || len(refs) > MaxContextCitationRefs || len(refs) > 0 && snapshot.Absent() {
		return false
	}
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		if ref.Snapshot != snapshot || !ValidID(ref.ID) || seen[ref.ID] {
			return false
		}
		seen[ref.ID] = true
	}
	return true
}
