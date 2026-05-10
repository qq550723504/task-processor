package listingkit

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
