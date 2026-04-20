package listingkit

import "strings"

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

func detectReviewSessionPlatform(queue *GenerationWorkQueue, previews []PlatformAssetRenderPreviews) string {
	if queue != nil {
		for _, item := range queue.Items {
			if item.Platform != "" {
				return item.Platform
			}
		}
	}
	for _, group := range previews {
		if group.Platform != "" {
			return group.Platform
		}
	}
	return ""
}

func buildGenerationReviewSlots(queue *GenerationWorkQueue, selectedPlatform string, previews []PlatformAssetRenderPreviews) []GenerationReviewSlot {
	out := make([]GenerationReviewSlot, 0, 8)
	slotIndex := map[string]int{}
	if queue != nil {
		for _, item := range queue.Items {
			if selectedPlatform != "" && item.Platform != selectedPlatform {
				continue
			}
			slot := GenerationReviewSlot{
				Platform:               item.Platform,
				Slot:                   item.Slot,
				Purpose:                item.Purpose,
				State:                  item.State,
				QualityGrade:           item.QualityGrade,
				QualityGradeLabel:      item.QualityGradeLabel,
				AssetID:                firstNonEmpty(item.SelectedAssetID, item.AssetID),
				TemplateLabel:          item.TemplateLabel,
				RenderPreviewAvailable: item.RenderPreviewAvailable,
				PreviewCapabilities:    append([]string(nil), item.PreviewCapabilities...),
			}
			key := item.Platform + ":" + item.Slot
			slotIndex[key] = len(out)
			out = append(out, slot)
		}
	}
	for _, group := range previews {
		if selectedPlatform != "" && group.Platform != selectedPlatform {
			continue
		}
		for _, slot := range flattenPlatformRenderPreviewSlots(group) {
			key := group.Platform + ":" + slot.Slot
			capabilities := buildRenderPreviewCapabilities(GenerationWorkQueueItem{RenderPreviewLayerTypes: append([]string(nil), slot.LayerTypes...)})
			if idx, ok := slotIndex[key]; ok {
				out[idx].RenderPreviewAvailable = true
				if out[idx].AssetID == "" {
					out[idx].AssetID = slot.AssetID
				}
				if out[idx].TemplateLabel == "" {
					out[idx].TemplateLabel = slot.TemplateLabel
				}
				if len(out[idx].PreviewCapabilities) == 0 {
					out[idx].PreviewCapabilities = capabilities
				}
				continue
			}
			out = append(out, GenerationReviewSlot{
				Platform:               group.Platform,
				Slot:                   slot.Slot,
				Purpose:                slot.Purpose,
				AssetID:                slot.AssetID,
				TemplateLabel:          slot.TemplateLabel,
				RenderPreviewAvailable: true,
				PreviewCapabilities:    capabilities,
			})
			slotIndex[key] = len(out) - 1
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func flattenPlatformRenderPreviewSlots(group PlatformAssetRenderPreviews) []AssetRenderPreviewSlot {
	out := make([]AssetRenderPreviewSlot, 0, 1+len(group.Gallery)+len(group.Auxiliary))
	if group.Main != nil {
		out = append(out, *group.Main)
	}
	out = append(out, group.Gallery...)
	out = append(out, group.Auxiliary...)
	return out
}

func detectReviewSessionSlot(slots []GenerationReviewSlot, query *GenerationQueueQuery) string {
	if query != nil {
		targetSlot := strings.TrimSpace(query.Slot)
		if targetSlot != "" {
			for _, slot := range slots {
				if slot.Slot == targetSlot {
					return slot.Slot
				}
			}
		}
	}
	for _, slot := range slots {
		if strings.EqualFold(slot.ReviewStatus, "pending") && slot.RenderPreviewAvailable {
			return slot.Slot
		}
	}
	for _, slot := range slots {
		if slot.RenderPreviewAvailable {
			return slot.Slot
		}
	}
	if len(slots) == 0 {
		return ""
	}
	return slots[0].Slot
}

func detectReviewSessionCapability(query *GenerationQueueQuery, slots []GenerationReviewSlot, previews []PlatformAssetRenderPreviews, state *generationReviewState) string {
	if query != nil && query.PreviewCapability != "" {
		return query.PreviewCapability
	}
	for _, slot := range slots {
		if strings.EqualFold(slot.ReviewStatus, "pending") && len(slot.PreviewCapabilities) > 0 {
			return slot.PreviewCapabilities[0]
		}
	}
	if state != nil {
		for _, item := range state.ByKey {
			if item.Pending {
				return item.Key.Capability
			}
		}
	}
	for _, slot := range slots {
		if len(slot.PreviewCapabilities) > 0 {
			return slot.PreviewCapabilities[0]
		}
	}
	for _, group := range previews {
		if group.Summary == nil {
			continue
		}
		for capability, count := range group.Summary.CapabilityCounts {
			if count > 0 {
				return capability
			}
		}
	}
	return ""
}

func buildGenerationReviewSections(queue *GenerationWorkQueue, selectedPlatform, focusCapability string, previews []PlatformAssetRenderPreviews, reviewState *generationReviewState) []GenerationReviewSection {
	sections := map[string]*GenerationReviewSection{}
	order := []string{}
	if queue != nil {
		for _, item := range queue.Items {
			if selectedPlatform != "" && item.Platform != selectedPlatform {
				continue
			}
			capabilities := item.PreviewCapabilities
			if len(capabilities) == 0 {
				continue
			}
			for _, capability := range capabilities {
				section := sections[capability]
				if section == nil {
					order = append(order, capability)
					spec := generationReviewSectionSpecForCapability(capability)
					section = &GenerationReviewSection{
						Capability:      capability,
						CapabilityLabel: generationPreviewCapabilityLabel(capability),
						SectionKey:      spec.SectionKey,
						Title:           spec.Title,
						Description:     spec.Description,
						EmptyState:      spec.EmptyState,
					}
					sections[capability] = section
				}
				section.ItemCount++
				section.Platforms = append(section.Platforms, item.Platform)
				section.Slots = append(section.Slots, GenerationReviewSlot{
					Platform:               item.Platform,
					Slot:                   item.Slot,
					Purpose:                item.Purpose,
					State:                  item.State,
					QualityGrade:           item.QualityGrade,
					QualityGradeLabel:      item.QualityGradeLabel,
					AssetID:                firstNonEmpty(item.SelectedAssetID, item.AssetID),
					TemplateLabel:          item.TemplateLabel,
					RenderPreviewAvailable: item.RenderPreviewAvailable,
					PreviewCapabilities:    append([]string(nil), item.PreviewCapabilities...),
				})
			}
		}
	}
	for _, group := range previews {
		if selectedPlatform != "" && group.Platform != selectedPlatform {
			continue
		}
		for _, slot := range flattenPlatformRenderPreviewSlots(group) {
			item := GenerationWorkQueueItem{RenderPreviewLayerTypes: append([]string(nil), slot.LayerTypes...)}
			for _, capability := range buildRenderPreviewCapabilities(item) {
				section := sections[capability]
				if section == nil {
					order = append(order, capability)
					spec := generationReviewSectionSpecForCapability(capability)
					section = &GenerationReviewSection{
						Capability:      capability,
						CapabilityLabel: generationPreviewCapabilityLabel(capability),
						SectionKey:      spec.SectionKey,
						Title:           spec.Title,
						Description:     spec.Description,
						EmptyState:      spec.EmptyState,
					}
					sections[capability] = section
				}
				found := false
				for _, existing := range section.Slots {
					if existing.Platform == group.Platform && existing.Slot == slot.Slot {
						found = true
						break
					}
				}
				if found {
					continue
				}
				section.ItemCount++
				section.Platforms = append(section.Platforms, group.Platform)
				section.Slots = append(section.Slots, GenerationReviewSlot{
					Platform:               group.Platform,
					Slot:                   slot.Slot,
					Purpose:                slot.Purpose,
					AssetID:                slot.AssetID,
					TemplateLabel:          slot.TemplateLabel,
					RenderPreviewAvailable: true,
					PreviewCapabilities:    buildRenderPreviewCapabilities(item),
				})
			}
		}
	}
	if len(order) == 0 {
		return nil
	}
	out := make([]GenerationReviewSection, 0, len(order))
	for _, capability := range order {
		section := sections[capability]
		section.Platforms = uniqueStrings(section.Platforms)
		section.PrimaryAction = reviewActionLabelForCapability(section.Capability)
		section.PrimaryActionKey = reviewActionKeyForCapability(section.Capability)
		section.PrimaryActionTarget = buildGenerationReviewTarget(selectedPlatform, detectSectionSlot(section.Slots), section.Capability)
		section.ReviewTarget = buildGenerationReviewTarget(selectedPlatform, detectSectionSlot(section.Slots), section.Capability)
		section.ToolbarActions = buildGenerationReviewSectionToolbarActions(queue, selectedPlatform, section.Slots, section.Capability)
		section.WorkflowActions = buildGenerationReviewSectionWorkflowActions(queue, selectedPlatform, section.Slots, section.Capability)
		applyReviewStateToSection(section, reviewState)
		out = append(out, *section)
	}
	if focusCapability == "" && len(out) > 0 {
		focusCapability = out[0].Capability
	}
	markSelectedReviewSections(out, focusCapability)
	return out
}

func applyReviewStateToReviewSlots(slots []GenerationReviewSlot, state *generationReviewState) {
	if state == nil {
		return
	}
	for i := range slots {
		key := normalizeReviewKey(slots[i].Platform) + ":" + normalizeReviewKey(slots[i].Slot)
		if slotState, ok := state.SlotSummary[key]; ok {
			slots[i].ReviewDecision = slotState.Decision
			slots[i].ReviewStatus = slotState.Status
			slots[i].AssetRevision = resolveSlotReviewAssetRevision(state, slots[i].Platform, slots[i].Slot, slots[i].PreviewCapabilities)
			slots[i].PreviewRevision = resolveSlotReviewPreviewRevision(state, slots[i].Platform, slots[i].Slot, slots[i].PreviewCapabilities)
			slots[i].TaskRevision = resolveSlotReviewTaskRevision(state, slots[i].Platform, slots[i].Slot, slots[i].PreviewCapabilities)
		}
	}
}

func applyReviewStateToSection(section *GenerationReviewSection, state *generationReviewState) {
	if section == nil || state == nil {
		return
	}
	key := generationReviewStateKey{
		Platform:   normalizeReviewKey(detectSectionPlatform(section.Platforms)),
		Slot:       normalizeReviewKey(detectSectionSlot(section.Slots)),
		Capability: normalizeReviewKey(section.Capability),
	}
	item, ok := state.ByKey[key]
	if !ok {
		section.ReviewStatus = "pending"
		return
	}
	if item.Pending || item.Record == nil {
		section.ReviewStatus = "pending"
		return
	}
	section.ReviewDecision = string(item.Record.Decision)
	section.ReviewStatus = item.Record.Status
	if !item.Record.ReviewedAt.IsZero() {
		reviewedAt := item.Record.ReviewedAt
		section.ReviewedAt = &reviewedAt
	}
}

func resolveSlotReviewAssetRevision(state *generationReviewState, platform, slot string, capabilities []string) string {
	for _, capability := range capabilities {
		if item, ok := state.ByKey[generationReviewStateKey{
			Platform:   normalizeReviewKey(platform),
			Slot:       normalizeReviewKey(slot),
			Capability: normalizeReviewKey(capability),
		}]; ok {
			return item.AssetRev
		}
	}
	return ""
}

func resolveSlotReviewPreviewRevision(state *generationReviewState, platform, slot string, capabilities []string) string {
	for _, capability := range capabilities {
		if item, ok := state.ByKey[generationReviewStateKey{
			Platform:   normalizeReviewKey(platform),
			Slot:       normalizeReviewKey(slot),
			Capability: normalizeReviewKey(capability),
		}]; ok {
			return item.PreviewRev
		}
	}
	return ""
}

func resolveSlotReviewTaskRevision(state *generationReviewState, platform, slot string, capabilities []string) string {
	for _, capability := range capabilities {
		if item, ok := state.ByKey[generationReviewStateKey{
			Platform:   normalizeReviewKey(platform),
			Slot:       normalizeReviewKey(slot),
			Capability: normalizeReviewKey(capability),
		}]; ok {
			return item.TaskRev
		}
	}
	return ""
}

func markSelectedReviewSlots(slots []GenerationReviewSlot, selectedSlot string) {
	for i := range slots {
		slots[i].Selected = slots[i].Slot == selectedSlot
	}
}

func markSelectedReviewSections(sections []GenerationReviewSection, focusCapability string) {
	for i := range sections {
		sections[i].Selected = sections[i].Capability == focusCapability
	}
}

func detectSectionSlot(slots []GenerationReviewSlot) string {
	for _, slot := range slots {
		if slot.RenderPreviewAvailable {
			return slot.Slot
		}
	}
	if len(slots) == 0 {
		return ""
	}
	return slots[0].Slot
}

func attachReviewTargetsToSlots(slots []GenerationReviewSlot, focusCapability string) {
	for i := range slots {
		capability := focusCapability
		if capability == "" && len(slots[i].PreviewCapabilities) > 0 {
			capability = slots[i].PreviewCapabilities[0]
		}
		slots[i].ReviewTarget = buildGenerationReviewTarget(slots[i].Platform, slots[i].Slot, capability)
	}
}

func attachReviewTargetsToSections(sections []GenerationReviewSection) {
	for i := range sections {
		sections[i].PrimaryActionTarget = buildGenerationReviewTarget(detectSectionPlatform(sections[i].Platforms), detectSectionSlot(sections[i].Slots), sections[i].Capability)
		sections[i].ReviewTarget = buildGenerationReviewTarget(detectSectionPlatform(sections[i].Platforms), detectSectionSlot(sections[i].Slots), sections[i].Capability)
	}
}

func attachReviewTargetsToPlatformCards(cards []ListingKitPlatformCard, selectedSlot, focusCapability string) {
	for i := range cards {
		cards[i].ReviewTarget = buildGenerationReviewTarget(cards[i].Platform, selectedSlot, focusCapability)
	}
}

func enrichReviewTargetsWithContext(slots []GenerationReviewSlot, sections []GenerationReviewSection, cards []ListingKitPlatformCard, selectedPlatform, selectedSlot, selectedCapability, selectedSectionKey string, focusedPreview *AssetRenderPreviewSlot) {
	for i := range slots {
		slots[i].ReviewTarget = enrichGenerationReviewTargetWithContext(slots[i].ReviewTarget, selectedPlatform, selectedSlot, selectedCapability, selectedSectionKey, focusedPreview)
	}
	for i := range sections {
		sections[i].PrimaryActionTarget = enrichGenerationReviewTargetWithContext(sections[i].PrimaryActionTarget, selectedPlatform, selectedSlot, selectedCapability, selectedSectionKey, focusedPreview)
		sections[i].ReviewTarget = enrichGenerationReviewTargetWithContext(sections[i].ReviewTarget, selectedPlatform, selectedSlot, selectedCapability, selectedSectionKey, focusedPreview)
		for j := range sections[i].ToolbarActions {
			sections[i].ToolbarActions[j].Target = enrichGenerationReviewTargetWithContext(sections[i].ToolbarActions[j].Target, selectedPlatform, selectedSlot, selectedCapability, selectedSectionKey, focusedPreview)
		}
		for j := range sections[i].WorkflowActions {
			sections[i].WorkflowActions[j].Target = enrichGenerationReviewTargetWithContext(sections[i].WorkflowActions[j].Target, selectedPlatform, selectedSlot, selectedCapability, selectedSectionKey, focusedPreview)
		}
	}
	for i := range cards {
		cards[i].ReviewTarget = enrichGenerationReviewTargetWithContext(cards[i].ReviewTarget, selectedPlatform, selectedSlot, selectedCapability, selectedSectionKey, focusedPreview)
	}
}

func detectSectionPlatform(platforms []string) string {
	if len(platforms) == 0 {
		return ""
	}
	return platforms[0]
}
