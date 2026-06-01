package listingkit

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	defaultStudioBatchTransientRetryLimit = 3
	defaultStudioBatchStaleRecoveryLimit  = defaultStudioBatchTransientRetryLimit + 1
	defaultStudioBatchAttemptStaleAfter   = 10 * time.Minute
)

type studioBatchGenerator interface {
	RunPendingStudioBatchItems(ctx context.Context, batchID string) error
	RecoverStudioBatchMaterialization(ctx context.Context, batchID string) error
}

type studioBatchGenerateExecutor func(context.Context, StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error)

type studioBatchGenerationServiceConfig struct {
	repo        StudioBatchRepository
	execute     studioBatchGenerateExecutor
	currentTime func() time.Time
}

type studioBatchGenerationService struct {
	repo        StudioBatchRepository
	execute     studioBatchGenerateExecutor
	currentTime func() time.Time
}

func newStudioBatchGenerationService(config studioBatchGenerationServiceConfig) *studioBatchGenerationService {
	return &studioBatchGenerationService{
		repo:        config.repo,
		execute:     config.execute,
		currentTime: config.currentTime,
	}
}

func (g *studioBatchGenerationService) RunPendingStudioBatchItems(ctx context.Context, batchID string) error {
	if g == nil || g.repo == nil {
		return fmt.Errorf("studio batch repository is not configured")
	}
	if g.execute == nil {
		return fmt.Errorf("studio batch execute function is not configured")
	}

	detail, err := g.repo.GetStudioBatchDetail(ctx, strings.TrimSpace(batchID))
	if err != nil {
		return err
	}
	if detail == nil || detail.Batch == nil {
		return nil
	}

	for _, item := range detail.Items {
		if item.Status != StudioBatchItemStatusPending {
			continue
		}
		claimedItem, claimed, err := g.repo.ClaimStudioBatchItem(ctx, item.ID, StudioBatchItemStatusPending, StudioBatchItemStatusGenerating, g.now())
		if err != nil {
			return err
		}
		if !claimed || claimedItem == nil {
			continue
		}
		attemptNo := len(detail.AttemptsByItem[item.ID]) + 1
		if err := g.runItemAttempt(ctx, detail.Batch, *claimedItem, attemptNo); err != nil {
			return err
		}
	}

	return g.refreshBatchStatus(ctx, detail.Batch.ID)
}

func (g *studioBatchGenerationService) RecoverStudioBatchMaterialization(ctx context.Context, batchID string) error {
	if g == nil || g.repo == nil {
		return fmt.Errorf("studio batch repository is not configured")
	}

	detail, err := g.repo.GetStudioBatchDetail(ctx, strings.TrimSpace(batchID))
	if err != nil {
		return err
	}
	if detail == nil || detail.Batch == nil {
		return nil
	}

	for _, item := range detail.Items {
		attempts := detail.AttemptsByItem[item.ID]
		switch item.Status {
		case StudioBatchItemStatusAwaitingMaterialization:
			if err := g.recoverAwaitingMaterializationItem(ctx, detail.Batch, item, attempts); err != nil {
				return err
			}
		case StudioBatchItemStatusGenerating:
			if err := g.recoverGeneratingItem(ctx, detail.Batch, item, attempts); err != nil {
				return err
			}
		case StudioBatchItemStatusFailed:
			if err := g.recoverFailedItem(ctx, item, attempts); err != nil {
				return err
			}
		}
	}

	return g.refreshBatchStatus(ctx, detail.Batch.ID)
}

