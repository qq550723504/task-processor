package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinSubmitImageUploadSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSrc, err := os.ReadFile("shein_submit_images.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_images.go) error = %v", err)
	}
	rootContent := string(rootSrc)

	for _, needle := range []string{
		"func sheinImageUploadCache(pkg *SheinPackage) map[string]string {",
	} {
		if !strings.Contains(rootContent, needle) {
			t.Fatalf("shein_submit_images.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func uploadSheinProductImages(product *sheinproduct.Product, uploader sheinimage.ImageAPI, cached map[string]string) (int, map[string]string, error) {",
		"func collectSheinProductImageRefs(product *sheinproduct.Product) []sheinImageUploadRef {",
		"func appendSheinImageInfoRefs(refs []sheinImageUploadRef, info *sheinproduct.ImageInfo) []sheinImageUploadRef {",
		"func uploadSheinImageJobs(jobs map[string]sheinImageUploadJob, uploader sheinimage.ImageAPI, uploaded map[string]string) (int, error) {",
		"func runSheinImageUploadJobs(jobs map[string]sheinImageUploadJob, uploader sheinimage.ImageAPI, uploaded map[string]string, existing map[string]string) (int, error) {",
		"func uploadSingleSheinImage(job sheinImageUploadJob, uploader sheinimage.ImageAPI, existing map[string]string) (string, error) {",
		"func uploadSheinImageInfo(info *sheinproduct.ImageInfo, uploader sheinimage.ImageAPI, uploaded map[string]string) (int, error) {",
		"func cloneSheinProductForSubmit(",
		"func sheinProductImageURLCount(",
		"func sheinProductPendingImageUploadCount(",
		"func sheinImageInfoURLCount(",
		"func sheinImageInfoPendingUploadCount(",
		"func isSheinUploadedImageURL(",
		"func isSDSImageURL(",
		"func sheinImageUploadCacheHit(",
		"func cloneSheinImageUploadCache(",
	} {
		if strings.Contains(rootContent, needle) {
			t.Fatalf("shein_submit_images.go should delegate upload support helper %q", needle)
		}
	}

	assertFileAbsent(t, "shein_submit_image_upload_support.go")

	publishingUploadSrc, err := os.ReadFile("../publishing/shein/submit_image_upload.go")
	if err != nil {
		t.Fatalf("ReadFile(../publishing/shein/submit_image_upload.go) error = %v", err)
	}
	publishingUploadContent := string(publishingUploadSrc)
	for _, needle := range []string{
		"func UploadImageInfo(info *sheinproduct.ImageInfo, uploader sheinimage.ImageAPI, uploaded map[string]string, colorBlockBuilder ColorBlockBuilder) (int, error) {",
	} {
		if !strings.Contains(publishingUploadContent, needle) {
			t.Fatalf("publishing submit_image_upload.go should contain %q", needle)
		}
	}

	publishingPolicySrc, err := os.ReadFile("../publishing/shein/submit_image_policy.go")
	if err != nil {
		t.Fatalf("ReadFile(../publishing/shein/submit_image_policy.go) error = %v", err)
	}
	publishingPolicyContent := string(publishingPolicySrc)
	for _, needle := range []string{
		"sheinmarketpub.IsUploadedImageURL(url)",
		"sheinmarketpub.IsSDSImageURL(url)",
		"sheinmarketpub.CloneImageUploadCache(input)",
	} {
		if !strings.Contains(publishingPolicyContent, needle) {
			t.Fatalf("publishing submit_image_policy.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		`strings.Contains(value, "shein.com")`,
		`strings.Contains(value, "sdspod.com")`,
		"for sourceURL, uploadedURL := range input",
	} {
		if strings.Contains(publishingPolicyContent, needle) {
			t.Fatalf("publishing submit_image_policy.go should delegate image URL/cache policy instead of keeping %q", needle)
		}
	}
}
