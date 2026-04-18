package listingkit

import "strings"

type generationRecoveryProfile struct {
	Hint       string
	Priority   int
	Severity   string
	Urgency    string
	CTAKind    string
	Title      string
	Summary    string
	TitleKey   string
	SummaryKey string
}

var generationRecoveryProfiles = map[string]generationRecoveryProfile{
	"review_fallback": {
		Hint:       "review_fallback",
		Priority:   0,
		Severity:   "medium",
		Urgency:    "now",
		CTAKind:    "review",
		Title:      "Review Fallback Path",
		Summary:    "A fallback result is available and should be reviewed before retrying generation.",
		TitleKey:   "generation.recovery.review_fallback.title",
		SummaryKey: "generation.recovery.review_fallback.summary",
	},
	"retry_dispatch": {
		Hint:       "retry_dispatch",
		Priority:   1,
		Severity:   "high",
		Urgency:    "now",
		CTAKind:    "retry",
		Title:      "Retry Generation Step",
		Summary:    "A recoverable generation step failed and can be retried now.",
		TitleKey:   "generation.recovery.retry_dispatch.title",
		SummaryKey: "generation.recovery.retry_dispatch.summary",
	},
	"refresh_revision": {
		Hint:       "refresh_revision",
		Priority:   2,
		Severity:   "medium",
		Urgency:    "soon",
		CTAKind:    "refresh",
		Title:      "Refresh Resource Revision",
		Summary:    "The current revision is stale and should be refreshed before continuing.",
		TitleKey:   "generation.recovery.refresh_revision.title",
		SummaryKey: "generation.recovery.refresh_revision.summary",
	},
	"wait_for_generation": {
		Hint:       "wait_for_generation",
		Priority:   3,
		Severity:   "low",
		Urgency:    "later",
		CTAKind:    "monitor",
		Title:      "Wait For Generation",
		Summary:    "The asset is not ready yet. Refresh the queue after generation progresses.",
		TitleKey:   "generation.recovery.wait_for_generation.title",
		SummaryKey: "generation.recovery.wait_for_generation.summary",
	},
}

var defaultGenerationRecoveryProfile = generationRecoveryProfile{
	Priority:   4,
	Title:      "Review Recovery Options",
	Summary:    "Review available recovery actions for the current resource set.",
	TitleKey:   "generation.recovery.default.title",
	SummaryKey: "generation.recovery.default.summary",
}

func generationRecoveryProfileForHint(hint string) generationRecoveryProfile {
	normalized := strings.TrimSpace(strings.ToLower(hint))
	if profile, ok := generationRecoveryProfiles[normalized]; ok {
		return profile
	}
	return defaultGenerationRecoveryProfile
}