func (g *studioBatchGenerationService) recoverAwaitingMaterializationItem(ctx context.Context, batch *StudioBatchRecord, item StudioBatchItemRecord, attempts []StudioGenerationAttemptRecord) error {
	attempt := latestRecoverableStudioBatchAttempt(attempts)
	if attempt == nil {
		return g.failItemAndAttempt(ctx, item, nil, "materialization recovery missing generation attempt")
	}
	if strings.TrimSpace(attempt.ResultPayload) == "" {
		return g.failItemAndAttempt(ctx, item, attempt, "generation result payload missing")
	}

	claimedItem, claimed, err := g.repo.ClaimStudioBatchItem(ctx, item.ID, StudioBatchItemStatusAwaitingMaterialization, StudioBatchItemStatusGenerating, g.now())
	if err != nil {
		return err
	}
	if !claimed || claimedItem == nil {
		return nil
	}

	var response StudioDesignResponse
	if err := json.Unmarshal([]byte(attempt.ResultPayload), &response); err != nil {
		return g.failItemAndAttempt(ctx, *claimedItem, attempt, "generation result payload invalid")
	}
	return g.materializeAttempt(ctx, batch, *claimedItem, attempt, &response)
}

func (g *studioBatchGenerationService) recoverGeneratingItem(ctx context.Context, batch *StudioBatchRecord, item StudioBatchItemRecord, attempts []StudioGenerationAttemptRecord) error {
	attempt := latestStudioBatchAttempt(attempts)
	if attempt == nil {
		return g.failItemAndAttempt(ctx, item, nil, "generation interrupted before attempt persisted")
	}

	switch attempt.Status {
	case StudioGenerationAttemptStatusSucceeded, StudioGenerationAttemptStatusMaterialized:
		if strings.TrimSpace(attempt.ResultPayload) == "" {
			return g.failItemAndAttempt(ctx, item, attempt, "generation result payload missing")
		}
		claimedItem, claimed, err := g.repo.ClaimStudioBatchItem(ctx, item.ID, StudioBatchItemStatusGenerating, StudioBatchItemStatusAwaitingMaterialization, g.now())
		if err != nil {
			return err
		}
		if !claimed || claimedItem == nil {
			return nil
		}
		var response StudioDesignResponse
		if err := json.Unmarshal([]byte(attempt.ResultPayload), &response); err != nil {
			return g.failItemAndAttempt(ctx, *claimedItem, attempt, "generation result payload invalid")
		}
		return g.recoverAwaitingMaterializationItem(ctx, batch, *claimedItem, attempts)
	case StudioGenerationAttemptStatusRunning, StudioGenerationAttemptStatusQueued:
		if !isStudioBatchAttemptStale(attempt, g.now()) {
			return nil
		}
		message := "generation attempt timed out before result persisted"
		if shouldRetryStudioBatchRecoveredFailure(message, attempt.AttemptNo) {
			return g.requeueItemAfterFailedAttempt(ctx, item, attempt, message)
		}
		return g.failItemAndAttempt(ctx, item, attempt, message)
	default:
		return g.failItemAndAttempt(ctx, item, attempt, firstNonEmpty(strings.TrimSpace(attempt.ErrorMessage), "generation failed"))
	}
}

func (g *studioBatchGenerationService) recoverFailedItem(ctx context.Context, item StudioBatchItemRecord, attempts []StudioGenerationAttemptRecord) error {
	attempt := latestStudioBatchAttempt(attempts)
	if attempt == nil {
		return nil
	}
	if attempt.Status != StudioGenerationAttemptStatusFailed {
		return nil
	}
	message := firstNonEmpty(strings.TrimSpace(attempt.ErrorMessage), strings.TrimSpace(item.LastError))
	if !shouldRetryStudioBatchRecoveredFailure(message, attempt.AttemptNo) {
		return nil
	}
	return g.requeueItemAfterFailedAttempt(ctx, item, attempt, message)
}

func isStudioBatchAttemptStale(attempt *StudioGenerationAttemptRecord, now time.Time) bool {
	if attempt == nil {
		return false
	}
	referenceTime := attempt.UpdatedAt
	if attempt.StartedAt != nil && attempt.StartedAt.After(referenceTime) {
		referenceTime = *attempt.StartedAt
	}
	if referenceTime.IsZero() {
		referenceTime = attempt.CreatedAt
	}
	if referenceTime.IsZero() {
		return false
	}
	return now.UTC().Sub(referenceTime.UTC()) >= defaultStudioBatchAttemptStaleAfter
}

