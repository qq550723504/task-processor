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
		"func cloneSheinProductForSubmit(product *sheinproduct.Product) (*sheinproduct.Product, error) {",
		"func sheinProductImageURLCount(product *sheinproduct.Product) int {",
		"func sheinProductPendingImageUploadCount(product *sheinproduct.Product) int {",
		"func sheinImageInfoURLCount(info *sheinproduct.ImageInfo) int {",
		"func sheinImageInfoPendingUploadCount(info *sheinproduct.ImageInfo) int {",
		"func uploadSheinProductImages(product *sheinproduct.Product, uploader sheinimage.ImageAPI, cached map[string]string) (int, map[string]string, error) {",
		"func isSheinUploadedImageURL(url string) bool {",
		"func isSDSImageURL(url string) bool {",
		"func sheinImageUploadCache(pkg *SheinPackage) map[string]string {",
		"func sheinImageUploadCacheHit(pkg *SheinPackage, sourceURL string) bool {",
		"func cloneSheinImageUploadCache(input map[string]string) map[string]string {",
	} {
		if !strings.Contains(rootContent, needle) {
			t.Fatalf("shein_submit_images.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func collectSheinProductImageRefs(product *sheinproduct.Product) []sheinImageUploadRef {",
		"func appendSheinImageInfoRefs(refs []sheinImageUploadRef, info *sheinproduct.ImageInfo) []sheinImageUploadRef {",
		"func uploadSheinImageJobs(jobs map[string]sheinImageUploadJob, uploader sheinimage.ImageAPI, uploaded map[string]string) (int, error) {",
		"func runSheinImageUploadJobs(jobs map[string]sheinImageUploadJob, uploader sheinimage.ImageAPI, uploaded map[string]string, existing map[string]string) (int, error) {",
		"func uploadSingleSheinImage(job sheinImageUploadJob, uploader sheinimage.ImageAPI, existing map[string]string) (string, error) {",
		"func uploadSheinImageInfo(info *sheinproduct.ImageInfo, uploader sheinimage.ImageAPI, uploaded map[string]string) (int, error) {",
	} {
		if strings.Contains(rootContent, needle) {
			t.Fatalf("shein_submit_images.go should delegate upload support helper %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("shein_submit_image_upload_support.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_image_upload_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"func collectSheinProductImageRefs(product *sheinproduct.Product) []sheinImageUploadRef {",
		"func appendSheinImageInfoRefs(refs []sheinImageUploadRef, info *sheinproduct.ImageInfo) []sheinImageUploadRef {",
		"func uploadSheinImageJobs(jobs map[string]sheinImageUploadJob, uploader sheinimage.ImageAPI, uploaded map[string]string) (int, error) {",
		"func runSheinImageUploadJobs(jobs map[string]sheinImageUploadJob, uploader sheinimage.ImageAPI, uploaded map[string]string, existing map[string]string) (int, error) {",
		"func uploadSingleSheinImage(job sheinImageUploadJob, uploader sheinimage.ImageAPI, existing map[string]string) (string, error) {",
		"func uploadSheinImageInfo(info *sheinproduct.ImageInfo, uploader sheinimage.ImageAPI, uploaded map[string]string) (int, error) {",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("shein_submit_image_upload_support.go should contain %q", needle)
		}
	}
}
