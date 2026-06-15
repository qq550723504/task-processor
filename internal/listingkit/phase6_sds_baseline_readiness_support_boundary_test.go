package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSDSBaselineReadinessSupportBoundary(t *testing.T) {
	t.Parallel()

	serviceSrc, err := os.ReadFile("sds_baseline_service.go")
	if err != nil {
		t.Fatalf("ReadFile(sds_baseline_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"func newSDSBaselineService(config sdsBaselineServiceConfig) *sdsBaselineService {",
		"func (b *sdsBaselineService) GetCachedBaseline(ctx context.Context, task *Task) (*canonical.Product, bool, error) {",
		"func (b *sdsBaselineService) GetReadiness(ctx context.Context, query *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error) {",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("sds_baseline_service.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func (b *sdsBaselineService) reconcileCachedSDSLoginBaselineReadiness(",
		"func isSDSBaselineCredentialBootstrapReadinessFailure(readiness *SDSBaselineReadiness) bool {",
		"func normalizedSDSBaselineValidationStatus(status string) string {",
		"func (b *sdsBaselineService) revalidateSDSBaseline(ctx context.Context, entry *SDSBaselineCacheEntry, _ *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error) {",
		"func deriveSDSBaselineOverallStatusFromResult(result sdsBaselineValidationResult) string {",
		"func deriveSDSBaselineOverallStatus(validationStatus string, validationReasonCode string, validationReason string) (string, string, string) {",
	} {
		if strings.Contains(serviceContent, needle) {
			t.Fatalf("sds_baseline_service.go should delegate readiness support helper %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("sds_baseline_readiness_support.go")
	if err != nil {
		t.Fatalf("ReadFile(sds_baseline_readiness_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"func (b *sdsBaselineService) reconcileCachedSDSLoginBaselineReadiness(",
		"func isSDSBaselineCredentialBootstrapReadinessFailure(readiness *SDSBaselineReadiness) bool {",
		"func normalizedSDSBaselineValidationStatus(status string) string {",
		"func (b *sdsBaselineService) revalidateSDSBaseline(ctx context.Context, entry *SDSBaselineCacheEntry, _ *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error) {",
		"func deriveSDSBaselineOverallStatusFromResult(result sdsBaselineValidationResult) string {",
		"func deriveSDSBaselineOverallStatus(validationStatus string, validationReasonCode string, validationReason string) (string, string, string) {",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("sds_baseline_readiness_support.go should contain %q", needle)
		}
	}
}