func latestStudioBatchAttempt(attempts []StudioGenerationAttemptRecord) *StudioGenerationAttemptRecord {
	if len(attempts) == 0 {
		return nil
	}
	cloned := attempts[len(attempts)-1]
	return &cloned
}

func (g *studioBatchGenerationService) failItemAndAttempt(ctx context.Context, item StudioBatchItemRecord, attempt *StudioGenerationAttemptRecord, message string) error {
	now := g.now()
	if attempt != nil {
		attempt.Status = StudioGenerationAttemptStatusFailed
		attempt.ErrorMessage = message
		if attempt.FinishedAt == nil {
			attempt.FinishedAt = timePtr(now)
		}
		attempt.UpdatedAt = now
		if err := g.repo.UpdateStudioGenerationAttempt(ctx, attempt); err != nil {
			return err
		}
	}

	item.Status = StudioBatchItemStatusFailed
	item.LastError = message
	item.UpdatedAt = now
	return g.repo.UpdateStudioBatchItem(ctx, &item)
}

func (g *studioBatchGenerationService) requeueItemAfterFailedAttempt(ctx context.Context, item StudioBatchItemRecord, attempt *StudioGenerationAttemptRecord, message string) error {
	now := g.now()
	if attempt != nil {
		attempt.Status = StudioGenerationAttemptStatusFailed
		attempt.ErrorMessage = message
		if attempt.FinishedAt == nil {
			attempt.FinishedAt = timePtr(now)
		}
		attempt.UpdatedAt = now
		if err := g.repo.UpdateStudioGenerationAttempt(ctx, attempt); err != nil {
			return err
		}
	}

	item.Status = StudioBatchItemStatusPending
	item.LastError = ""
	item.UpdatedAt = now
	return g.repo.UpdateStudioBatchItem(ctx, &item)
}

