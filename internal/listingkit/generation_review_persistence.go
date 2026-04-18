package listingkit

import (
	"context"
	"strings"
	"time"

	"task-processor/internal/listingkit/reviewstore"
)

func (s *service) persistGenerationReviewDecision(ctx context.Context, taskID string, actionKey string, session *GenerationReviewSession, target *AssetGenerationActionTarget) (*GenerationReviewRecord, error) {
	if s.reviewRepo == nil || !isPersistedGenerationReviewAction(actionKey) {
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
	if err := s.reviewRepo.SaveReview(ctx, &reviewstore.ReviewRecord{
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
	platform := targetPlatform(target)
	slot := targetSlot(target)
	capability := targetCapability(target)
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
	} else if slotState := findGenerationReviewSlot(session, platform, slot, capability); slotState != nil {
		record.AssetID = slotState.AssetID
		record.AssetRevision = slotState.AssetRevision
		record.PreviewRevision = slotState.PreviewRevision
		record.TaskRevision = slotState.TaskRevision
	}
	return record
}

func isPersistedGenerationReviewAction(actionKey string) bool {
	switch strings.TrimSpace(actionKey) {
	case "approve_section_review", "defer_section_review":
		return true
	default:
		return false
	}
}

func generationReviewDecisionFromAction(actionKey string) GenerationReviewDecision {
	switch strings.TrimSpace(actionKey) {
	case "approve_section_review":
		return GenerationReviewDecisionApprove
	case "defer_section_review":
		return GenerationReviewDecisionDefer
	default:
		return ""
	}
}

func generationReviewStatusFromDecision(decision GenerationReviewDecision) string {
	switch decision {
	case GenerationReviewDecisionApprove:
		return "approved"
	case GenerationReviewDecisionDefer:
		return "deferred"
	default:
		return "pending"
	}
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

func targetPlatform(target *AssetGenerationActionTarget) string {
	if target == nil || target.QueueQuery == nil {
		return ""
	}
	return target.QueueQuery.Platform
}

func targetSlot(target *AssetGenerationActionTarget) string {
	if target == nil || target.QueueQuery == nil {
		return ""
	}
	return target.QueueQuery.Slot
}

func targetCapability(target *AssetGenerationActionTarget) string {
	if target == nil || target.QueueQuery == nil {
		return ""
	}
	return target.QueueQuery.PreviewCapability
}

func findGenerationReviewSlot(session *GenerationReviewSession, platform, slot, capability string) *GenerationReviewSlot {
	if session == nil {
		return nil
	}
	for i := range session.SlotNavigation {
		item := &session.SlotNavigation[i]
		if normalizeReviewKey(item.Platform) != normalizeReviewKey(platform) || normalizeReviewKey(item.Slot) != normalizeReviewKey(slot) {
			continue
		}
		if capability == "" || item.FocusCapability == capability || len(item.PreviewCapabilities) == 0 {
			return item
		}
		for _, previewCapability := range item.PreviewCapabilities {
			if previewCapability == capability {
				return item
			}
		}
	}
	return nil
}
