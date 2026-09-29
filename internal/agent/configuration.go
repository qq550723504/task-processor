package agent

import (
	"encoding/hex"
	"github.com/google/uuid"
)

// ConfigurationSnapshotRef is opaque comparable metadata. The runtime carries
// it unchanged; enterprise policy is resolved only by its own application.
type ConfigurationSnapshotRef struct{ Kind, ID, Digest string }

func (r ConfigurationSnapshotRef) Absent() bool { return r == ConfigurationSnapshotRef{} }
func (r ConfigurationSnapshotRef) ValidOrAbsent() bool {
	if r.Absent() {
		return true
	}
	id, e := uuid.Parse(r.ID)
	digest, de := hex.DecodeString(r.Digest)
	return r.Kind == "agent-configuration-v1" && e == nil && id != uuid.Nil && id.String() == r.ID && de == nil && len(digest) == 32 && hex.EncodeToString(digest) == r.Digest
}