func (g *studioBatchGenerationService) runItemAttempt(ctx context.Context, batch *StudioBatchRecord, item StudioBatchItemRecord, attemptNo int) error {
	request := buildStudioBatchItemDesignRequest(batch, item)
	requestPayload, err := json.Marshal(request)
	if err != nil {
		return err
	}

	nextAttemptNo := attemptNo
	for {
		now := g.now()
		attempt := &StudioGenerationAttemptRecord{
			ID:             buildStudioBatchAttemptID(item.ID, nextAttemptNo),
			ItemID:         item.ID,
			AttemptNo:      nextAttemptNo,
			Status:         StudioGenerationAttemptStatusRunning,
			RequestPayload: string(requestPayload),
			StartedAt:      timePtr(now),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := g.repo.CreateStudioGenerationAttempt(ctx, attempt); err != nil {
			return err
		}

		execution, execErr := g.execute(ctx, StudioBatchGenerateExecutionInput{
			BatchID:   batch.ID,
			ItemID:    item.ID,
			AttemptID: attempt.ID,
			Request:   request,
		})
		finishedAt := g.now()
		attempt.FinishedAt = timePtr(finishedAt)
		attempt.UpdatedAt = finishedAt
		if execErr != nil {
			attempt.Status = StudioGenerationAttemptStatusFailed
			attempt.ErrorMessage = execErr.Error()
			if updateErr := g.repo.UpdateStudioGenerationAttempt(ctx, attempt); updateErr != nil {
				return updateErr
			}
			if shouldRetryStudioBatchAttempt(execErr, nextAttemptNo) {
				nextAttemptNo++
				continue
			}
			item.Status = StudioBatchItemStatusFailed
			item.LastError = execErr.Error()
			item.UpdatedAt = finishedAt
			return g.repo.UpdateStudioBatchItem(ctx, &item)
		}

		attempt.Status = StudioGenerationAttemptStatusSucceeded
		attempt.ResultPayload = strings.TrimSpace(execution.ResultPayload)
		if attempt.ResultPayload == "" && execution.Response != nil {
			payload, marshalErr := json.Marshal(execution.Response)
			if marshalErr != nil {
				return marshalErr
			}
			attempt.ResultPayload = string(payload)
		}
		if err := g.repo.UpdateStudioGenerationAttempt(ctx, attempt); err != nil {
			return err
		}

		claimedItem, claimed, err := g.repo.ClaimStudioBatchItem(ctx, item.ID, StudioBatchItemStatusGenerating, StudioBatchItemStatusAwaitingMaterialization, finishedAt)
		if err != nil {
			return err
		}
		if !claimed || claimedItem == nil {
			return nil
		}

		return g.materializeAttempt(ctx, batch, *claimedItem, attempt, execution.Response)
	}
}

func (g *studioBatchGenerationService) materializeAttempt(ctx context.Context, batch *StudioBatchRecord, item StudioBatchItemRecord, attempt *StudioGenerationAttemptRecord, response *StudioDesignResponse) error {
	if response == nil || len(response.Images) == 0 {
		item.Status = StudioBatchItemStatusFailed
		item.LastError = "generation returned no images"
		item.UpdatedAt = g.now()
		return g.repo.UpdateStudioBatchItem(ctx, &item)
	}

	now := g.now()
	designs := make([]StudioMaterializedDesignRecord, 0, len(response.Images))
	for index, image := range response.Images {
		designID := strings.TrimSpace(image.ID)
		if designID == "" {
			designID = buildStudioBatchDesignID(attempt.ID, index)
		}
		designs = append(designs, StudioMaterializedDesignRecord{
			ID:               designID,
			BatchID:          item.BatchID,
			ItemID:           item.ID,
			SourceAttemptID:  attempt.ID,
			TargetGroupKey:   item.TargetGroupKey,
			TargetGroupLabel: item.TargetGroupLabel,
			ImageURL:         strings.TrimSpace(image.ImageURL),
			SortOrder:        index,
			CreatedAt:        now,
			UpdatedAt:        now,
		})
	}
	if err := g.repo.ReplaceStudioItemMaterializedDesigns(ctx, item.ID, designs); err != nil {
		return err
	}

	attempt.Status = StudioGenerationAttemptStatusMaterialized
	attempt.UpdatedAt = now
	if attempt.FinishedAt == nil {
		attempt.FinishedAt = timePtr(now)
	}
	if err := g.repo.UpdateStudioGenerationAttempt(ctx, attempt); err != nil {
		return err
	}

	item.Status = StudioBatchItemStatusReviewReady
	item.LastError = ""
	item.UpdatedAt = now
	return g.repo.UpdateStudioBatchItem(ctx, &item)
}

func (g *studioBatchGenerationService) refreshBatchStatus(ctx context.Context, batchID string) error {
	detail, err := g.repo.GetStudioBatchDetail(ctx, batchID)
	if err != nil {
		return err
	}
	if detail == nil || detail.Batch == nil {
		return nil
	}

	nextStatus := aggregateStudioBatchStatus(detail.Items)
	if detail.Batch.Status == nextStatus {
		return nil
	}
	batch := *detail.Batch
	batch.Status = nextStatus
	batch.UpdatedAt = g.now()
	return g.repo.UpdateStudioBatch(ctx, &batch)
}

func (g *studioBatchGenerationService) now() time.Time {
	if g != nil && g.currentTime != nil {
		return g.currentTime().UTC()
	}
	return time.Now().UTC()
}

func shouldRetryStudioBatchAttempt(err error, attemptNo int) bool {
	if err == nil {
		return false
	}
	return shouldRetryStudioBatchAttemptMessage(err.Error(), attemptNo)
}

func shouldRetryStudioBatchRecoveredFailure(message string, attemptNo int) bool {
	if isStudioBatchTimeoutFailureMessage(message) {
		return attemptNo < defaultStudioBatchStaleRecoveryLimit
	}
	return shouldRetryStudioBatchAttemptMessage(message, attemptNo)
}

func shouldRetryStudioBatchAttemptMessage(message string, attemptNo int) bool {
	if attemptNo >= defaultStudioBatchTransientRetryLimit {
		return false
	}
	message = strings.ToLower(strings.TrimSpace(message))
	if message == "" {
		return false
	}
	return isStudioBatchTransientRetryMessage(message)
}

func isStudioBatchTimeoutFailureMessage(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	if message == "" {
		return false
	}
	return strings.Contains(message, "timeout") ||
		strings.Contains(message, "timed out") ||
		strings.Contains(message, "gateway timeout")
}

func isStudioBatchTransientRetryMessage(message string) bool {
	if message == "" {
		return false
	}
	return strings.Contains(message, "excessive system load") ||
		strings.Contains(message, "rate limit") ||
		strings.Contains(message, "rate limited") ||
		strings.Contains(message, "too many requests") ||
		strings.Contains(message, "temporarily unavailable") ||
		strings.Contains(message, "timeout") ||
		strings.Contains(message, "timed out") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "service unavailable") ||
		strings.Contains(message, "bad gateway") ||
		strings.Contains(message, "gateway timeout")
}

