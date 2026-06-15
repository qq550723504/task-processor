package listingkit

type generationReviewSectionSpec struct {
	SectionKey  string
	Title       string
	Description string
	EmptyState  string
}

func generationReviewSectionSpecForCapability(capability string) generationReviewSectionSpec {
	cfg := generationReviewSectionConfigForCapability(capability)
	return generationReviewSectionSpec{
		SectionKey:  generationReviewSectionKey(capability),
		Title:       cfg.Title,
		Description: cfg.Description,
		EmptyState:  cfg.EmptyState,
	}
}

func generationPreviewCapabilityLabel(capability string) string {
	switch capability {
	case "detail_preview":
		return "Detail Preview"
	case "measurement_preview":
		return "Measurement Preview"
	case "badge_preview":
		return "Badge Preview"
	case "copy_preview":
		return "Copy Preview"
	case "subject_preview":
		return "Subject Preview"
	default:
		return capability
	}
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
			capabilities := buildRenderPreviewCapabilitiesForSlot(slot)
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
					PreviewCapabilities:    capabilities,
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

func detectSectionPlatform(platforms []string) string {
	if len(platforms) == 0 {
		return ""
	}
	return platforms[0]
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

func attachReviewTargetsToSections(sections []GenerationReviewSection) {
	for i := range sections {
		sections[i].PrimaryActionTarget = buildGenerationReviewTarget(detectSectionPlatform(sections[i].Platforms), detectSectionSlot(sections[i].Slots), sections[i].Capability)
		sections[i].ReviewTarget = buildGenerationReviewTarget(detectSectionPlatform(sections[i].Platforms), detectSectionSlot(sections[i].Slots), sections[i].Capability)
	}
}
