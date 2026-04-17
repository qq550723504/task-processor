package listingkit

type revisionPresentationData struct {
	Status        string
	Title         string
	Subtitle      string
	PrimaryAction string
	PrimaryView   string
	NextActions   []string
	Highlights    []string
}

func normalizeRevisionPresentation(data revisionPresentationData) revisionPresentationData {
	data.NextActions = uniqueStrings(filterNonEmptyStrings(data.NextActions))
	data.Highlights = uniqueStrings(filterNonEmptyStrings(data.Highlights))
	return data
}

func buildRevisionApplySummaryCardData(
	headline string,
	changeCount int,
	messages *RevisionResultMessages,
	appliedChanges *RevisionDiffPreview,
	listingResult *ListingKitResult,
) revisionPresentationData {
	statusSummary := buildRevisionRestoreStatusSummary(listingResult)
	recommendedView := buildRevisionApplyRecommendedView(listingResult)
	highlights := append(buildRevisionApplyHighlights(changeCount, appliedChanges), summaryHighlightsValue(statusSummary)...)
	highlights = append(highlights, messageWarningsValue(messages)...)

	return normalizeRevisionPresentation(revisionPresentationData{
		Status: firstNonEmpty(summaryStatusValue(statusSummary), "ready"),
		Title:  firstNonEmpty(headline, "资料已更新"),
		Subtitle: firstNonEmpty(
			messageDescriptionValue(messages),
			summarySubheadlineValue(statusSummary),
			buildRevisionApplySubtitle(changeCount),
		),
		PrimaryAction: firstNonEmpty(buildRevisionApplyPrimaryAction(statusSummary), "继续提交流程"),
		PrimaryView:   recommendedViewValue(recommendedView),
		Highlights:    highlights,
	})
}

func buildRevisionRestoreSummaryCardData(
	headline string,
	relationText string,
	nextActions []string,
	statusSummary *RevisionStatusSummary,
	messages *RevisionResultMessages,
	recommendedView *RevisionRecommendedView,
) revisionPresentationData {
	highlights := append([]string{relationText}, summaryHighlightsValue(statusSummary)...)
	highlights = append(highlights, messageWarningsValue(messages)...)

	return normalizeRevisionPresentation(revisionPresentationData{
		Status: firstNonEmpty(summaryStatusValue(statusSummary), "ready"),
		Title: firstNonEmpty(
			messageTitleValue(messages),
			headline,
			"历史版本已恢复",
		),
		Subtitle: firstNonEmpty(
			summarySubheadlineValue(statusSummary),
			messageDescriptionValue(messages),
			relationText,
		),
		PrimaryAction: firstNonEmpty(firstString(nextActions), "继续提交流程"),
		PrimaryView:   recommendedViewValue(recommendedView),
		Highlights:    highlights,
	})
}

func buildRevisionHistoryRestoreOverviewData(
	record *ListingKitRevisionRecord,
	safety *RevisionHistoryRestoreSafety,
	comparePreview *RevisionHistoryComparePreview,
) revisionPresentationData {
	data := revisionPresentationData{
		Status:        "ready",
		Title:         "恢复这条历史版本",
		PrimaryAction: "恢复历史版本",
		Highlights:    []string{},
		NextActions:   buildRevisionHistoryRestoreNextActions(record, safety, comparePreview),
	}

	if record != nil && record.Timeline != nil && record.Timeline.Headline != "" {
		data.Title = record.Timeline.Headline
	}

	if safety != nil {
		switch {
		case !safety.CanRestore:
			data.Status = "blocked"
			data.PrimaryAction = "暂不建议恢复"
			data.Subtitle = "当前上下文还不满足直接回滚条件，建议先处理阻塞项"
		case len(safety.RestoreWarnings) > 0:
			data.Status = "ready_with_warnings"
			data.Subtitle = "可以恢复，但建议先确认潜在影响"
		default:
			data.Subtitle = "当前可以直接恢复到这条历史版本"
		}
		data.Highlights = append(data.Highlights, safety.RestoreWarnings...)
	}

	if comparePreview != nil {
		if comparePreview.RelationLabel != "" {
			data.Highlights = append(data.Highlights, "比较目标："+comparePreview.RelationLabel)
		}
		if comparePreview.DiffPreview != nil {
			switch {
			case comparePreview.DiffPreview.ChangeCount > 0:
				data.Highlights = append(data.Highlights, "恢复后预计会影响 "+formatInt(comparePreview.DiffPreview.ChangeCount)+" 个字段")
			case comparePreview.CompareTo != "":
				data.Highlights = append(data.Highlights, "恢复后与"+firstNonEmpty(comparePreview.RelationLabel, "比较目标")+"没有字段差异")
			}
		}
	}

	if record != nil && record.Timeline != nil && record.Timeline.RelationText != "" {
		data.Highlights = append(data.Highlights, record.Timeline.RelationText)
	}

	return normalizeRevisionPresentation(data)
}