func latestRecoverableStudioBatchAttempt(attempts []StudioGenerationAttemptRecord) *StudioGenerationAttemptRecord {
	for index := len(attempts) - 1; index >= 0; index-- {
		attempt := attempts[index]
		if attempt.Status != StudioGenerationAttemptStatusSucceeded && attempt.Status != StudioGenerationAttemptStatusMaterialized {
			continue
		}
		cloned := attempt
		return &cloned
	}
	return nil
}

func aggregateStudioBatchStatus(items []StudioBatchItemRecord) StudioBatchStatus {
	if len(items) == 0 {
		return StudioBatchStatusDraft
	}

	reviewReady := 0
	failed := 0
	active := 0
	for _, item := range items {
		switch item.Status {
		case StudioBatchItemStatusReviewReady:
			reviewReady++
		case StudioBatchItemStatusFailed:
			failed++
		case StudioBatchItemStatusGenerating, StudioBatchItemStatusAwaitingMaterialization, StudioBatchItemStatusPending:
			active++
		}
	}

	switch {
	case reviewReady == len(items):
		return StudioBatchStatusReviewReady
	case failed == len(items):
		return StudioBatchStatusFailed
	case failed > 0 && reviewReady > 0:
		return StudioBatchStatusPartiallyFailed
	case failed > 0 && active > 0:
		return StudioBatchStatusPartiallyFailed
	case reviewReady > 0 && active > 0:
		return StudioBatchStatusPartiallyMaterialized
	default:
		return StudioBatchStatusGenerating
	}
}

func buildStudioBatchItemDesignRequest(batch *StudioBatchRecord, item StudioBatchItemRecord) *StudioDesignRequest {
	selection := firstStudioBatchItemSelection(batch, item)
	return &StudioDesignRequest{
		Prompt:                    strings.TrimSpace(batch.Prompt),
		Count:                     parseStudioBatchRunStyleCount(strings.TrimSpace(batch.StyleCount)),
		VariationIntensity:        strings.TrimSpace(batch.VariationIntensity),
		PrintableWidth:            selection.PrintableWidth,
		PrintableHeight:           selection.PrintableHeight,
		ProductReferenceImageURLs: studioBatchItemReferenceImageURLs(batch, item),
		ImageModel:                strings.TrimSpace(batch.ArtworkModel),
		TransparentBackground:     batch.TransparentBackground,
	}
}

func firstStudioBatchItemSelection(batch *StudioBatchRecord, item StudioBatchItemRecord) SheinStudioSelection {
	selections := resolveStudioBatchItemSelections(batch, item)
	if len(selections) == 0 {
		return SheinStudioSelection(batch.Selection)
	}
	return selections[0].Selection
}

