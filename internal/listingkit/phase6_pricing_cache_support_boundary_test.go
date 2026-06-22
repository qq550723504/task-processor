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

	assertFileAbsent(t, "pricing_cache_key_support.go")

	reviewSrc, err := os.ReadFile("pricing_cache_review_support.go")
	if err != nil {
		t.Fatalf("ReadFile(pricing_cache_review_support.go) error = %v", err)
	}
	reviewContent := string(reviewSrc)

	for _, needle := range []string{
		"func normalizePublishedSheinPricingReview(pkg *sheinpub.Package) *sheinpub.PricingReview {",
		"func sheinPricingReviewApplicable(pkg *sheinpub.Package, review *sheinpub.PricingReview) bool {",
		"func cloneSheinPricingReview(review *sheinpub.PricingReview) *sheinpub.PricingReview {",
		"return sheinpub.NormalizePublishedPricingReview(pkg)",
		"return sheinpub.PricingReviewApplicable(pkg, review)",
		"return sheinpub.ClonePricingReview(review)",
		"func attachPricingCacheInfo(",
		"func logPricingCacheEvent(event string, req *sheinpub.BuildRequest, pkg *sheinpub.Package, info *sheinpub.ResolutionCacheInfo, fields logrus.Fields) {",
		"sheinpub.PricingShortKey(key)",
		"sheinpub.PricingStoreID(req)",
		"sheinpub.PricingProductIdentity(pkg)",
		"sheinpub.SortedPricingSKUFacts(pkg, sheinpub.PricingRule{})",
	} {
		if !strings.Contains(reviewContent, needle) {
			t.Fatalf("pricing_cache_review_support.go should contain %q", needle)
		}
	}

	publishingSrc, err := os.ReadFile("../publishing/shein/pricing_cache_identity.go")
	if err != nil {
		t.Fatalf("ReadFile(../publishing/shein/pricing_cache_identity.go) error = %v", err)
	}
	publishingContent := string(publishingSrc)

	for _, needle := range []string{
		"func PricingCacheKey(req *BuildRequest, pkg *Package, rule PricingRule) string {",
		"func PricingSKUFacts(pkg *Package, rule PricingRule) map[string]PricingSKUFact {",
		"func PricingReviewApplicable(pkg *Package, review *PricingReview) bool {",
		"func NormalizePublishedPricingReview(pkg *Package) *PricingReview {",
		"func ClonePricingReview(review *PricingReview) *PricingReview {",
	} {
		if !strings.Contains(publishingContent, needle) {
			t.Fatalf("publishing SHEIN pricing cache identity should contain %q", needle)
		}
	}
}
