package listingkit

import "testing"

func TestResolveReviewReasonsPrefersWorkflowReasons(t *testing.T) {
	t.Parallel()

	got := resolveReviewReasons(reviewReasonSources{
		WorkflowReasons:    []string{"workflow review", "workflow review"},
		ResultReasons:      []string{"legacy result"},
		SummaryNeedsReview: true,
		SummaryWarnings:    []string{"summary warning"},
		PodBlocked:         true,
		PodFailureReason:   "pod failure",
		PlatformReasons:    []string{"platform review"},
	})
	if len(got) != 1 || got[0] != "workflow review" {
		t.Fatalf("resolveReviewReasons() = %#v, want workflow reason precedence", got)
	}
}

func TestResolveReviewReasonsFallsBackThroughSources(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		sources reviewReasonSources
		want    []string
	}{
		{
			name: "result reasons",
			sources: reviewReasonSources{
				ResultReasons: []string{"result reason", "result reason"},
			},
			want: []string{"result reason"},
		},
		{
			name: "summary warnings when review required",
			sources: reviewReasonSources{
				SummaryNeedsReview: true,
				SummaryWarnings:    []string{"summary reason", "summary reason"},
			},
			want: []string{"summary reason"},
		},
		{
			name: "blocked pod failure",
			sources: reviewReasonSources{
				PodBlocked:       true,
				PodFailureReason: "pod reason",
			},
			want: []string{"pod reason"},
		},
		{
			name: "platform reasons",
			sources: reviewReasonSources{
				PlatformReasons: []string{"platform reason", "platform reason"},
			},
			want: []string{"platform reason"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := resolveReviewReasons(tc.sources)
			if len(got) != len(tc.want) {
				t.Fatalf("resolveReviewReasons() = %#v, want %#v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("resolveReviewReasons() = %#v, want %#v", got, tc.want)
				}
			}
		})
	}
}
