package listingkit

import (
	"io/fs"
	"os"
	"path/filepath"
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

func TestListingKitSourcesRejectRemovedSheinStoreSettingTerms(t *testing.T) {
	t.Parallel()

	forbidden := []struct {
		name  string
		value string
	}{
		{name: "legacy exported setting identifier", value: strings.Join([]string{"Shein", "Default", "StoreID"}, "")},
		{name: "legacy setting identifier", value: strings.Join([]string{"Default", "StoreID"}, "")},
		{name: "legacy settings key", value: strings.Join([]string{"default", "_store_id"}, "")},
		{name: "legacy Chinese health wording", value: strings.Join([]string{"默认", "店铺"}, "")},
		{name: "legacy English health wording", value: strings.Join([]string{"default", " store"}, "")},
	}

	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := strings.ToLower(string(src))
		for _, term := range forbidden {
			if strings.Contains(content, strings.ToLower(term.value)) {
				t.Errorf("%s contains %s", path, term.name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan ListingKit Go sources: %v", err)
	}
}
