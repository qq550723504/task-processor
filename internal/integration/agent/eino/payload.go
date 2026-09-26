package einoruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"task-processor/internal/agent"
)

// Leave room before dispatch for bounded call references, usage, stop metadata
// and resume feedback (including JSON escaping). Payloads never consume it.
const stateHeadroom = 64 << 10
const maxCallMetadataBytes = 4 << 10

func fits(value any, limit int) bool {
	raw, err := json.Marshal(value)
	return err == nil && len(raw) <= limit
}

func (s *flowState) rejectPayload(kind, callID string, raw []byte, reason string) {
	sum := sha256.Sum256(raw)
	s.State.RejectedPayload = &agent.RejectedPayload{Kind: kind, CallID: callID, Reason: reason, SHA256: hex.EncodeToString(sum[:]), Bytes: len(raw)}
}

func (s *flowState) accept(next flowState, kind, callID string, raw []byte) bool {
	if !fits(next, agent.MaxStateBytes-stateHeadroom) {
		s.rejectPayload(kind, callID, raw, "state_limit")
		s.stop(agent.StopTooLarge)
		return false
	}
	*s = next
	return true
}
