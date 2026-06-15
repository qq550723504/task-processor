package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestStudioBatchGenerationSupportFilesOwnSeparatedHelperFamilies(t *testing.T) {
	t.Parallel()

	homeSrc, err := os.ReadFile("studio_batch_generation.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_batch_generation.go) error = %v", err)
	}
	homeContent := string(homeSrc)

	for _, needle := range []string{
		"func (g *studioBatchGenerationService) RunPendingStudioBatchItems(ctx context.Context, batchID string) error {",
		"func (g *studioBatchGenerationService) RecoverStudioBatchMaterialization(ctx context.Context, batchID string) error {",
		"func (g *studioBatchGenerationService) runItemAttempt(ctx context.Context, batch *StudioBatchRecord, item StudioBatchItemRecord, attemptNo int) error {",
		"func (g *studioBatchGenerationService) materializeAttempt(ctx context.Context, batch *StudioBatchRecord, item StudioBatchItemRecord, attempt *StudioGenerationAttemptRecord, response *StudioDesignResponse) error {",
		"func (g *studioBatchGenerationService) refreshBatchStatus(ctx context.Context, batchID string) error {",
	} {
		if !strings.Contains(homeContent, needle) {
			t.Fatalf("studio_batch_generation.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func (g *studioBatchGenerationService) recoverAwaitingMaterializationItem(",
		"func shouldRetryStudioBatchAttemptMessage(message string, attemptNo int) bool {",
		"func buildStudioBatchItemDesignRequest(batch *StudioBatchRecord, item StudioBatchItemRecord) *StudioDesignRequest {",
		"func expandStudioBatchItems(batch *StudioBatchRecord) []StudioBatchItemRecord {",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("studio_batch_generation.go should delegate helper family %q", needle)
		}
	}

	recoverySrc, err := os.ReadFile("studio_batch_generation_recovery_support.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_batch_generation_recovery_support.go) error = %v", err)
	}
	recoveryContent := string(recoverySrc)

	for _, needle := range []string{
		"func (g *studioBatchGenerationService) recoverAwaitingMaterializationItem(",
		"func (g *studioBatchGenerationService) recoverGeneratingItem(",
		"func (g *studioBatchGenerationService) recoverFailedItem(",
		"func shouldRetryStudioBatchAttemptMessage(message string, attemptNo int) bool {",
		"func latestRecoverableStudioBatchAttempt(attempts []StudioGenerationAttemptRecord) *StudioGenerationAttemptRecord {",
	} {
		if !strings.Contains(recoveryContent, needle) {
			t.Fatalf("studio_batch_generation_recovery_support.go should contain %q", needle)
		}
	}

	requestSrc, err := os.ReadFile("studio_batch_generation_request_support.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_batch_generation_request_support.go) error = %v", err)
	}
	requestContent := string(requestSrc)

	for _, needle := range []string{
		"func aggregateStudioBatchStatus(items []StudioBatchItemRecord) StudioBatchStatus {",
		"func buildStudioBatchItemDesignRequest(batch *StudioBatchRecord, item StudioBatchItemRecord) *StudioDesignRequest {",
		"func expandStudioBatchItems(batch *StudioBatchRecord) []StudioBatchItemRecord {",
		"func studioBatchAllGroupedSelections(batch *StudioBatchRecord) []SheinStudioGroupedSelection {",
		"func buildStudioBatchDesignID(attemptID string, index int) string {",
	} {
		if !strings.Contains(requestContent, needle) {
			t.Fatalf("studio_batch_generation_request_support.go should contain %q", needle)
		}
	}
}
