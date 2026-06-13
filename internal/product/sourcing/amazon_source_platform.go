package sourcing

import "strings"

// CrawlerPlatformForSource maps product source platforms onto the crawler
// platform that can fetch their source product data.
func CrawlerPlatformForSource(platform string) string {
	trimmed := strings.TrimSpace(platform)
	switch strings.ToLower(trimmed) {
	case "shein", "temu":
		return "amazon"
	default:
		return trimmed
	}
}

// SupportsCrawlerSource reports whether the platform has a crawler-backed
// product source path.
func SupportsCrawlerSource(platform string) bool {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "amazon", "shein", "temu", "1688":
		return true
	default:
		return false
	}
}
