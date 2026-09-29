package review

import "encoding/hex"

// ContextProvenanceRef explains the original AI candidate. It is never Product
// evidence or permission to read content, and stores no labels or excerpts.
type ContextProvenanceRef struct {
	Kind, BundleID, BundleDigest, OriginAgentRunID string
	CitationIDs                                    []string
}

func (p *ContextProvenanceRef) Valid() bool {
	if p == nil {
		return true
	}
	digest, err := hex.DecodeString(p.BundleDigest)
	if !ValidKey(p.Kind) || !validID(p.BundleID) || !validID(p.OriginAgentRunID) || err != nil ||
		len(digest) != 32 || hex.EncodeToString(digest) != p.BundleDigest || len(p.CitationIDs) > 64 {
		return false
	}
	seen := make(map[string]bool, len(p.CitationIDs))
	for _, id := range p.CitationIDs {
		if !validID(id) || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func (p *ContextProvenanceRef) clone() *ContextProvenanceRef {
	if p == nil {
		return nil
	}
	copied := *p
	copied.CitationIDs = append([]string(nil), p.CitationIDs...)
	return &copied
}
