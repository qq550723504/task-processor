package listingkit

import (
	"context"
	"fmt"
	"strings"
)

func (s *service) ExecuteTaskGenerationAction(ctx context.Context, taskID string, req *ExecuteGenerationActionRequest) (*GenerationActionExecutionResult, error) {
	return s.taskGenerationOrDefault().ExecuteTaskGenerationAction(ctx, taskID, req)
}

func resolveLayerTemporalPlatform(req *ExecuteGenerationActionRequest) string {
	return resolveTemporalRequestPlatform(req)
}

func resolveAssetGenerationActionTarget(overview *AssetGenerationOverview, req *ExecuteGenerationActionRequest) (*AssetGenerationActionTarget, string, error) {
	actionKey := requestedAssetGenerationActionKey(req)
	if actionKey == "" {
		return nil, "", fmt.Errorf("%w: missing action key", ErrGenerationActionNotFound)
	}
	if !isAllowedAssetGenerationActionKey(actionKey) {
		return nil, "", fmt.Errorf("%w: %s", ErrGenerationActionNotFound, actionKey)
	}
	if overview != nil {
		for _, candidate := range collectAssetGenerationActionTargets(overview) {
			if candidate == nil {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(candidate.ActionKey), actionKey) {
				return cloneAssetGenerationActionTarget(candidate), "overview", nil
			}
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

func cloneAssetGenerationActionTarget(target *AssetGenerationActionTarget) *AssetGenerationActionTarget {
	if target == nil {
		return nil
	}
	cloned := *target
	cloned.Filters = cloneAssetGenerationFilters(target.Filters)
	cloned.QueueQuery = cloneGenerationQueueQuery(target.QueueQuery)
	cloned.RetryRequest = cloneRetryGenerationTasksRequest(target.RetryRequest)
	cloned.ExpectedImpact = cloneAssetGenerationActionImpact(target.ExpectedImpact)
	cloned.NavigationTarget = cloneGenerationReviewNavigationTarget(target.NavigationTarget)
	return &cloned
}

func cloneAssetGenerationActionImpact(impact *AssetGenerationActionImpact) *AssetGenerationActionImpact {
	if impact == nil {
		return nil
	}
	cloned := *impact
	cloned.Platforms = append([]string(nil), impact.Platforms...)
	cloned.QualityGrades = append([]string(nil), impact.QualityGrades...)
	cloned.States = append([]string(nil), impact.States...)
	return &cloned
}

func cloneGenerationQueueQuery(query *GenerationQueueQuery) *GenerationQueueQuery {
	if query == nil {
		return nil
	}
	cloned := *query
	return &cloned
}

func cloneRetryGenerationTasksRequest(req *RetryGenerationTasksRequest) *RetryGenerationTasksRequest {
	if req == nil {
		return nil
	}
	cloned := *req
	cloned.TaskIDs = append([]string(nil), req.TaskIDs...)
	cloned.Slots = append([]string(nil), req.Slots...)
	return &cloned
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
