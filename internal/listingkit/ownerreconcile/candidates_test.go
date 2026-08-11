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
