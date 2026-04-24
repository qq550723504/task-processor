package listingkit

import (
	"strings"
	"time"

	assetgeneration "task-processor/internal/asset/generation"
)

type generationReviewStateKey struct {
	Platform   string
	Slot       string
	Capability string
}

type generationReviewStateRecord struct {
	Key        generationReviewStateKey
	Current    bool
	Pending    bool
	Record     *GenerationReviewRecord
	AssetID    string
	AssetRev   string
	PreviewRev string
	TaskRev    string
}

type generationReviewState struct {
	ByKey       map[generationReviewStateKey]generationReviewStateRecord
	SlotSummary map[string]generationReviewSlotState
	Summary     *GenerationReviewSummary
}

type generationReviewSlotState struct {
	Decision   string
	Status     string
	Blocked    bool
	ReviewedAt *time.Time
}

func withListingKitResultGenerationAndReview(result *ListingKitResult, tasks []assetgeneration.Task, reviews []GenerationReviewRecord) *ListingKitResult {
	cloned := withListingKitResultGeneration(result, tasks)
	if cloned == nil {
		return nil
	}
	decorateListingKitResultReview(cloned, reviews)
	return cloned
}

func decorateListingKitResultReview(result *ListingKitResult, reviews []GenerationReviewRecord) {
	if result == nil {
		return
	}
	syncAssetRenderPreviews(result)
	result.ReviewRecords = append([]GenerationReviewRecord(nil), reviews...)
	state := buildGenerationReviewState(result.AssetGenerationQueue, result.PlatformAssetRenderPreviews, reviews)
	if state == nil {
		return
	}
	result.ReviewSummary = cloneGenerationReviewSummary(state.Summary)
	applyReviewStateToQueue(result.AssetGenerationQueue, state)
	if result.AssetGenerationQueue != nil {
		result.AssetGenerationOverview = buildAssetGenerationOverview(result.AssetGenerationQueue)
	}
}

func buildGenerationReviewState(queue *GenerationWorkQueue, previews []PlatformAssetRenderPreviews, reviews []GenerationReviewRecord) *generationReviewState {
	current := collectCurrentGenerationReviewKeys(queue, previews)
	if len(current) == 0 && len(reviews) == 0 {
		return nil
	}
	state := &generationReviewState{
		ByKey:       make(map[generationReviewStateKey]generationReviewStateRecord, len(current)),
		SlotSummary: map[string]generationReviewSlotState{},
		Summary: &GenerationReviewSummary{
			SectionCounts: map[string]int{},
		},
	}
	for key, item := range current {
		item.Pending = true
		state.ByKey[key] = item
	}
	for i := range reviews {
		record := reviews[i]
		key := generationReviewStateKey{
			Platform:   normalizeReviewKey(record.Platform),
			Slot:       normalizeReviewKey(record.Slot),
			Capability: normalizeReviewKey(record.Capability),
		}
		currentItem, ok := state.ByKey[key]
		if !ok {
			continue
		}
		if !generationReviewRecordMatchesCurrent(record, currentItem) {
			continue
		}
		currentItem.Pending = false
		currentItem.Current = true
		copyRecord := record
		currentItem.Record = &copyRecord
		state.ByKey[key] = currentItem
	}
	slotStates := map[string][]generationReviewStateRecord{}
	for key, item := range state.ByKey {
		slotKey := key.Platform + ":" + key.Slot
		slotStates[slotKey] = append(slotStates[slotKey], item)
		state.Summary.SectionCounts[key.Capability]++
		switch {
		case item.Pending:
			state.Summary.ReviewPendingSections++
		case item.Record != nil && item.Record.Decision == GenerationReviewDecisionApprove:
			state.Summary.ApprovedSections++
		case item.Record != nil && item.Record.Decision == GenerationReviewDecisionDefer:
			state.Summary.DeferredSections++
		}
		state.Summary.Platforms = append(state.Summary.Platforms, key.Platform)
	}
	state.Summary.Platforms = uniqueStrings(state.Summary.Platforms)
	for slotKey, items := range slotStates {
		state.SlotSummary[slotKey] = buildGenerationReviewSlotState(items)
	}
	return state
}

