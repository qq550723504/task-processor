package listingkit

import (
	"fmt"
	"strings"
)

type taskGenerationActionTargetResolutionPhase struct{}

type taskGenerationActionTargetResolutionResult struct {
	target *AssetGenerationActionTarget
	source string
}

func buildTaskGenerationActionTargetResolutionPhase() *taskGenerationActionTargetResolutionPhase {
	return &taskGenerationActionTargetResolutionPhase{}
}

func (p *taskGenerationActionTargetResolutionPhase) run(queue *GenerationWorkQueue, req *ExecuteGenerationActionRequest) (*taskGenerationActionTargetResolutionResult, error) {
	overview := buildAssetGenerationOverview(queue)
	target, source, err := resolveAssetGenerationActionTarget(overview, req)
	if err != nil {
		return nil, err
	}
	return &taskGenerationActionTargetResolutionResult{
		target: target,
		source: source,
	}, nil
}

func resolveAssetGenerationActionTarget(overview *AssetGenerationOverview, req *ExecuteGenerationActionRequest) (*AssetGenerationActionTarget, string, error) {
	actionKey := requestedAssetGenerationActionKey(req)
	if actionKey == "" {
		return nil, "", fmt.Errorf("%w: missing action key", ErrGenerationActionNotFound)
	}
	if !isAllowedAssetGenerationActionKey(actionKey) {
		return nil, "", fmt.Errorf("%w: %s", ErrGenerationActionNotFound, actionKey)
	}
	for _, candidate := range collectAssetGenerationActionTargets(overview) {
		if candidate == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(candidate.ActionKey), actionKey) {
			return cloneAssetGenerationActionTarget(candidate), "overview", nil
		}
	}
	if req != nil && req.Target != nil && strings.EqualFold(strings.TrimSpace(req.Target.ActionKey), actionKey) {
		cloned := cloneAssetGenerationActionTarget(req.Target)
		if strings.TrimSpace(cloned.InteractionMode) == "" {
			cloned.InteractionMode = actionInteractionMode(cloned.ActionKey)
		}
		return cloned, "request_target", nil
	}
	return nil, "", fmt.Errorf("%w: %s", ErrGenerationActionNotFound, actionKey)
}

func collectAssetGenerationActionTargets(overview *AssetGenerationOverview) []*AssetGenerationActionTarget {
	if overview == nil {
		return nil
	}
	out := make([]*AssetGenerationActionTarget, 0, 1+len(overview.SecondaryActionTargets))
	if overview.PrimaryActionTarget != nil {
		out = append(out, overview.PrimaryActionTarget)
	}
	out = append(out, overview.SecondaryActionTargets...)
	return out
}

func requestedAssetGenerationActionKey(req *ExecuteGenerationActionRequest) string {
	if req == nil {
		return ""
	}
	actionKey := strings.TrimSpace(req.ActionKey)
	if actionKey == "" && req.Target != nil {
		actionKey = strings.TrimSpace(req.Target.ActionKey)
	}
	return actionKey
}
