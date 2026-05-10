package listingkit

import (
	"context"
	"fmt"
	"strings"
)

func (s *service) DispatchTaskGenerationNavigation(ctx context.Context, taskID string, req *GenerationReviewNavigationDispatchRequest) (*GenerationReviewNavigationDispatchResponse, error) {
	if req == nil || req.Target == nil {
		return nil, fmt.Errorf("%w: missing navigation target", ErrGenerationActionNotFound)
	}
	target := cloneGenerationReviewNavigationTarget(req.Target)
	ApplyGenerationConditionalBaselineToNavigationTarget(target, "")

	responseMode := normalizeGenerationActionResponseMode(req.ResponseMode)
	planMode := normalizeGenerationNavigationDispatchPlanMode(req.PlanMode)
	response, err := s.dispatchGenerationNavigationPrimary(ctx, taskID, target, responseMode)
	if err != nil {
		return nil, err
	}
	response.PlanMode = planMode
	if planMode == "execute_plan" {
		executedPlan, err := s.executeGenerationNavigationDispatchPlan(ctx, taskID, target, responseMode)
		if err != nil {
			return nil, err
		}
		applyExecutedPlanToDispatchResponse(response, executedPlan)
	}
	return finalizeGenerationReviewNavigationDispatchResponse(response), nil
}

func (s *service) dispatchGenerationNavigationPrimary(ctx context.Context, taskID string, target *GenerationReviewNavigationTarget, responseMode string) (*GenerationReviewNavigationDispatchResponse, error) {
	switch normalizeGenerationReviewDispatchKind(target) {
	case "action":
		actionReq := &ExecuteGenerationActionRequest{
			ResponseMode: responseMode,
			Target:       cloneAssetGenerationActionTarget(target.ActionTarget),
		}
		if actionReq.Target == nil {
			return nil, fmt.Errorf("%w: missing action target", ErrGenerationActionNotFound)
		}
		actionReq.ActionKey = actionReq.Target.ActionKey
		action, err := s.ExecuteTaskGenerationAction(ctx, taskID, actionReq)
		if err != nil {
			return nil, err
		}
		return &GenerationReviewNavigationDispatchResponse{
			TaskID:       taskID,
			DispatchKind: "action",
			ResponseMode: responseMode,
			DeltaToken:   action.DeltaToken,
			Action:       action,
		}, nil
	case "preview":
		preview, err := s.GetTaskGenerationReviewPreview(ctx, taskID, cloneGenerationQueueQuery(target.PreviewQuery))
		if err != nil {
			return nil, err
		}
		return &GenerationReviewNavigationDispatchResponse{
			TaskID:        taskID,
			DispatchKind:  "preview",
			ResponseMode:  responseMode,
			DeltaToken:    preview.DeltaToken,
			ReviewPreview: preview,
		}, nil
	case "queue":
		queue, err := s.GetTaskGenerationQueue(ctx, taskID, cloneGenerationQueueQuery(target.QueueQuery))
		if err != nil {
			return nil, err
		}
		return &GenerationReviewNavigationDispatchResponse{
			TaskID:       taskID,
			DispatchKind: "queue",
			ResponseMode: responseMode,
			DeltaToken:   queue.DeltaToken,
			Queue:        queue,
		}, nil
	default:
		sessionQuery := cloneGenerationQueueQuery(target.SessionQuery)
		if sessionQuery == nil {
			sessionQuery = cloneGenerationQueueQuery(target.QueueQuery)
		}
		if sessionQuery == nil {
			sessionQuery = cloneGenerationQueueQuery(target.PreviewQuery)
		}
		if sessionQuery != nil && strings.TrimSpace(responseMode) != "" {
			sessionQuery.ResponseMode = responseMode
		}
		session, err := s.GetTaskGenerationReviewSession(ctx, taskID, sessionQuery)
		if err != nil {
			return nil, err
		}
		return &GenerationReviewNavigationDispatchResponse{
			TaskID:        taskID,
			DispatchKind:  "session",
			ResponseMode:  responseMode,
			DeltaToken:    session.DeltaToken,
			ReviewSession: session,
		}, nil
	}
}
