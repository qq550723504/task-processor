package listingkit

import "strings"

func ApplyGenerationConditionalBaselineToNavigationTarget(target *GenerationReviewNavigationTarget, fallbackIfMatch string) {
	if target == nil {
		return
	}
	baseline := strings.TrimSpace(fallbackIfMatch)
	if target.Conditional != nil && strings.TrimSpace(target.Conditional.DeltaToken) != "" {
		baseline = strings.TrimSpace(target.Conditional.DeltaToken)
	}
	if baseline == "" {
		return
	}
	applyGenerationConditionalBaselineToQuery(target.QueueQuery, baseline)
	applyGenerationConditionalBaselineToQuery(target.SessionQuery, baseline)
	applyGenerationConditionalBaselineToQuery(target.PreviewQuery, baseline)
	if target.ActionTarget != nil {
		applyGenerationConditionalBaselineToQuery(target.ActionTarget.QueueQuery, baseline)
	}
}

func applyGenerationConditionalBaselineToQuery(query *GenerationQueueQuery, baseline string) {
	if query == nil {
		return
	}
	if strings.TrimSpace(query.IfMatch) != "" || strings.TrimSpace(query.DeltaToken) != "" {
		return
	}
	query.IfMatch = strings.TrimSpace(baseline)
}
