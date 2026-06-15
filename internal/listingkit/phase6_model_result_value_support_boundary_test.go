package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestModelResultValueSupportBoundary(t *testing.T) {
	t.Parallel()

	modelSrc, err := os.ReadFile("model_result.go")
	if err != nil {
		t.Fatalf("ReadFile(model_result.go) error = %v", err)
	}
	supportSrc, err := os.ReadFile("model_result_value_support.go")
	if err != nil {
		t.Fatalf("ReadFile(model_result_value_support.go) error = %v", err)
	}

	modelContent := string(modelSrc)
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"type ListingKitResult struct {",
		"type StandardProductSnapshot struct {",
	} {
		if !strings.Contains(modelContent, needle) {
			t.Fatalf("model_result.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"type SDSSyncSummary struct {",
		"type PodExecutionSummary struct {",
		"type SDSSyncSensitiveWordHit struct {",
	} {
		if strings.Contains(modelContent, needle) {
			t.Fatalf("model_result.go should delegate execution summary model %q", needle)
		}
	}

	for _, needle := range []string{
		"func (r GenerateRequest) Value() (driver.Value, error) { return json.Marshal(r) }",
		"func (r *GenerateRequest) Scan(value any) error {",
		"func (r ListingKitResult) Value() (driver.Value, error) { return json.Marshal(r) }",
		"func (r *ListingKitResult) Scan(value any) error {",
	} {
		if strings.Contains(modelContent, needle) {
			t.Fatalf("model_result.go should delegate persistence helper %q", needle)
		}
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("model_result_value_support.go should contain %q", needle)
		}
	}
}