func collectCurrentGenerationReviewKeys(queue *GenerationWorkQueue, previews []PlatformAssetRenderPreviews) map[generationReviewStateKey]generationReviewStateRecord {
	out := map[generationReviewStateKey]generationReviewStateRecord{}
	if queue != nil {
		for _, item := range queue.Items {
			if len(item.PreviewCapabilities) == 0 {
				continue
			}
			for _, capability := range item.PreviewCapabilities {
				key := generationReviewStateKey{
					Platform:   normalizeReviewKey(item.Platform),
					Slot:       normalizeReviewKey(item.Slot),
					Capability: normalizeReviewKey(capability),
				}
				out[key] = generationReviewStateRecord{
					Key:     key,
					AssetID: strings.TrimSpace(firstNonEmpty(item.SelectedAssetID, item.AssetID)),
				}
			}
		}
	}
	for _, group := range previews {
		for _, slot := range flattenPlatformRenderPreviewSlots(group) {
			item, _ := findGenerationQueueItemByPlatformSlot(queue, group.Platform, slot.Slot)
			capabilities := item.PreviewCapabilities
			if len(capabilities) == 0 {
				capabilities = buildRenderPreviewCapabilities(GenerationWorkQueueItem{
					RenderPreviewLayerTypes: append([]string(nil), slot.LayerTypes...),
				})
			}
			for _, capability := range capabilities {
				key := generationReviewStateKey{
					Platform:   normalizeReviewKey(group.Platform),
					Slot:       normalizeReviewKey(slot.Slot),
					Capability: normalizeReviewKey(capability),
				}
				out[key] = generationReviewStateRecord{
					Key:        key,
					AssetID:    strings.TrimSpace(slot.AssetID),
					AssetRev:   strings.TrimSpace(slot.AssetRevision),
					PreviewRev: strings.TrimSpace(slot.PreviewRevision),
					TaskRev:    strings.TrimSpace(slot.TaskRevision),
				}
			}
		}
	}
	return out
}

func findGenerationQueueItemByPlatformSlot(queue *GenerationWorkQueue, platform, slot string) (GenerationWorkQueueItem, bool) {
	if queue == nil {
		return GenerationWorkQueueItem{}, false
	}
	for _, item := range queue.Items {
		if normalizeReviewKey(item.Platform) == normalizeReviewKey(platform) && normalizeReviewKey(item.Slot) == normalizeReviewKey(slot) {
			return item, true
		}
	}
	return GenerationWorkQueueItem{}, false
}

func generationReviewRecordMatchesCurrent(record GenerationReviewRecord, current generationReviewStateRecord) bool {
	if current.AssetID != "" && strings.TrimSpace(record.AssetID) != "" && strings.TrimSpace(record.AssetID) != current.AssetID {
		return false
	}
	if current.AssetRev != "" && strings.TrimSpace(record.AssetRevision) != "" && strings.TrimSpace(record.AssetRevision) != current.AssetRev {
		return false
	}
	if current.PreviewRev != "" && strings.TrimSpace(record.PreviewRevision) != "" && strings.TrimSpace(record.PreviewRevision) != current.PreviewRev {
		return false
	}
	if current.TaskRev != "" && strings.TrimSpace(record.TaskRevision) != "" && strings.TrimSpace(record.TaskRevision) != current.TaskRev {
		return false
	}
	return true
}

func buildGenerationReviewSlotState(items []generationReviewStateRecord) generationReviewSlotState {
	state := generationReviewSlotState{Status: "pending"}
	allApproved := len(items) > 0
	var latest *time.Time
	for _, item := range items {
		if item.Record != nil && !item.Record.ReviewedAt.IsZero() {
			reviewedAt := item.Record.ReviewedAt
			if latest == nil || reviewedAt.After(*latest) {
				latest = &reviewedAt
			}
		}
		if item.Pending {
			allApproved = false
			continue
		}
		if item.Record == nil {
			allApproved = false
			continue
		}
		switch item.Record.Decision {
		case GenerationReviewDecisionDefer:
			state.Decision = string(GenerationReviewDecisionDefer)
			state.Status = "deferred"
			state.Blocked = true
			state.ReviewedAt = latest
			return state
		case GenerationReviewDecisionApprove:
			state.Decision = string(GenerationReviewDecisionApprove)
		default:
			allApproved = false
		}
	}
	if allApproved && state.Decision == string(GenerationReviewDecisionApprove) {
		state.Status = "approved"
	}
	state.ReviewedAt = latest
	return state
}

func applyReviewStateToQueue(queue *GenerationWorkQueue, state *generationReviewState) {
	if queue == nil || state == nil {
		return
	}
	for i := range queue.Items {
		slotKey := normalizeReviewKey(queue.Items[i].Platform) + ":" + normalizeReviewKey(queue.Items[i].Slot)
		slotState, ok := state.SlotSummary[slotKey]
		if !ok {
			continue
		}
		queue.Items[i].ReviewDecision = slotState.Decision
		queue.Items[i].ReviewStatus = slotState.Status
		queue.Items[i].ReviewBlocked = slotState.Blocked
		queue.Items[i].ReviewedAt = slotState.ReviewedAt
	}
	if queue.Summary == nil {
		queue.Summary = buildGenerationWorkQueueSummary(queue.Items)
	}
	if queue.Summary != nil && state.Summary != nil {
		queue.Summary.ApprovedSections = state.Summary.ApprovedSections
		queue.Summary.DeferredSections = state.Summary.DeferredSections
		queue.Summary.ReviewPendingSections = state.Summary.ReviewPendingSections
	}
}

func cloneGenerationReviewSummary(summary *GenerationReviewSummary) *GenerationReviewSummary {
	if summary == nil {
		return nil
	}
	cloned := *summary
	cloned.Platforms = append([]string(nil), summary.Platforms...)
	if len(summary.SectionCounts) > 0 {
		cloned.SectionCounts = make(map[string]int, len(summary.SectionCounts))
		for key, value := range summary.SectionCounts {
			cloned.SectionCounts[key] = value
		}
	}
	return &cloned
}

func normalizeReviewKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
