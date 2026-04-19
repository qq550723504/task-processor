package storage

import (
	"net/url"
	"strings"
)

func ResolveObjectURL(publicBase, key, fallbackURL string) string {
	trimmedKey := strings.TrimLeft(strings.TrimSpace(key), "/")
	if base := strings.TrimRight(strings.TrimSpace(publicBase), "/"); base != "" && trimmedKey != "" {
		return base + "/" + trimmedKey
	}
	return strings.TrimSpace(fallbackURL)
}

func BuildS3PublicBase(endpoint, bucket string, usePathStyle bool) string {
	trimmedEndpoint := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	trimmedBucket := strings.TrimSpace(bucket)
	if trimmedEndpoint == "" || trimmedBucket == "" {
		return ""
	}

	parsed, err := url.Parse(trimmedEndpoint)
	if err != nil {
		return ""
	}
	if usePathStyle {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + trimmedBucket
		return strings.TrimRight(parsed.String(), "/")
	}

	host := parsed.Host
	if host == "" {
		return ""
	}
	parsed.Host = trimmedBucket + "." + host
	return strings.TrimRight(parsed.String(), "/")
}