func studioBatchItemReferenceImageURLs(batch *StudioBatchRecord, item StudioBatchItemRecord) []string {
	selections := resolveStudioBatchItemSelections(batch, item)
	seen := make(map[string]struct{})
	result := make([]string, 0, len(selections)*4+len(batch.SelectedSDSImages))
	appendURL := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	for _, grouped := range selections {
		appendURL(grouped.Selection.MockupImageURL)
		for _, value := range grouped.Selection.MockupImageURLs {
			appendURL(value)
		}
		for _, value := range grouped.Selection.SizeReferenceImageURLs {
			appendURL(value)
		}
	}
	for _, image := range batch.SelectedSDSImages {
		appendURL(image.ImageURL)
	}
	return result
}

func resolveStudioBatchItemSelections(batch *StudioBatchRecord, item StudioBatchItemRecord) []SheinStudioGroupedSelection {
	selectionMap := studioBatchSelectionSnapshotMap(batch)
	selections := make([]SheinStudioGroupedSelection, 0, len(item.SelectionIDs))
	for _, selectionID := range item.SelectionIDs {
		grouped, ok := selectionMap[strings.TrimSpace(selectionID)]
		if !ok {
			continue
		}
		selections = append(selections, grouped)
	}
	return selections
}

func expandStudioBatchItems(batch *StudioBatchRecord) []StudioBatchItemRecord {
	groupedSelections := studioBatchAllGroupedSelections(batch)
	if len(groupedSelections) == 0 {
		return nil
	}

	items := make([]StudioBatchItemRecord, 0, len(groupedSelections))
	if strings.TrimSpace(batch.GroupedImageMode) == "per_product" {
		for index, grouped := range groupedSelections {
			selectionID := strings.TrimSpace(grouped.SelectionID)
			if selectionID == "" {
				continue
			}
			items = append(items, StudioBatchItemRecord{
				ID:               buildStudioBatchItemID(batch.ID, index),
				BatchID:          batch.ID,
				TargetGroupKey:   selectionID,
				TargetGroupLabel: buildStudioBatchPerProductLabel(grouped.Selection),
				SelectionIDs:     SheinStudioStringList{selectionID},
				GroupMode:        "per_product",
				Status:           StudioBatchItemStatusPending,
				SelectionCount:   1,
			})
		}
		return items
	}

	type bucket struct {
		key          string
		label        string
		selectionIDs []string
	}
	buckets := make([]bucket, 0)
	bucketIndex := make(map[string]int)
	for _, grouped := range groupedSelections {
		selectionID := strings.TrimSpace(grouped.SelectionID)
		if selectionID == "" {
			continue
		}
		key := buildStudioBatchSharedBySizeGroupKey(grouped.Selection)
		index, ok := bucketIndex[key]
		if !ok {
			bucketIndex[key] = len(buckets)
			buckets = append(buckets, bucket{
				key:          key,
				label:        buildStudioBatchSharedBySizeGroupLabel(grouped.Selection),
				selectionIDs: []string{selectionID},
			})
			continue
		}
		buckets[index].selectionIDs = append(buckets[index].selectionIDs, selectionID)
	}
	for index, bucket := range buckets {
		items = append(items, StudioBatchItemRecord{
			ID:               buildStudioBatchItemID(batch.ID, index),
			BatchID:          batch.ID,
			TargetGroupKey:   bucket.key,
			TargetGroupLabel: bucket.label,
			SelectionIDs:     append(SheinStudioStringList(nil), bucket.selectionIDs...),
			GroupMode:        "shared_by_size",
			Status:           StudioBatchItemStatusPending,
			SelectionCount:   len(bucket.selectionIDs),
		})
	}
	return items
}

