package listingkit

import (
	"context"
	"fmt"
	"strings"
)

type taskGenerationNavigationDispatchPrimaryPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationNavigationDispatchPrimaryPhase(service *taskGenerationService) *taskGenerationNavigationDispatchPrimaryPhase {
	return &taskGenerationNavigationDispatchPrimaryPhase{service: service}
}

func (p *taskGenerationNavigationDispatchPrimaryPhase) run(ctx context.Context, taskID string, target *GenerationReviewNavigationTarget, responseMode string) (*GenerationReviewNavigationDispatchResponse, error) {
	switch normalizeGenerationReviewDispatchKind(target) {
	case "action":
		actionTarget := cloneAssetGenerationActionTarget(target.ActionTarget)
		if actionTarget == nil {
			return nil, fmt.Errorf("%w: missing action target", ErrGenerationActionNotFound)
		}
		actionReq := &ExecuteGenerationActionRequest{
			ResponseMode: responseMode,
			Target:       actionTarget,
			ActionKey:    actionTarget.ActionKey,
		}
		action, err := p.service.ExecuteTaskGenerationAction(ctx, taskID, actionReq)
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
		preview, err := p.service.GetTaskGenerationReviewPreview(ctx, taskID, cloneGenerationQueueQuery(target.PreviewQuery))
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
		queue, err := p.service.GetTaskGenerationQueue(ctx, taskID, cloneGenerationQueueQuery(target.QueueQuery))
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
		sessionQuery := resolveTaskGenerationNavigationPrimarySessionQuery(target)
		if sessionQuery != nil && strings.TrimSpace(responseMode) != "" {
			sessionQuery.ResponseMode = responseMode
		}
		session, err := p.service.GetTaskGenerationReviewSession(ctx, taskID, sessionQuery)
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

func resolveTaskGenerationNavigationPrimarySessionQuery(target *GenerationReviewNavigationTarget) *GenerationQueueQuery {
	if target == nil {
		return nil
	}
	sessionQuery := cloneGenerationQueueQuery(target.SessionQuery)
	if sessionQuery == nil {
		sessionQuery = cloneGenerationQueueQuery(target.QueueQuery)
	}
	if sessionQuery == nil {
		sessionQuery = cloneGenerationQueueQuery(target.PreviewQuery)
	}
	return sessionQuery
}
