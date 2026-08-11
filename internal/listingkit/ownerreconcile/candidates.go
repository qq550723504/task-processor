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

type CandidatePolicy uint8

const (
	CandidatePolicyUnset CandidatePolicy = iota
	CandidatePolicyCreatorFirst
	CandidatePolicyStoreOnly
	CandidatePolicySystemOwned
)

// resolveCandidateValues applies the persisted ownership policy to legacy IDs
// and only then translates the selected IDs through the verified migration map.
// A present but unmapped creator is intentionally not allowed to fall back to
// created_by; this keeps creator precedence deterministic and fail-closed.
func resolveCandidateValues(policy CandidatePolicy, tenantID string, candidates []Candidate, identities map[string]string) (subject, reason string) {
	if policy == CandidatePolicyUnset {
		mapped := make([]Candidate, 0, len(candidates))
		unmapped := false
		for _, candidate := range candidates {
			value := strings.TrimSpace(candidate.Subject)
			if value == "" {
				continue
			}
			subject := identities[legacyIdentityKey(tenantID, value)]
			if subject == "" {
				unmapped = true
			}
			mapped = append(mapped, Candidate{Source: candidate.Source, Subject: subject})
		}
		if unmapped {
			return "", "unmapped_candidate"
		}
		return ResolveCandidates(mapped)
	}
	if policy == CandidatePolicySystemOwned {
		return "", "system_owned"
	}
	selected := candidates
	if policy == CandidatePolicyStoreOnly {
		selected = make([]Candidate, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate.Source == "store" || candidate.Source == "store_created_by" {
				selected = append(selected, candidate)
			}
		}
	} else if policy == CandidatePolicyCreatorFirst {
		selected = make([]Candidate, 0, 2)
		for _, candidate := range candidates {
			if candidate.Source == "creator" || candidate.Source == "created_by" {
				selected = append(selected, candidate)
			}
		}
	}
	mapped := make([]Candidate, 0, len(selected))
	for _, candidate := range selected {
		value := strings.TrimSpace(candidate.Subject)
		if value == "" {
			continue
		}
		mappedSubject := identities[legacyIdentityKey(tenantID, value)]
		mapped = append(mapped, Candidate{Source: candidate.Source, Subject: mappedSubject})
	}
	return resolvePreferredCreator(mapped)
}

func resolveCandidatePolicy(policy CandidatePolicy, candidates []Candidate) (subject, reason string) {
	if policy == CandidatePolicySystemOwned {
		return "", "system_owned"
	}
	if policy == CandidatePolicyStoreOnly {
		filtered := make([]Candidate, 0, 2)
		for _, candidate := range candidates {
			if candidate.Source == "store" || candidate.Source == "store_created_by" {
				filtered = append(filtered, candidate)
			}
		}
		return resolvePreferredCreator(filtered)
	}
	filtered := make([]Candidate, 0, 2)
	for _, candidate := range candidates {
		if candidate.Source == "creator" || candidate.Source == "created_by" {
			filtered = append(filtered, candidate)
		}
	}
	return resolvePreferredCreator(filtered)
}

func resolvePreferredCreator(candidates []Candidate) (subject, reason string) {
	var creator Candidate
	var createdBy Candidate
	for _, candidate := range candidates {
		switch candidate.Source {
		case "creator", "store":
			creator = candidate
		case "created_by", "store_created_by":
			createdBy = candidate
		}
	}
	if strings.TrimSpace(creator.Subject) != "" {
		return strings.TrimSpace(creator.Subject), ""
	}
	if creator.Source != "" {
		return "", "unmapped_candidate"
	}
	if strings.TrimSpace(createdBy.Subject) != "" {
		return strings.TrimSpace(createdBy.Subject), ""
	}
	if createdBy.Source != "" {
		return "", "unmapped_candidate"
	}
	return "", "no_candidate"
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
