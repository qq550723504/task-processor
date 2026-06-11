package listingkit

import (
	"context"
	"strings"
	"time"
)

func (s *service) decorateSheinStoreResolutionPreview(ctx context.Context, task *Task, preview *ListingKitPreview) {
	if s == nil || task == nil || preview == nil || preview.Shein == nil {
		return
	}
	preview.Shein.SubmissionEvents = sheinSubmissionEventsWithStoreResolution(preview.Shein.SubmissionEvents, task)
	selection, err := s.resolveSheinStoreSelection(ctx, task)
	if err != nil || selection == nil || selection.Profile == nil {
		return
	}
	summary := buildSheinStoreResolutionSummary(selection, task, preview)
	preview.Shein.StoreResolution = summary
	if preview.Shein.FinalReview != nil {
		if preview.Shein.FinalReview.StoreID <= 0 {
			preview.Shein.FinalReview.StoreID = summary.StoreID
		}
		if strings.TrimSpace(preview.Shein.FinalReview.Site) == "" {
			preview.Shein.FinalReview.Site = summary.Site
		}
	}
}

func buildSheinStoreResolutionSummary(selection *sheinStoreSelection, task *Task, preview *ListingKitPreview) *SheinStoreResolutionSummary {
	if selection == nil || selection.Profile == nil {
		return nil
	}
	var matchedProfileID int64
	var resolvedAt string
	if snapshot := sheinStoreResolutionSnapshotFromTask(task); snapshot != nil {
		matchedProfileID = snapshot.MatchedProfileID
		if !snapshot.ResolvedAt.IsZero() {
			resolvedAt = snapshot.ResolvedAt.UTC().Format(time.RFC3339)
		}
	}
	if matchedProfileID <= 0 {
		matchedProfileID = selection.Profile.ID
	}
	return buildSheinStoreResolutionSummaryValue(
		selection.Profile.StoreID,
		firstNonEmpty(strings.TrimSpace(selection.Profile.Site), sheinPreviewSite(selection.Profile, task, preview)),
		selection.Strategy,
		selection.Reason,
		selection.MatchedRuleKinds,
		matchedProfileID,
		selection.ManualOverride,
		selection.Fallback,
		resolvedAt,
	)
}

func sheinStoreResolutionSummaryFromSnapshot(snapshot *SheinStoreResolutionSnapshot) *SheinStoreResolutionSummary {
	if snapshot == nil || snapshot.StoreID <= 0 {
		return nil
	}
	var resolvedAt string
	if !snapshot.ResolvedAt.IsZero() {
		resolvedAt = snapshot.ResolvedAt.UTC().Format(time.RFC3339)
	}
	return buildSheinStoreResolutionSummaryValue(
		snapshot.StoreID,
		snapshot.Site,
		snapshot.Strategy,
		snapshot.Reason,
		snapshot.MatchedRuleKinds,
		snapshot.MatchedProfileID,
		snapshot.ManualOverride,
		snapshot.Fallback,
		resolvedAt,
	)
}

func buildSheinStoreResolutionSummaryValue(
	storeID int64,
	site string,
	strategy string,
	reason string,
	matchedRuleKinds []string,
	matchedProfileID int64,
	manualOverride bool,
	fallback bool,
	resolvedAt string,
) *SheinStoreResolutionSummary {
	return &SheinStoreResolutionSummary{
		StoreID:          storeID,
		Site:             site,
		Strategy:         strategy,
		Reason:           reason,
		MatchedRuleKinds: append([]string(nil), matchedRuleKinds...),
		MatchedProfileID: matchedProfileID,
		ManualOverride:   manualOverride,
		Fallback:         fallback,
		ResolvedAt:       resolvedAt,
	}
}

func sheinStoreResolutionSnapshotFromSelection(selection *sheinStoreSelection, task *Task, preview *ListingKitPreview) *SheinStoreResolutionSnapshot {
	if selection == nil || selection.Profile == nil {
		return nil
	}
	summary := buildSheinStoreResolutionSummary(selection, task, preview)
	if summary == nil {
		return nil
	}
	return &SheinStoreResolutionSnapshot{
		StoreID:           summary.StoreID,
		Site:              summary.Site,
		WarehouseCode:     selection.Profile.WarehouseCode,
		DefaultStock:      selection.Profile.DefaultStock,
		DefaultSubmitMode: selection.Profile.DefaultSubmitMode,
		Pricing:           selection.Profile.Pricing,
		Strategy:          summary.Strategy,
		Reason:            summary.Reason,
		MatchedRuleKinds:  append([]string(nil), summary.MatchedRuleKinds...),
		MatchedProfileID:  selection.Profile.ID,
		ManualOverride:    summary.ManualOverride,
		Fallback:          summary.Fallback,
		ResolvedAt:        time.Now(),
	}
}

func sheinPreviewSite(profile *ListingKitStoreProfile, task *Task, preview *ListingKitPreview) string {
	if profile != nil && strings.TrimSpace(profile.Site) != "" {
		return profile.Site
	}
	if preview != nil && preview.Shein != nil && len(preview.Shein.CategoryPath) > 0 {
		if preview.Shein.FinalReview != nil && strings.TrimSpace(preview.Shein.FinalReview.Site) != "" {
			return preview.Shein.FinalReview.Site
		}
	}
	if task != nil && task.Request != nil {
		return strings.ToUpper(strings.TrimSpace(task.Request.Country))
	}
	return ""
}
