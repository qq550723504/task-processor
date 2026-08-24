package ownerreconcile

import "testing"

func TestResolveCandidatesRequiresOneUniqueSubject(t *testing.T) {
	tests := []struct {
		name       string
		candidates []Candidate
		want       string
		reason     string
	}{
		{name: "no candidate", candidates: nil, reason: "no_candidate"},
		{name: "one candidate", candidates: []Candidate{{Source: "creator", Subject: "sub-1"}}, want: "sub-1"},
		{name: "equal candidates", candidates: []Candidate{{Source: "creator", Subject: "sub-1"}, {Source: "store", Subject: "sub-1"}}, want: "sub-1"},
		{name: "blank candidate is ignored", candidates: []Candidate{{Source: "creator", Subject: " "}, {Source: "store", Subject: "sub-1"}}, want: "sub-1"},
		{name: "conflicting candidates", candidates: []Candidate{{Source: "task", Subject: "sub-1"}, {Source: "store", Subject: "sub-2"}}, reason: "conflicting_candidates"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := ResolveCandidates(tt.candidates)
			if got != tt.want || reason != tt.reason {
				t.Fatalf("ResolveCandidates() = (%q, %q), want (%q, %q)", got, reason, tt.want, tt.reason)
			}
		})
	}
}

func TestResolveCandidatePolicyCreatorFirst(t *testing.T) {
	got, reason := resolveCandidatePolicy(CandidatePolicyCreatorFirst, []Candidate{
		{Source: "creator", Subject: "subject-creator"},
		{Source: "created_by", Subject: "subject-auditor"},
	})
	if got != "subject-creator" || reason != "" {
		t.Fatalf("resolveCandidatePolicy() = (%q, %q), want creator subject without a reason", got, reason)
	}
}

func TestResolveCandidatePolicyUnmappedCreatorDoesNotFallback(t *testing.T) {
	got, reason := resolveCandidatePolicy(CandidatePolicyCreatorFirst, []Candidate{
		{Source: "creator", Subject: ""},
		{Source: "created_by", Subject: "subject-auditor"},
	})
	if got != "" || reason != "unmapped_candidate" {
		t.Fatalf("resolveCandidatePolicy() = (%q, %q), want unmapped creator blocker", got, reason)
	}
}

func TestResolveCandidatePolicyStoreOnly(t *testing.T) {
	got, reason := resolveCandidatePolicy(CandidatePolicyStoreOnly, []Candidate{
		{Source: "creator", Subject: "subject-row"},
		{Source: "store", Subject: "subject-store"},
		{Source: "store_created_by", Subject: "subject-auditor"},
	})
	if got != "subject-store" || reason != "" {
		t.Fatalf("resolveCandidatePolicy() = (%q, %q), want store subject without a reason", got, reason)
	}
}

func TestResolveCandidateValuesCreatorFirstDoesNotUseStoreAsFallback(t *testing.T) {
	subject, reason := resolveCandidateValues(CandidatePolicyCreatorFirst, "tenant-1", []Candidate{
		{Source: "creator", Subject: "legacy-missing"},
		{Source: "created_by", Subject: ""},
		{Source: "store", Subject: "legacy-store"},
	}, map[string]string{
		legacyIdentityKey("tenant-1", "legacy-store"): "subject-store",
	})
	if subject != "" || reason != "unmapped_candidate" {
		t.Fatalf("resolveCandidateValues() = (%q, %q), want unmapped row creator without store fallback", subject, reason)
	}
}
