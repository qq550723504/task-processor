package productenrich

import (
	"net/url"
	"strings"
)

func is1688ProductDetailURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "detail.1688.com" {
		return false
	}
	path := strings.ToLower(strings.TrimSpace(parsed.EscapedPath()))
	return strings.HasPrefix(path, "/offer/") && len(strings.TrimPrefix(path, "/offer/")) > 0
}
