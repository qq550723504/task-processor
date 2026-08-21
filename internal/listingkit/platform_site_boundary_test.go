package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSDSRetirementSiteDefaultsUseCommonPolicy(t *testing.T) {
	t.Parallel()

	retirementSource, err := os.ReadFile("sds_retirement_sites.go")
	if err != nil {
		t.Fatalf("read sds_retirement_sites.go: %v", err)
	}
	if !strings.Contains(string(retirementSource), "common.DefaultSites(") {
		t.Fatal("sds_retirement_sites.go should use common.DefaultSites")
	}

	helperSource, err := os.ReadFile("platform_helpers.go")
	if err != nil {
		t.Fatalf("read platform_helpers.go: %v", err)
	}
	if strings.Contains(string(helperSource), "func defaultPlatformSites(") {
		t.Fatal("platform_helpers.go should not duplicate the common site-default policy")
	}
}
