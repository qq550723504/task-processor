package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestPricingCacheSupportFilesOwnHelperFamilies(t *testing.T) {
	t.Parallel()

	homeSrc, err := os.ReadFile("pricing_cache_service.go")
	if err != nil {
		t.Fatalf("ReadFile(pricing_cache_service.go) error = %v", err)
	}
	homeContent := string(homeSrc)

	for _, needle := range []string{
		"func (s *service) rememberSheinSubmittedPricing(task *Task, action string) {",
		"func (s *service) loadSheinPricingCache(req *GenerateRequest, pkg *sheinpub.Package) *sheinpub.PricingReview {",
		"func (s *service) clearSheinPricingCache(req *sheinpub.BuildRequest, pkg *sheinpub.Package) error {",
	} {
		if !strings.Contains(homeContent, needle) {
			t.Fatalf("pricing_cache_service.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func sheinPricingCacheKey(req *sheinpub.BuildRequest, pkg *sheinpub.Package, rule sheinpub.PricingRule) string {",
		"func sheinPricingSKUFacts(pkg *sheinpub.Package, rule sheinpub.PricingRule) map[string]sheinPricingSKUFact {",
		"func cloneSheinPricingReview(review *sheinpub.PricingReview) *sheinpub.PricingReview {",
		"func logPricingCacheEvent(event string, req *sheinpub.BuildRequest, pkg *sheinpub.Package, info *sheinpub.ResolutionCacheInfo, fields logrus.Fields) {",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("pricing_cache_service.go should delegate helper family %q", needle)
		}
	}

	keySrc, err := os.ReadFile("pricing_cache_key_support.go")
	if err != nil {
		t.Fatalf("ReadFile(pricing_cache_key_support.go) error = %v", err)
	}
	keyContent := string(keySrc)

	for _, needle := range []string{
		"func sheinPricingCacheKey(req *sheinpub.BuildRequest, pkg *sheinpub.Package, rule sheinpub.PricingRule) string {",
		"func sortedSheinPricingSKUFacts(pkg *sheinpub.Package, rule sheinpub.PricingRule) []string {",
		"func sheinPricingSKUFacts(pkg *sheinpub.Package, rule sheinpub.PricingRule) map[string]sheinPricingSKUFact {",
		"func sheinPricingSKUAlias(value string) string {",
		"func sheinPricingStoreID(req *sheinpub.BuildRequest) string {",
	} {
		if !strings.Contains(keyContent, needle) {
			t.Fatalf("pricing_cache_key_support.go should contain %q", needle)
		}
	}

	reviewSrc, err := os.ReadFile("pricing_cache_review_support.go")
	if err != nil {
		t.Fatalf("ReadFile(pricing_cache_review_support.go) error = %v", err)
	}
	reviewContent := string(reviewSrc)

	for _, needle := range []string{
		"func normalizePublishedSheinPricingReview(pkg *sheinpub.Package) *sheinpub.PricingReview {",
		"func sheinPricingReviewApplicable(pkg *sheinpub.Package, review *sheinpub.PricingReview) bool {",
		"func cloneSheinPricingReview(review *sheinpub.PricingReview) *sheinpub.PricingReview {",
		"func attachPricingCacheInfo(",
		"func logPricingCacheEvent(event string, req *sheinpub.BuildRequest, pkg *sheinpub.Package, info *sheinpub.ResolutionCacheInfo, fields logrus.Fields) {",
	} {
		if !strings.Contains(reviewContent, needle) {
			t.Fatalf("pricing_cache_review_support.go should contain %q", needle)
		}
	}
}
