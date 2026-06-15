package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestWorkflowSDSSyncSupportFilesOwnHelperFamilies(t *testing.T) {
	t.Parallel()

	homeSrc, err := os.ReadFile("workflow_sds_sync.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_sds_sync.go) error = %v", err)
	}
	homeContent := string(homeSrc)

	for _, needle := range []string{
		"func (s *service) syncSDSDesign(ctx context.Context, task *Task, result *ListingKitResult, imageResult *productimage.ImageProcessResult, recorder *workflowRecorder) {",
		"func (s *service) syncSDSDesignFromRemote(ctx context.Context, task *Task, result *ListingKitResult, recorder *workflowRecorder) {",
		"func (s *service) syncSDSDesignVariantsFromRemote(ctx context.Context, task *Task, result *ListingKitResult, imageURL string, recorder *workflowRecorder) {",
	} {
		if !strings.Contains(homeContent, needle) {
			t.Fatalf("workflow_sds_sync.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func (s *service) syncSDSDesignFromUploadedImageKey(ctx context.Context, task *Task, key string, syncInput sdsusecase.SyncInput, timeout time.Duration) (*sdsworkflow.SyncResult, bool, error) {",
		"func representativeSDSVariantsByColor(variants []SDSSyncVariantOption) []SDSSyncVariantOption {",
		"func mergeSDSVariantSyncSummaries(options *SDSSyncOptions, summaries []SDSSyncSummary) *SDSSyncSummary {",
		"func needsLocalSDSMockupFallback(summary *SDSSyncSummary, options *SDSSyncOptions) bool {",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("workflow_sds_sync.go should delegate helper seam %q", needle)
		}
	}

	uploadedSrc, err := os.ReadFile("workflow_sds_sync_uploaded_support.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_sds_sync_uploaded_support.go) error = %v", err)
	}
	uploadedContent := string(uploadedSrc)

	for _, needle := range []string{
		"func (s *service) syncSDSDesignFromUploadedImagePath(ctx context.Context, task *Task, imageURL string, syncInput sdsusecase.SyncInput) (*sdsworkflow.SyncResult, bool, error) {",
		"func (s *service) syncSDSDesignFromUploadedImageKey(ctx context.Context, task *Task, key string, syncInput sdsusecase.SyncInput, timeout time.Duration) (*sdsworkflow.SyncResult, bool, error) {",
		"func uploadedListingKitImageKeyFromURL(rawURL string) (string, bool) {",
		"func studioSDSMaterialFileName(task *Task) string {",
	} {
		if !strings.Contains(uploadedContent, needle) {
			t.Fatalf("workflow_sds_sync_uploaded_support.go should contain %q", needle)
		}
	}

	variantSrc, err := os.ReadFile("workflow_sds_sync_variant_support.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_sds_sync_variant_support.go) error = %v", err)
	}
	variantContent := string(variantSrc)

	for _, needle := range []string{
		"func sdsVariantIDs(variants []SDSSyncVariantOption) []int64 {",
		"func representativeSDSVariantsByColor(variants []SDSSyncVariantOption) []SDSSyncVariantOption {",
		"func mergeSDSVariantSyncSummaries(options *SDSSyncOptions, summaries []SDSSyncSummary) *SDSSyncSummary {",
		"func firstNonZeroInt64(values ...int64) int64 {",
	} {
		if !strings.Contains(variantContent, needle) {
			t.Fatalf("workflow_sds_sync_variant_support.go should contain %q", needle)
		}
	}

	fallbackSrc, err := os.ReadFile("workflow_sds_sync_fallback_support.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_sds_sync_fallback_support.go) error = %v", err)
	}
	fallbackContent := string(fallbackSrc)

	for _, needle := range []string{
		"func needsLocalSDSMockupFallback(summary *SDSSyncSummary, options *SDSSyncOptions) bool {",
		"func (s *service) applyLocalSDSMockupFallback(ctx context.Context, result *ListingKitResult, sourceURL string, options *SDSSyncOptions) {",
		"func firstImageResultURL(imageResult *productimage.ImageProcessResult) string {",
	} {
		if !strings.Contains(fallbackContent, needle) {
			t.Fatalf("workflow_sds_sync_fallback_support.go should contain %q", needle)
		}
	}
}
