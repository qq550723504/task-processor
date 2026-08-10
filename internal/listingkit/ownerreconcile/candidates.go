package ownerreconcile

import "strings"

// LegacyIdentity is the verified migration relationship between a legacy user
// and the canonical ZITADEL subject. It is kept internal to reconciliation and
// is never serialized into an operator report.
type LegacyIdentity struct {
	TenantID     string
	LegacyUserID string
	Subject      string
}

// Candidate is a possible source for one persisted row's canonical subject.
type Candidate struct {
	Source  string
	Subject string
}

// ResolveCandidates returns a subject only when every non-empty candidate
// agrees. An empty candidate set or disagreement is intentionally fail-closed.
func ResolveCandidates(candidates []Candidate) (subject, reason string) {
	for _, candidate := range candidates {
		value := strings.TrimSpace(candidate.Subject)
		if value == "" {
			continue
		}
		if subject == "" {
			subject = value
			continue
		}
		if subject != value {
			return "", "conflicting_candidates"
		}
	}
	if subject == "" {
		return "", "no_candidate"
	}
	return subject, ""
}