func studioBatchAllGroupedSelections(batch *StudioBatchRecord) []SheinStudioGroupedSelection {
	if batch == nil {
		return nil
	}

	result := make([]SheinStudioGroupedSelection, 0, len(batch.GroupedSelections)+1)
	seen := make(map[string]struct{}, len(batch.GroupedSelections)+1)
	appendGrouped := func(grouped SheinStudioGroupedSelection) {
		selectionID := strings.TrimSpace(grouped.SelectionID)
		if selectionID == "" || grouped.Selection.VariantID <= 0 {
			return
		}
		if _, ok := seen[selectionID]; ok {
			return
		}
		seen[selectionID] = struct{}{}
		result = append(result, grouped)
	}

	primary := SheinStudioSelection(batch.Selection)
	if primary.VariantID > 0 {
		appendGrouped(SheinStudioGroupedSelection{
			SelectionID: selectionIDForStudioSelection(primary),
			Selection:   primary,
			Eligible:    true,
		})
	}
	for _, grouped := range batch.GroupedSelections {
		if !grouped.Eligible {
			continue
		}
		appendGrouped(grouped)
	}
	return result
}

func studioBatchSelectionSnapshotMap(batch *StudioBatchRecord) map[string]SheinStudioGroupedSelection {
	selections := studioBatchAllGroupedSelections(batch)
	result := make(map[string]SheinStudioGroupedSelection, len(selections))
	for _, grouped := range selections {
		result[strings.TrimSpace(grouped.SelectionID)] = grouped
	}
	return result
}

func selectionIDForStudioSelection(selection SheinStudioSelection) string {
	if selection.VariantID <= 0 {
		return ""
	}
	selectedVariantIDs := append([]int64(nil), selection.SelectedVariantIDs...)
	if len(selectedVariantIDs) == 0 {
		for _, variant := range selection.Variants {
			if variant.VariantID > 0 {
				selectedVariantIDs = append(selectedVariantIDs, variant.VariantID)
			}
		}
	}
	tokens := make([]string, 0, len(selectedVariantIDs))
	for _, id := range selectedVariantIDs {
		tokens = append(tokens, strconv.FormatInt(id, 10))
	}
	parentProductID := selection.ParentProductID
	if parentProductID <= 0 {
		parentProductID = selection.ProductID
	}
	return strings.Join([]string{
		strconv.FormatInt(parentProductID, 10),
		strconv.FormatInt(selection.PrototypeGroupID, 10),
		strconv.FormatInt(selection.VariantID, 10),
		strings.TrimSpace(selection.LayerID),
		strings.Join(tokens, ","),
	}, ":")
}

func buildStudioBatchSharedBySizeGroupKey(selection SheinStudioSelection) string {
	return fmt.Sprintf("size:%dx%d", selection.PrintableWidth, selection.PrintableHeight)
}

func buildStudioBatchSharedBySizeGroupLabel(selection SheinStudioSelection) string {
	if selection.PrintableWidth > 0 && selection.PrintableHeight > 0 {
		return fmt.Sprintf("%d x %d", selection.PrintableWidth, selection.PrintableHeight)
	}
	return "自动尺寸"
}

func buildStudioBatchPerProductLabel(selection SheinStudioSelection) string {
	productName := strings.TrimSpace(selection.ProductName)
	if productName == "" {
		productName = "SDS 商品"
	}
	variantLabel := strings.TrimSpace(selection.VariantLabel)
	if variantLabel == "" {
		return productName
	}
	return fmt.Sprintf("%s · %s", productName, variantLabel)
}

func buildStudioBatchItemID(batchID string, index int) string {
	return fmt.Sprintf("%s:item:%d", strings.TrimSpace(batchID), index+1)
}

func buildStudioBatchAttemptID(itemID string, attemptNo int) string {
	return fmt.Sprintf("%s:attempt:%d", strings.TrimSpace(itemID), attemptNo)
}

func buildStudioBatchDesignID(attemptID string, index int) string {
	return fmt.Sprintf("%s:design:%d", strings.TrimSpace(attemptID), index+1)
}

func timePtr(value time.Time) *time.Time {
	return &value
}
