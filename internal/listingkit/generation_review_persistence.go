package listingkit

import (
	"context"
	"strings"
	"time"

	listinggeneration "task-processor/internal/listingkit/generation"
	"task-processor/internal/listingkit/reviewstore"
)

func (s *service) persistGenerationReviewDecision(ctx context.Context, taskID string, actionKey string, session *GenerationReviewSession, target *AssetGenerationActionTarget) (*GenerationReviewRecord, error) {
	reviewRepo := resolveReviewRepository(s)
	if reviewRepo == nil || !isPersistedGenerationReviewAction(actionKey) {
		return nil, nil
	}
	record := buildGenerationReviewRecord(taskID, actionKey, session, target)
	if record == nil && target != nil && target.QueueQuery != nil {
		record = &GenerationReviewRecord{
			TaskID:          taskID,
			Platform:        strings.TrimSpace(target.QueueQuery.Platform),
			Slot:            strings.TrimSpace(target.QueueQuery.Slot),
			Capability:      strings.TrimSpace(target.QueueQuery.PreviewCapability),
			Decision:        generationReviewDecisionFromAction(actionKey),
			Status:          generationReviewStatusFromDecision(generationReviewDecisionFromAction(actionKey)),
			Message:         generationReviewWorkflowMessage(actionKey, target.QueueQuery.Platform, target.QueueQuery.Slot, target.QueueQuery.PreviewCapability),
			ReviewedAt:      time.Now().UTC(),
			ReviewedBy:      "listingkit",
			SourceActionKey: actionKey,
		}
	}
	if record == nil {
		return nil, nil
	}
	if err := reviewRepo.SaveReview(ctx, &reviewstore.ReviewRecord{
		TaskID:          record.TaskID,
		Platform:        record.Platform,
		Slot:            record.Slot,
		Capability:      record.Capability,
		Decision:        string(record.Decision),
		Status:          record.Status,
		Message:         record.Message,
		ReviewedAt:      record.ReviewedAt,
		ReviewedBy:      record.ReviewedBy,
		AssetID:         record.AssetID,
		AssetRevision:   record.AssetRevision,
		PreviewRevision: record.PreviewRevision,
		TaskRevision:    record.TaskRevision,
		SourceActionKey: record.SourceActionKey,
	}); err != nil {
		return nil, err
	}
	return record, nil
}

func buildGenerationReviewRecord(taskID string, actionKey string, session *GenerationReviewSession, target *AssetGenerationActionTarget) *GenerationReviewRecord {
	decision := generationReviewDecisionFromAction(actionKey)
	if decision == "" {
		return nil
	}
	platform := ""
	slot := ""
	capability := ""
	if target != nil && target.QueueQuery != nil {
		platform = target.QueueQuery.Platform
		slot = target.QueueQuery.Slot
		capability = target.QueueQuery.PreviewCapability
	}
	if session != nil {
		platform = firstNonEmpty(session.SelectedPlatform, platform)
		slot = firstNonEmpty(session.SelectedSlot, slot)
		capability = firstNonEmpty(session.FocusCapability, capability)
	}
	if platform == "" || slot == "" || capability == "" {
		return nil
	}
	var preview *AssetRenderPreviewSlot
	if session != nil {
		preview = session.FocusedRenderPreview
	}
	now := time.Now().UTC()
	record := &GenerationReviewRecord{
		TaskID:          taskID,
		Platform:        platform,
		Slot:            slot,
		Capability:      capability,
		Decision:        decision,
		Status:          generationReviewStatusFromDecision(decision),
		Message:         generationReviewWorkflowMessage(actionKey, platform, slot, capability),
		ReviewedAt:      now,
		ReviewedBy:      "listingkit",
		SourceActionKey: actionKey,
	}
	if preview != nil {
		record.AssetID = preview.AssetID
		record.AssetRevision = preview.AssetRevision
		record.PreviewRevision = preview.PreviewRevision
		record.TaskRevision = preview.TaskRevision
	} else if session != nil {
		for i := range session.SlotNavigation {
			item := &session.SlotNavigation[i]
			if normalizeReviewKey(item.Platform) != normalizeReviewKey(platform) || normalizeReviewKey(item.Slot) != normalizeReviewKey(slot) {
				continue
			}
			if capability != "" && item.FocusCapability != capability && len(item.PreviewCapabilities) > 0 {
				matched := false
				for _, previewCapability := range item.PreviewCapabilities {
					if previewCapability == capability {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
			record.AssetID = item.AssetID
			record.AssetRevision = item.AssetRevision
			record.PreviewRevision = item.PreviewRevision
			record.TaskRevision = item.TaskRevision
			break
		}
	}
	return record
}

func isPersistedGenerationReviewAction(actionKey string) bool {
	return listinggeneration.IsPersistedReviewAction(actionKey)
}

func generationReviewDecisionFromAction(actionKey string) GenerationReviewDecision {
	return GenerationReviewDecision(listinggeneration.ReviewDecisionFromAction(actionKey))
}

func generationReviewStatusFromDecision(decision GenerationReviewDecision) string {
	return listinggeneration.ReviewStatusFromDecision(string(decision))
}

func generationReviewWorkflowMessage(actionKey, platform, slot, capability string) string {
	target := &AssetGenerationActionTarget{QueueQuery: &GenerationQueueQuery{
		Platform:          platform,
		Slot:              slot,
		PreviewCapability: capability,
	}}
	if workflow := buildGenerationReviewWorkflowResult(actionKey, target); workflow != nil {
		return workflow.Message
	}
	return ""
}
