package listingkit

import (
	"fmt"
	"strings"
)

type taskGenerationNavigationDispatchEntry struct{}

type taskGenerationNavigationDispatchInput struct {
	target       *GenerationReviewNavigationTarget
	responseMode string
	planMode     string
}

func buildTaskGenerationNavigationDispatchEntry() *taskGenerationNavigationDispatchEntry {
	return &taskGenerationNavigationDispatchEntry{}
}

func (e *taskGenerationNavigationDispatchEntry) run(req *GenerationReviewNavigationDispatchRequest) (*taskGenerationNavigationDispatchInput, error) {
	if req == nil || req.Target == nil {
		return nil, fmt.Errorf("%w: missing navigation target", ErrGenerationActionNotFound)
	}

	target := cloneGenerationReviewNavigationTarget(req.Target)
	ApplyGenerationConditionalBaselineToNavigationTarget(target, "")

	return &taskGenerationNavigationDispatchInput{
		target:       target,
		responseMode: normalizeGenerationActionResponseMode(req.ResponseMode),
		planMode:     normalizeGenerationNavigationDispatchPlanMode(req.PlanMode),
	}, nil
}

func normalizeGenerationNavigationDispatchPlanMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "execute_plan":
		return "execute_plan"
	default:
		return "primary_only"
	}
}
