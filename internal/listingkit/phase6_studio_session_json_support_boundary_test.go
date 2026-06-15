package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestStudioSessionJSONSupportBoundary(t *testing.T) {
	t.Parallel()

	modelSrc, err := os.ReadFile("studio_session_model.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_session_model.go) error = %v", err)
	}
	supportSrc, err := os.ReadFile("studio_session_json_value_support.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_session_json_value_support.go) error = %v", err)
	}

	modelContent := string(modelSrc)
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"type SheinStudioSessionStatus string",
		"type SheinStudioSession struct {",
		"type SheinStudioDesign struct {",
		"type SheinStudioSessionDetail struct {",
	} {
		if !strings.Contains(modelContent, needle) {
			t.Fatalf("studio_session_model.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"type SheinStudioSelectionVariants []SheinStudioSelectionVariant",
		"type SheinStudioSelectionSnapshot SheinStudioSelection",
		"type SheinStudioStringList []string",
		"type SheinStudioGroupedSelectionList []SheinStudioGroupedSelection",
		"func (value SheinStudioSelectionVariants) Value() (driver.Value, error) {",
		"func (value *SheinStudioGroupedSelectionList) Scan(input any) error {",
	} {
		if strings.Contains(modelContent, needle) {
			t.Fatalf("studio_session_model.go should delegate JSON value helper %q", needle)
		}
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("studio_session_json_value_support.go should contain %q", needle)
		}
	}
}
