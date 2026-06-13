package sourcing

import "testing"

func TestCrawlerPlatformForSourceMapsMarketplaceAliasesToAmazon(t *testing.T) {
	tests := map[string]string{
		"shein": "amazon",
		"SHEIN": "amazon",
		"temu":  "amazon",
		"TEMU":  "amazon",
	}

	for platform, want := range tests {
		got := CrawlerPlatformForSource(platform)
		if got != want {
			t.Fatalf("CrawlerPlatformForSource(%q) = %q, want %q", platform, got, want)
		}
	}
}

func TestCrawlerPlatformForSourcePreservesNativePlatform(t *testing.T) {
	got := CrawlerPlatformForSource("Amazon")
	if got != "Amazon" {
		t.Fatalf("CrawlerPlatformForSource() = %q, want original platform", got)
	}
}

func TestSupportsCrawlerSource(t *testing.T) {
	supported := []string{"amazon", "shein", "temu", "1688", " SHEIN "}
	for _, platform := range supported {
		if !SupportsCrawlerSource(platform) {
			t.Fatalf("SupportsCrawlerSource(%q) = false, want true", platform)
		}
	}

	if SupportsCrawlerSource("walmart") {
		t.Fatal("SupportsCrawlerSource(walmart) = true, want false")
	}
}
