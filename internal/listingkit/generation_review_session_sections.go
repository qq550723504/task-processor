package listingkit

func buildGenerationReviewSession(result *ListingKitResult, queue *GenerationWorkQueue, query *GenerationQueueQuery) *GenerationReviewSession {
	if result == nil && queue == nil {
		return nil
	}
	selectedPlatform := ""
	if query != nil {
		selectedPlatform = query.Platform
	}
	platformRenderPreviews := buildActionPlatformRenderPreviews(result, query)
	sessionResult := &ListingKitResult{}
	if result != nil {
		*sessionResult = *result
	}
	sessionResult.AssetGenerationQueue = queue
	sessionResult.PlatformAssetRenderPreviews = platformRenderPreviews
	reviewQueue := queue
	if reviewQueue == nil && result != nil {
		reviewQueue = result.AssetGenerationQueue
	}
	reviewQueue = cloneGenerationWorkQueue(reviewQueue)
	if selectedPlatform == "" {
		selectedPlatform = detectReviewSessionPlatform(reviewQueue, platformRenderPreviews)
	}
	reviewState := buildGenerationReviewState(reviewQueue, platformRenderPreviews, sessionResult.ReviewRecords)
	applyReviewStateToQueue(reviewQueue, reviewState)
	slotNavigation := buildGenerationReviewSlots(reviewQueue, selectedPlatform, platformRenderPreviews)
	applyReviewStateToReviewSlots(slotNavigation, reviewState)
	selectedSlot := detectReviewSessionSlot(slotNavigation, query)
	focusCapability := detectReviewSessionCapability(query, slotNavigation, platformRenderPreviews, reviewState)
	enrichGenerationReviewSlotsWithFocus(slotNavigation, platformRenderPreviews, focusCapability)
	sections := buildGenerationReviewSections(reviewQueue, selectedPlatform, focusCapability, platformRenderPreviews, reviewState)
	markSelectedReviewSlots(slotNavigation, selectedSlot)
	markSelectedReviewSections(sections, focusCapability)
	platformCards := buildPlatformPreviewCards(sessionResult, selectedPlatform)
	attachReviewTargetsToSlots(slotNavigation, focusCapability)
	attachReviewTargetsToSections(sections)
	attachReviewTargetsToPlatformCards(platformCards, selectedSlot, focusCapability)
	focusedTarget, focusedRenderPreview, focusedSectionKey := buildGenerationReviewSessionFocus(platformRenderPreviews, selectedPlatform, selectedSlot, focusCapability)
	enrichReviewTargetsWithContext(slotNavigation, sections, platformCards, selectedPlatform, selectedSlot, focusCapability, focusedSectionKey, focusedRenderPreview)
	focusedToolbar := buildGenerationReviewToolbarInput(reviewQueue, platformRenderPreviews, slotNavigation, selectedPlatform, selectedSlot, focusCapability)
	defaultTarget := buildGenerationReviewTarget(selectedPlatform, selectedSlot, focusCapability)
	if defaultTarget != nil && defaultTarget.PanelState != nil && focusedRenderPreview != nil {
		defaultTarget.PanelState.FocusedPreviewAssetID = focusedRenderPreview.AssetID
	}
	defaultTarget = enrichGenerationReviewTargetWithContext(defaultTarget, selectedPlatform, selectedSlot, focusCapability, focusedSectionKey, focusedRenderPreview)
	focusedTarget = enrichGenerationReviewTargetWithContext(focusedTarget, selectedPlatform, selectedSlot, focusCapability, focusedSectionKey, focusedRenderPreview)
	return &GenerationReviewSession{
		SelectedPlatform:       selectedPlatform,
		SelectedSlot:           selectedSlot,
		FocusCapability:        focusCapability,
		FocusedSectionKey:      focusedSectionKey,
		DefaultTarget:          defaultTarget,
		FocusedTarget:          focusedTarget,
		FocusedRenderPreview:   focusedRenderPreview,
		FocusedScenePreset:     buildGenerationScenePresetSummary(sessionResult.AssetBundle, focusedPreviewAssetID(focusedRenderPreview)),
		FocusedToolbar:         focusedToolbar,
		Queue:                  reviewQueue,
		Overview:               buildAssetGenerationOverview(reviewQueue),
		ReviewSummary:          cloneGenerationReviewSummary(sessionResult.ReviewSummary),
		PlatformCards:          platformCards,
		PlatformRenderPreviews: platformRenderPreviews,
		SlotNavigation:         slotNavigation,
		Sections:               sections,
	}
}

func focusedPreviewAssetID(preview *AssetRenderPreviewSlot) string {
	if preview == nil {
		return ""
	}
	return preview.AssetID
}
