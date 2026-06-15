package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestModelRequestSupportFilesOwnSplitFamilies(t *testing.T) {
	t.Parallel()

	rootSrc, err := os.ReadFile("model_request_support.go")
	if err != nil {
		t.Fatalf("ReadFile(model_request_support.go) error = %v", err)
	}
	rootContent := string(rootSrc)

	for _, needle := range []string{
		"type modelRequestSupportBoundary struct{}",
	} {
		if !strings.Contains(rootContent, needle) {
			t.Fatalf("model_request_support.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"type SheinStudioOptions struct {",
		"type SDSSyncOptions struct {",
		"type SubmitTaskRequest struct {",
		"type SheinSettings struct {",
	} {
		if strings.Contains(rootContent, needle) {
			t.Fatalf("model_request_support.go should not contain %q after family split", needle)
		}
	}

	studioSrc, err := os.ReadFile("model_request_studio_support.go")
	if err != nil {
		t.Fatalf("ReadFile(model_request_studio_support.go) error = %v", err)
	}
	studioContent := string(studioSrc)

	for _, needle := range []string{
		"type SheinStudioOptions struct {",
		"type StudioProductImageRequest struct {",
		"type StudioDesignResponse struct {",
		"type SDSSyncOptions struct {",
		"type SDSSyncVariantOption struct {",
	} {
		if !strings.Contains(studioContent, needle) {
			t.Fatalf("model_request_studio_support.go should contain %q", needle)
		}
	}

	submitSrc, err := os.ReadFile("model_request_submit_support.go")
	if err != nil {
		t.Fatalf("ReadFile(model_request_submit_support.go) error = %v", err)
	}
	submitContent := string(submitSrc)

	for _, needle := range []string{
		"type SubmitTaskRequest struct {",
		"type SheinSettings struct {",
		"type AIClientSettings struct {",
		"type SheinFinalDraftUpdateRequest struct {",
		"type SheinCategorySearchResult struct {",
	} {
		if !strings.Contains(submitContent, needle) {
			t.Fatalf("model_request_submit_support.go should contain %q", needle)
		}
	}
}
