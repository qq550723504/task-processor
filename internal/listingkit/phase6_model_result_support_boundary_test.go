package listingkit

import "testing"

func TestModelResultSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSource := readTaskGenerationSourceFile(t, "model_result.go")
	assertSourceContainsAll(t, rootSource, []string{
		"type ListingKitResult struct {",
		"type StandardProductSnapshot struct {",
		"type GenerationSummary struct {",
	})
	assertSourceExcludesAll(t, rootSource, []string{
		"type SDSSyncSummary struct {",
		"type SDSSyncDiagnostics struct {",
		"type PodExecutionSummary struct {",
		"type PodExecutionAuditEvent struct {",
		"type SDSSyncFinishedProductObservation struct {",
		"type SDSSyncSensitiveWordHit struct {",
		"type GenerationRecoverySummary struct {",
		"type GenerationResolvedActionSummary struct {",
		"type AmazonPackage struct {",
		"type TemuPackage struct {",
		"type WalmartPackage struct {",
		"type TemuBatchSKUInfo struct {",
	})

	supportSource := readTaskGenerationSourceFile(t, "model_result_support.go")
	assertSourceContainsAll(t, supportSource, []string{
		"type GenerationRecoverySummary struct {",
		"type GenerationResolvedActionSummary struct {",
		"type AmazonPackage struct {",
		"type TemuPackage struct {",
		"type WalmartPackage struct {",
		"type TemuBatchSKUInfo struct {",
	})
	assertSourceExcludesAll(t, supportSource, []string{
		"type ListingKitResult struct {",
		"type StandardProductSnapshot struct {",
	})

	executionSource := readTaskGenerationSourceFile(t, "model_result_execution_support.go")
	assertSourceContainsAll(t, executionSource, []string{
		"type SDSSyncSummary struct {",
		"type PodExecutionSummary struct {",
		"type SDSSyncDiagnostics struct {",
		"type PodExecutionAuditEvent struct {",
		"type SDSSyncFinishedProductObservation struct {",
		"type SDSSyncSensitiveWordHit struct {",
	})
	assertSourceExcludesAll(t, executionSource, []string{
		"type ListingKitResult struct {",
		"type StandardProductSnapshot struct {",
		"type GenerationRecoverySummary struct {",
		"type GenerationResolvedActionSummary struct {",
	})
}
